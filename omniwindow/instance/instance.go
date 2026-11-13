// Package instance coordinates application processes using an OS lock and an
// authenticated loopback transport. Messages must be dispatched onto the UI by
// the application host; this package never invokes UI callbacks on a worker.
package instance

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrRunning = errors.New("instance: another process owns the application")

// ErrNotReady means no payload was sent and retrying cannot duplicate a launch.
var ErrNotReady = errors.New("instance: owner endpoint not ready")

var ErrBusy = errors.New("instance: launch queue full")

const maxMessage = 1 << 20

type Launch struct {
	Args            []string `json:"args"`
	WorkingDir      string   `json:"workingDir"`
	ActivationToken string   `json:"activationToken,omitempty"`
}
type endpoint struct {
	Address string
	Token   string
}
type request struct {
	Token  string
	Launch Launch
}
type Lease struct {
	listener net.Listener
	lock     *os.File
	path     string
	endpoint endpoint
	messages chan Launch
	done     chan struct{}
	once     sync.Once
}

func paths(root, id string) (string, error) {
	if root == "" || id == "" {
		return "", errors.New("instance: private directory and application ID required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err = prepareDirectory(absolute); err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(id))
	return filepath.Join(absolute, hex.EncodeToString(sum[:])), nil
}

// Open acquires the process lock. The root must be private to the current user.
// Close or context cancellation releases it; the OS also releases it on a crash.
func Open(ctx context.Context, root, id string) (*Lease, error) {
	if ctx == nil {
		return nil, errors.New("instance: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := paths(root, id)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	held, err := tryLock(f)
	if err != nil || !held {
		f.Close()
		if err != nil {
			return nil, err
		}
		return nil, ErrRunning
	}
	cleanup := func() { unlock(f); f.Close() }
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		cleanup()
		return nil, err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		ln.Close()
		cleanup()
		return nil, err
	}
	l := &Lease{listener: ln, lock: f, path: path + ".json", endpoint: endpoint{ln.Addr().String(), hex.EncodeToString(token)}, messages: make(chan Launch, 32), done: make(chan struct{})}
	raw, _ := json.Marshal(l.endpoint)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".instance-")
	if err == nil {
		_, err = tmp.Write(raw)
		if e := tmp.Close(); err == nil {
			err = e
		}
		if err == nil {
			err = os.Rename(tmp.Name(), l.path)
		}
		os.Remove(tmp.Name())
	}
	if err != nil {
		ln.Close()
		cleanup()
		return nil, err
	}
	go l.serve()
	go func() {
		select {
		case <-ctx.Done():
			l.Close()
		case <-l.done:
		}
	}()
	return l, nil
}

// Messages is bounded. A successful Notify acknowledges enqueue, not UI handling.
func (l *Lease) Messages() <-chan Launch { return l.messages }
func (l *Lease) Close() {
	l.once.Do(func() {
		l.listener.Close()
		<-l.done
		os.Remove(l.path)
		unlock(l.lock)
		l.lock.Close()
	})
}
func (l *Lease) serve() {
	defer close(l.done)
	defer close(l.messages)
	for {
		c, err := l.listener.Accept()
		if err != nil {
			return
		}
		l.receive(c)
	}
}
func (l *Lease) receive(c net.Conn) {
	defer c.Close()
	c.SetDeadline(time.Now().Add(2 * time.Second))
	var r request
	raw, readErr := bufio.NewReader(io.LimitReader(c, maxMessage+1)).ReadBytes('\n')
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	result := "invalid"
	if readErr == nil && len(raw) <= maxMessage && d.Decode(&r) == nil && d.Decode(new(any)) == io.EOF && subtle.ConstantTimeCompare([]byte(r.Token), []byte(l.endpoint.Token)) == 1 {
		select {
		case l.messages <- r.Launch:
			result = "ok"
		default:
			result = "busy"
		}
	}
	json.NewEncoder(c).Encode(result)
}

// Notify forwards launch arguments to an existing owner. Cancellation closes the
// transport; it cannot withdraw a message already queued by the receiving process.
func Notify(ctx context.Context, root, id string, launch Launch) error {
	if ctx == nil {
		return errors.New("instance: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := paths(root, id)
	if err != nil {
		return err
	}
	f, err := os.Open(path + ".json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrNotReady, err)
		}
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	f.Close()
	if err != nil {
		return err
	}
	if len(raw) > 4096 {
		return errors.New("instance: oversized endpoint")
	}
	var e endpoint
	if err = json.Unmarshal(raw, &e); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(e.Address)
	if err != nil || host != "127.0.0.1" || len(e.Token) != 64 {
		return errors.New("instance: invalid endpoint")
	}
	body, err := json.Marshal(request{e.Token, launch})
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if len(body) > maxMessage {
		return errors.New("instance: launch exceeds limit")
	}
	c, err := (&net.Dialer{}).DialContext(ctx, "tcp4", e.Address)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.Join(ErrNotReady, err)
	}
	defer c.Close()
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	deadline := time.Now().Add(2 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	c.SetDeadline(deadline)
	if _, err = c.Write(body); err == nil {
		var result string
		err = json.NewDecoder(io.LimitReader(c, 128)).Decode(&result)
		if err == nil {
			switch result {
			case "ok":
				return nil
			case "busy":
				return ErrBusy
			default:
				return errors.New("instance: rejected launch")
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("instance: notify: %w", err)
}
