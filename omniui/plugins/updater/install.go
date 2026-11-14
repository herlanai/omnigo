package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
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

// Installation owns an exclusive target lock and the previous executable until
// Confirm or Rollback succeeds. Call from an external updater after the app has
// stopped. Only regular-file executables are supported, not app bundles/archives.
// Process death leaves a journal and backup for explicit RecoverInstallation;
// it never permits bypassing a live updater's process lock.
type Installation struct {
	mu                        sync.Mutex
	target, directory, digest string
	backupDigest              string
	restored                  bool
	closed                    bool
	releaseLock               func()
}

func hashFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("updater: expected a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// InstallVerified verifies the signed manifest again and hashes the staged copy
// before replacing target. The source is never renamed or deleted. target's
// parent must be owned by the application and not writable by untrusted users.
// No caller-controlled Release can bypass manifest authentication here.
func InstallVerified(ctx context.Context, target, source string, manifest, signature []byte, key ed25519.PublicKey) (*Installation, error) {
	if ctx == nil {
		return nil, errors.New("updater: nil context")
	}
	release, err := Verify(manifest, signature, key)
	if err != nil {
		return nil, err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("updater: target must be a regular executable")
	}
	releaseLock, err := acquireInstallLock(target)
	if err != nil {
		return nil, err
	}
	lockOwned := true
	defer func() {
		if lockOwned {
			releaseLock()
		}
	}()
	directory := target + ".omnigo-update"
	if err = os.Mkdir(directory, 0700); err != nil {
		return nil, fmt.Errorf("updater: acquire transaction (existing directory requires recovery): %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(directory)
		}
	}()
	copyFile := func(src, dst string, limit int64) (int64, string, error) {
		stat, e := os.Lstat(src)
		if e != nil {
			return 0, "", e
		}
		if !stat.Mode().IsRegular() {
			return 0, "", errors.New("updater: source must be regular")
		}
		in, e := os.Open(src)
		if e != nil {
			return 0, "", e
		}
		defer in.Close()
		out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if e != nil {
			return 0, "", e
		}
		h := sha256.New()
		n, e := io.Copy(io.MultiWriter(out, h), io.LimitReader(&cancelReader{ctx: ctx, reader: in}, limit))
		syncErr := out.Sync()
		closeErr := out.Close()
		return n, hex.EncodeToString(h.Sum(nil)), errors.Join(e, syncErr, closeErr)
	}
	candidate := filepath.Join(directory, "candidate")
	n, digest, err := copyFile(source, candidate, release.Size+1)
	if err != nil {
		return nil, err
	}
	if n != release.Size || !strings.EqualFold(digest, release.SHA256) {
		return nil, errors.New("updater: staged artifact does not match signed manifest")
	}
	_, backupDigest, err := copyFile(target, filepath.Join(directory, "previous"), 1<<63-1)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(directory, "manifest.json"), manifest, 0600); err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(directory, "manifest.sig"), signature, 0600); err != nil {
		return nil, err
	}
	journal, err := json.Marshal(installJournal{Version: 1, Target: target, Previous: backupDigest})
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, "transaction.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	_, writeErr := file.Write(journal)
	err = errors.Join(writeErr, file.Sync(), file.Close())
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = os.Rename(candidate, target); err != nil {
		return nil, fmt.Errorf("updater: replacement failed; original retained: %w", err)
	}
	keep = true
	lockOwned = false
	return &Installation{target: target, directory: directory, digest: digest, backupDigest: backupDigest, releaseLock: releaseLock}, nil
}

type cancelReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *cancelReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// BackupPath identifies the retained previous executable for diagnostics.
func (i *Installation) BackupPath() string { return filepath.Join(i.directory, "previous") }

// Confirm removes recovery data only after the host has confirmed startup.
func (i *Installation) Confirm() error { return i.finish(false) }

// Rollback restores the old executable after the host has stopped the new app.
func (i *Installation) Rollback() error { return i.finish(true) }
func (i *Installation) finish(rollback bool) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed {
		return errors.New("updater: installation already finalized")
	}
	digest, err := hashFile(i.target)
	if err != nil {
		return err
	}
	expected := i.digest
	if i.restored {
		expected = i.backupDigest
	}
	if digest != expected {
		return errors.New("updater: target changed externally; recovery data retained")
	}
	if rollback && !i.restored {
		backupDigest, checkErr := hashFile(i.BackupPath())
		if checkErr != nil {
			return checkErr
		}
		if backupDigest != i.backupDigest {
			return errors.New("updater: backup changed; rollback refused")
		}
		if err = os.Rename(i.BackupPath(), i.target); err != nil {
			return err
		}
		i.restored = true
	}
	if err = os.RemoveAll(i.directory); err != nil {
		return fmt.Errorf("updater: cleanup recovery directory: %w", err)
	}
	i.closed = true
	if i.releaseLock != nil {
		i.releaseLock()
	}
	return nil
}

// Close relinquishes transaction ownership without confirming or rolling back.
// Recovery files remain intact; a subsequent RecoverInstallation may acquire
// them. The closed handle cannot mutate the target. Close is idempotent.
func (i *Installation) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.closed {
		i.closed = true
		if i.releaseLock != nil {
			i.releaseLock()
		}
	}
	return nil
}
