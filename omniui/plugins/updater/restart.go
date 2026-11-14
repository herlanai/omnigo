package updater

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/herlanai/omnigo/internal/startup"
)

// ConfirmStartup reports application health to a waiting update helper. OmniGo
// Application.Run calls this automatically after its native OnReady callback.
// Custom hosts should call it only after their own readiness checks pass.
func ConfirmStartup(ctx context.Context) error { return startup.Confirm(ctx) }

// RestartAndConfirm launches Contents/MacOS/executable and retains the previous
// bundle until the new native host acknowledges readiness. Run this from an
// external helper after the old application exits. On timeout or early exit the
// child is stopped and the previous bundle restored. A successful process remains
// running; the helper must remain alive until this call returns.
// executable is a filename, not a path. ctx may shorten the 30-second deadline.
func (b *BundleInstallation) RestartAndConfirm(ctx context.Context, executable string, args ...string) (*os.Process, error) {
	if ctx == nil {
		return nil, errors.New("updater: nil context")
	}
	if executable == "" || executable == "." || filepath.Base(executable) != executable || strings.ContainsAny(executable, "/\\\x00") {
		return nil, errors.New("updater: invalid bundle executable")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.restored {
		return nil, errors.New("updater: no pending bundle startup")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	digest, err := bundleHash(b.target)
	if err != nil {
		return nil, err
	}
	if digest != b.digest {
		return nil, errors.New("updater: installed bundle changed")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(secret)
	ack := make(chan struct{}, 1)
	go func() {
		for {
			c, e := listener.Accept()
			if e != nil {
				return
			}
			c.SetReadDeadline(time.Now().Add(time.Second))
			line, e := bufio.NewReaderSize(io.LimitReader(c, 66), 128).ReadString('\n')
			c.Close()
			if e == nil && subtle.ConstantTimeCompare([]byte(line), []byte(token+"\n")) == 1 {
				ack <- struct{}{}
				return
			}
		}
	}()
	cmd := exec.Command(filepath.Join(b.target, "Contents", "MacOS", executable), args...)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, startup.AddressEnv+"=") && !strings.HasPrefix(v, startup.TokenEnv+"=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, startup.AddressEnv+"="+listener.Addr().String(), startup.TokenEnv+"="+token)
	cmd.Dir = filepath.Dir(b.target)
	prepareRestart(cmd)
	if err = cmd.Start(); err != nil {
		return nil, errors.Join(err, b.finishLocked(true))
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case <-ack:
		select {
		case err = <-exited:
			_ = stopRestart(cmd)
			return nil, errors.Join(fmt.Errorf("updater: application exited during startup: %v", err), b.finishLocked(true))
		default:
		}
		if err = b.finishLocked(false); err != nil {
			return cmd.Process, err
		}
		return cmd.Process, nil
	case err = <-exited:
		_ = stopRestart(cmd)
		return nil, errors.Join(fmt.Errorf("updater: application exited before readiness: %v", err), b.finishLocked(true))
	case <-ctx.Done():
		killErr := stopRestart(cmd)
		if killErr != nil {
			select {
			case <-exited:
			default:
				return nil, errors.Join(ctx.Err(), fmt.Errorf("updater: cannot stop child; recovery retained: %w", killErr))
			}
		} else {
			<-exited
		}
		return nil, errors.Join(ctx.Err(), killErr, b.finishLocked(true))
	}
}
