package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type installJournal struct {
	Version  int    `json:"version"`
	Target   string `json:"target"`
	Previous string `json:"previous"`
}

func acquireInstallLock(target string) (func(), error) {
	path := target + ".omnigo-update.lock"
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("updater: invalid lock file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	locked, err := tryInstallLock(file)
	if err != nil || !locked {
		file.Close()
		if err == nil {
			err = errors.New("updater: installation is owned by a live process")
		}
		return nil, err
	}
	var once sync.Once
	// Never unlink the lock inode: unlinking would allow two separate locks for
	// the same target when another process already has this file open.
	return func() { once.Do(func() { unlockInstall(file); file.Close() }) }, nil
}
func recoveryRead(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("updater: invalid recovery file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("updater: oversized recovery file")
	}
	return data, nil
}

// RecoverInstallation reopens an interrupted transaction after its owning process
// exits. It refuses live owners and unexpected contents, never launches an app,
// and retains all evidence on error. The host chooses Confirm or Rollback.
// The application-owned directory and journal must not be writable by untrusted
// users. The signing key authenticates the new release, not the local backup.
func RecoverInstallation(ctx context.Context, target string, key ed25519.PublicKey) (*Installation, error) {
	if ctx == nil {
		return nil, errors.New("updater: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	release, err := acquireInstallLock(target)
	if err != nil {
		return nil, err
	}
	owned := true
	defer func() {
		if owned {
			release()
		}
	}()
	directory := target + ".omnigo-update"
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("updater: invalid recovery directory")
	}
	raw, err := recoveryRead(filepath.Join(directory, "transaction.json"), 4096)
	if err != nil {
		return nil, err
	}
	var journal installJournal
	if err = json.Unmarshal(raw, &journal); err != nil {
		return nil, err
	}
	previous, err := hex.DecodeString(journal.Previous)
	if err != nil || len(previous) != 32 || journal.Version != 1 || journal.Target != target {
		return nil, errors.New("updater: invalid recovery journal")
	}
	manifest, err := recoveryRead(filepath.Join(directory, "manifest.json"), 1<<20)
	if err != nil {
		return nil, err
	}
	signature, err := recoveryRead(filepath.Join(directory, "manifest.sig"), ed25519.SignatureSize)
	if err != nil {
		return nil, err
	}
	r, err := Verify(manifest, signature, key)
	if err != nil {
		return nil, err
	}
	current, err := hashFile(target)
	if err != nil {
		return nil, err
	}
	old := strings.EqualFold(current, journal.Previous)
	if !old && !strings.EqualFold(current, r.SHA256) {
		return nil, errors.New("updater: target matches neither recorded version")
	}
	if !old {
		backup, err := hashFile(filepath.Join(directory, "previous"))
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(backup, journal.Previous) {
			return nil, fmt.Errorf("updater: recovery backup digest mismatch")
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	owned = false
	return &Installation{target: target, directory: directory, digest: strings.ToLower(r.SHA256), backupDigest: strings.ToLower(journal.Previous), restored: old, releaseLock: release}, nil
}
