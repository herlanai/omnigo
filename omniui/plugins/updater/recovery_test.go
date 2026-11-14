package updater

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A child exits without finalizing. This exercises kernel lock release rather
// than treating an in-process handle reset as a crash.
func TestRecoveryAfterProcessExit(t *testing.T) {
	if os.Getenv("OMNIGO_RECOVERY_CHILD") == "1" {
		dir := os.Getenv("OMNIGO_RECOVERY_DIR")
		m, _ := os.ReadFile(filepath.Join(dir, "m"))
		s, _ := os.ReadFile(filepath.Join(dir, "s"))
		k, _ := hex.DecodeString(os.Getenv("OMNIGO_RECOVERY_KEY"))
		if _, err := InstallVerified(t.Context(), filepath.Join(dir, "app"), filepath.Join(dir, "download"), m, s, ed25519.PublicKey(k)); err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, rollback := range []bool{true, false} {
		target, _, m, s, k := installFixture(t)
		dir := filepath.Dir(target)
		os.WriteFile(filepath.Join(dir, "m"), m, 0600)
		os.WriteFile(filepath.Join(dir, "s"), s, 0600)
		cmd := exec.Command(os.Args[0], "-test.run=^TestRecoveryAfterProcessExit$")
		cmd.Env = append(os.Environ(), "OMNIGO_RECOVERY_CHILD=1", "OMNIGO_RECOVERY_DIR="+dir, "OMNIGO_RECOVERY_KEY="+hex.EncodeToString(k))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		tx, err := RecoverInstallation(t.Context(), target, k)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(tx.releaseLock)
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
		data, _ := os.ReadFile(target)
		if string(data) != expected {
			t.Fatal(string(data))
		}
	}
}
func TestRecoveryRefusesLiveOwnerAndCorruption(t *testing.T) {
	target, source, m, s, k := installFixture(t)
	tx, err := InstallVerified(t.Context(), target, source, m, s, k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tx.releaseLock)
	if _, err := RecoverInstallation(t.Context(), target, k); err == nil {
		t.Fatal("live owner bypassed")
	}
	tx.releaseLock()
	os.WriteFile(filepath.Join(target+".omnigo-update", "manifest.sig"), make([]byte, 64), 0600)
	if _, err := RecoverInstallation(t.Context(), target, k); err == nil {
		t.Fatal("signature corruption accepted")
	}
	if data, _ := os.ReadFile(tx.BackupPath()); string(data) != "old" {
		t.Fatal("backup lost")
	}
}

func TestCloseHandsOffRecoveryWithoutDeletingFiles(t *testing.T) {
	target, source, m, s, k := installFixture(t)
	tx, err := InstallVerified(t.Context(), target, source, m, s, k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Close() })
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err == nil {
		t.Fatal("closed handle changed installation")
	}
	data, err := os.ReadFile(tx.BackupPath())
	if err != nil || string(data) != "old" {
		t.Fatal("handoff lost backup", err)
	}
	recovered, err := RecoverInstallation(t.Context(), target, k)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if err := recovered.Rollback(); err != nil {
		t.Fatal(err)
	}
}
