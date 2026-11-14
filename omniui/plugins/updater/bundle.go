package updater

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// BundleInstallation retains the previous .app until explicit startup confirmation.
// The helper must run outside target; callers must stop both versions before rollback.
// This format accepts a ZIP with one top-level .app and no symlinks or special files.
type BundleInstallation struct {
	mu                            sync.Mutex
	target, dir, digest, previous string
	release                       func()
	closed                        bool
	restored                      bool
}

func bundleHash(root string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("updater: unsupported bundle entry %s", rel)
		}
		size := info.Size()
		if info.IsDir() {
			size = 0
		}
		fmt.Fprintf(h, "%d:%s:%o:%d:%t:", len(rel), rel, info.Mode().Perm(), size, info.IsDir())
		if info.Mode().IsRegular() {
			f, e := os.Open(path)
			if e != nil {
				return e
			}
			_, e = io.Copy(h, f)
			return errors.Join(e, f.Close())
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// InstallBundleVerified installs a publisher-signed ZIP after codesign verification.
// It requires macOS and a trusted, writable application directory. Replacement
// uses two renames with rollback on error; it is not an atomic directory swap.
func InstallBundleVerified(ctx context.Context, target, archive string, manifest, signature []byte, key ed25519.PublicKey) (*BundleInstallation, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("updater: .app installation requires macOS")
	}
	return installBundle(ctx, target, archive, manifest, signature, key, func(path string) error {
		out, err := exec.CommandContext(ctx, "codesign", "--verify", "--deep", "--strict", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("updater: invalid bundle signature: %w: %s", err, out)
		}
		oldID, err := bundleProperty(ctx, target, "CFBundleIdentifier")
		if err != nil {
			return err
		}
		newID, err := bundleProperty(ctx, path, "CFBundleIdentifier")
		if err != nil {
			return err
		}
		if oldID != newID {
			return errors.New("updater: bundle identifier mismatch")
		}
		executable, err := bundleProperty(ctx, path, "CFBundleExecutable")
		if err != nil {
			return err
		}
		if filepath.Base(executable) != executable || executable == "." || strings.ContainsAny(executable, "/\\") {
			return errors.New("updater: invalid CFBundleExecutable")
		}
		info, err := os.Stat(filepath.Join(path, "Contents", "MacOS", executable))
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return errors.New("updater: bundle executable is not executable")
		}
		return nil
	})
}
func bundleProperty(ctx context.Context, path, key string) (string, error) {
	out, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-extract", key, "raw", "-o", "-", filepath.Join(path, "Contents", "Info.plist")).Output()
	if err != nil {
		return "", fmt.Errorf("updater: read %s: %w", key, err)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", fmt.Errorf("updater: empty %s", key)
	}
	return value, nil
}
func installBundle(ctx context.Context, target, archive string, manifest, signature []byte, key ed25519.PublicKey, verify func(string) error) (*BundleInstallation, error) {
	if ctx == nil {
		return nil, errors.New("updater: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, err := Verify(manifest, signature, key)
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
	if !info.IsDir() || !strings.HasSuffix(target, ".app") {
		return nil, errors.New("updater: target must be an existing .app directory")
	}
	release, err := acquireInstallLock(target)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			release()
		}
	}()
	dir := target + ".omnigo-update"
	if err = os.Mkdir(dir, 0700); err != nil {
		return nil, err
	}
	defer func() {
		if !keep {
			os.RemoveAll(dir)
		}
	}()
	in, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	saved, err := os.OpenFile(filepath.Join(dir, "download.zip"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	defer saved.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(saved, h), io.LimitReader(&cancelReader{ctx: ctx, reader: in}, r.Size+1))
	if err != nil {
		return nil, err
	}
	if n != r.Size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), r.SHA256) {
		return nil, errors.New("updater: bundle archive does not match signed manifest")
	}
	reader, err := zip.NewReader(saved, n)
	if err != nil {
		return nil, err
	}
	if len(reader.File) > 20000 {
		return nil, errors.New("updater: too many bundle entries")
	}
	candidate := filepath.Join(dir, "candidate.app")
	if err = os.Mkdir(candidate, 0755); err != nil {
		return nil, err
	}
	prefix := filepath.Base(target) + "/"
	var total uint64
	for _, entry := range reader.File {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if !fs.ValidPath(name) || strings.Contains(name, "\\") || (!strings.HasPrefix(entry.Name, prefix) && name != filepath.Base(target)) {
			return nil, errors.New("updater: invalid archive path")
		}
		if !entry.Mode().IsRegular() && !entry.Mode().IsDir() {
			return nil, errors.New("updater: bundle symlinks and special files are unsupported")
		}
		if entry.UncompressedSize64 > 1<<30 || total > 1<<30-entry.UncompressedSize64 {
			return nil, errors.New("updater: expanded bundle exceeds 1 GiB")
		}
		total += entry.UncompressedSize64
		rel := strings.TrimPrefix(entry.Name, prefix)
		if name == filepath.Base(target) {
			continue
		}
		dst := filepath.Join(candidate, filepath.FromSlash(rel))
		if entry.FileInfo().IsDir() {
			if err = os.MkdirAll(dst, 0755); err != nil {
				return nil, err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return nil, err
		}
		out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, entry.Mode().Perm()&0755)
		if e != nil {
			return nil, e
		}
		stream, e := entry.Open()
		if e != nil {
			out.Close()
			return nil, e
		}
		count, e := io.Copy(out, io.LimitReader(&cancelReader{ctx: ctx, reader: stream}, int64(entry.UncompressedSize64)+1))
		e = errors.Join(e, stream.Close(), out.Close())
		if e != nil {
			return nil, e
		}
		if uint64(count) != entry.UncompressedSize64 {
			return nil, errors.New("updater: invalid extracted length")
		}
	}
	if _, err = os.Stat(filepath.Join(candidate, "Contents", "Info.plist")); err != nil {
		return nil, err
	}
	if err = verify(candidate); err != nil {
		return nil, err
	}
	previous, err := bundleHash(target)
	if err != nil {
		return nil, err
	}
	digest, err := bundleHash(candidate)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(bundleJournal{Target: target, Digest: digest, Previous: previous, Manifest: manifest, Signature: signature})
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(filepath.Join(dir, "bundle.json"), raw, 0600); err != nil {
		return nil, err
	}
	backup := filepath.Join(dir, "previous.app")
	if err = os.Rename(target, backup); err != nil {
		return nil, err
	}
	if err = os.Rename(candidate, target); err != nil {
		restore := os.Rename(backup, target)
		if restore != nil {
			keep = true
			release()
			return nil, fmt.Errorf("updater: replacement failed (%v), restore failed (%v); backup retained at %s", err, restore, backup)
		}
		return nil, err
	}
	keep = true
	return &BundleInstallation{target: target, dir: dir, digest: digest, previous: previous, release: release}, nil
}
func (b *BundleInstallation) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		b.release()
	}
	return nil
}
func (b *BundleInstallation) Confirm() error  { return b.finish(false) }
func (b *BundleInstallation) Rollback() error { return b.finish(true) }
func (b *BundleInstallation) finish(rollback bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.finishLocked(rollback)
}
func (b *BundleInstallation) finishLocked(rollback bool) error {
	if b.closed {
		return errors.New("updater: bundle transaction closed")
	}
	digest, err := bundleHash(b.target)
	if err != nil {
		return err
	}
	if digest != b.digest {
		return errors.New("updater: installed bundle changed; recovery retained")
	}
	if rollback && !b.restored {
		backup := filepath.Join(b.dir, "previous.app")
		old, e := bundleHash(backup)
		if e != nil {
			return e
		}
		if old != b.previous {
			return errors.New("updater: backup bundle changed")
		}
		failed := filepath.Join(b.dir, "failed.app")
		if e = os.Rename(b.target, failed); e != nil {
			return e
		}
		if e = os.Rename(backup, b.target); e != nil {
			return errors.Join(e, os.Rename(failed, b.target))
		}
		b.digest = b.previous
		b.restored = true
	}
	if err = os.RemoveAll(b.dir); err != nil {
		return err
	}
	b.closed = true
	b.release()
	return nil
}
