package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type bundleJournal struct {
	Target, Digest, Previous string
	Manifest, Signature      []byte
}

// RecoverBundleInstallation acquires an abandoned bundle transaction. Recovery
// metadata must remain in an application-owned directory. No power-loss durability
// is promised. Publisher metadata and both local tree digests are checked.
func RecoverBundleInstallation(ctx context.Context, target string, key ed25519.PublicKey) (*BundleInstallation, error) {
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
	keep := false
	defer func() {
		if !keep {
			release()
		}
	}()
	dir := target + ".omnigo-update"
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("updater: invalid bundle recovery directory")
	}
	raw, err := recoveryRead(filepath.Join(dir, "bundle.json"), 1<<20)
	if err != nil {
		return nil, err
	}
	var j bundleJournal
	if err = json.Unmarshal(raw, &j); err != nil {
		return nil, err
	}
	if j.Target != target || len(j.Digest) != 64 || len(j.Previous) != 64 {
		return nil, errors.New("updater: invalid bundle journal")
	}
	if _, err = Verify(j.Manifest, j.Signature, key); err != nil {
		return nil, err
	}
	current, err := bundleHash(target)
	if os.IsNotExist(err) {
		old, e := bundleHash(filepath.Join(dir, "previous.app"))
		if e != nil {
			return nil, e
		}
		if old != j.Previous {
			return nil, errors.New("updater: changed backup")
		}
		if err = os.Rename(filepath.Join(dir, "previous.app"), target); err != nil {
			return nil, err
		}
		current = j.Previous
	} else if err != nil {
		return nil, err
	}
	restored := current == j.Previous
	if current != j.Digest && !restored {
		return nil, errors.New("updater: changed installed bundle")
	}
	if !restored {
		old, e := bundleHash(filepath.Join(dir, "previous.app"))
		if e != nil {
			return nil, e
		}
		if old != j.Previous {
			return nil, errors.New("updater: changed backup")
		}
	}
	keep = true
	return &BundleInstallation{target: target, dir: dir, digest: current, previous: j.Previous, restored: restored, release: release}, nil
}
