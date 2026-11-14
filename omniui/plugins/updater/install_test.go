package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func installFixture(t *testing.T) (string, string, []byte, []byte, ed25519.PublicKey) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "app")
	source := filepath.Join(dir, "download")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(source, []byte("new"), 0600)
	sum := sha256.Sum256([]byte("new"))
	manifest, _ := json.Marshal(Release{Version: "2", URL: "https://example.com/app", SHA256: hex.EncodeToString(sum[:]), Size: 3})
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	return target, source, manifest, ed25519.Sign(private, manifest), public
}
func TestInstallConfirmRollbackAndLock(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirm", true: "rollback"}[rollback], func(t *testing.T) {
			target, source, m, s, k := installFixture(t)
			tx, err := InstallVerified(t.Context(), target, source, m, s, k)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = tx.Close() })
			if data, _ := os.ReadFile(target); string(data) != "new" {
				t.Fatal(string(data))
			}
			if data, _ := os.ReadFile(tx.BackupPath()); string(data) != "old" {
				t.Fatal(string(data))
			}
			if _, err := InstallVerified(t.Context(), target, source, m, s, k); err == nil {
				t.Fatal("concurrent install accepted")
			}
			if rollback {
				err = tx.Rollback()
			} else {
				err = tx.Confirm()
			}
			if err != nil {
				t.Fatal(err)
			}
			expected := "new"
			if rollback {
				expected = "old"
			}
			if data, _ := os.ReadFile(target); string(data) != expected {
				t.Fatal(string(data))
			}
			if _, err = os.Stat(target + ".omnigo-update"); !os.IsNotExist(err) {
				t.Fatal("transaction leaked", err)
			}
			if err = tx.Confirm(); err == nil {
				t.Fatal("double finalize accepted")
			}
			if data, _ := os.ReadFile(source); string(data) != "new" {
				t.Fatal("source modified")
			}
		})
	}
}
func TestInstallTamperCancellationAndRecovery(t *testing.T) {
	for _, mode := range []string{"signature", "artifact", "cancel", "existing-lock"} {
		t.Run(mode, func(t *testing.T) {
			target, source, m, s, k := installFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch mode {
			case "signature":
				s[0] ^= 1
			case "artifact":
				os.WriteFile(source, []byte("bad"), 0600)
			case "cancel":
				cancel()
			case "existing-lock":
				os.Mkdir(target+".omnigo-update", 0700)
			}
			if _, err := InstallVerified(ctx, target, source, m, s, k); err == nil {
				t.Fatal("invalid install succeeded")
			}
			if data, _ := os.ReadFile(target); string(data) != "old" {
				t.Fatal("old target lost")
			}
			_, err := os.Stat(target + ".omnigo-update")
			if mode == "existing-lock" {
				if err != nil {
					t.Fatal("foreign recovery removed")
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("staging leaked")
			}
		})
	}
}
func TestInstallRefusesExternalModification(t *testing.T) {
	target, source, m, s, k := installFixture(t)
	tx, err := InstallVerified(t.Context(), target, source, m, s, k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Close() })
	os.WriteFile(target, []byte("external"), 0755)
	if err = tx.Rollback(); err == nil {
		t.Fatal("overwrote external change")
	}
	if data, _ := os.ReadFile(tx.BackupPath()); string(data) != "old" {
		t.Fatal("recovery lost")
	}
}

func TestRollbackRejectsTamperedBackup(t *testing.T) {
	target, source, m, s, k := installFixture(t)
	tx, err := InstallVerified(t.Context(), target, source, m, s, k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Close() })
	if err := os.WriteFile(tx.BackupPath(), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err == nil {
		t.Fatal("tampered backup installed")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "new" {
		t.Fatal("working target replaced", string(data))
	}
	if _, err := os.Stat(tx.BackupPath()); err != nil {
		t.Fatal("recovery evidence removed", err)
	}
}

// Reconstruct the state after a successful restore but interrupted cleanup.
// Finalization must be retryable without expecting a now-moved backup file.
func TestRollbackCleanupCanResumeAfterRestore(t *testing.T) {
	target, source, m, s, k := installFixture(t)
	tx, err := InstallVerified(t.Context(), target, source, m, s, k)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tx.BackupPath(), target); err != nil {
		t.Fatal(err)
	}
	tx.restored = true
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "old" {
		t.Fatal(string(data))
	}
	if _, err := os.Stat(target + ".omnigo-update"); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
