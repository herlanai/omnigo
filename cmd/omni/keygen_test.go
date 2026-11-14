package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/omniui/plugins/updater"
)

func keyBytes(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestKeygenProducesPairAcceptedByUpdater(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := keygen([]string{"-o", dir}, &out); err != nil {
		t.Fatal(err)
	}
	secretText := keyBytes(t, dir, "omnigo-update.key")
	publicText := keyBytes(t, dir, "omnigo-update.pub")
	secret, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(secretText)))
	if err != nil || len(secret) != ed25519.PrivateKeySize {
		t.Fatal("invalid private key encoding")
	}
	public, err := updater.ParsePublicKey(publicText)
	if err != nil || len(public) != ed25519.PublicKeySize {
		t.Fatal("invalid public key encoding")
	}
	if !bytes.Equal(ed25519.PrivateKey(secret).Public().(ed25519.PublicKey), public) {
		t.Fatal("keys do not match")
	}
	manifest := []byte(`{"version":"1.0.0","url":"https://example.com/update.zip","sha256":"` + strings.Repeat("0", 64) + `","size":1}`)
	signature := ed25519.Sign(secret, manifest)
	if _, err := updater.Verify(manifest, signature, public); err != nil {
		t.Fatal(err)
	}
	manifest[12] = '2'
	if _, err := updater.Verify(manifest, signature, public); err == nil {
		t.Fatal("modified manifest accepted")
	}
	if bytes.Contains(out.Bytes(), bytes.TrimSpace(secretText)) {
		t.Fatal("private key printed")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "omnigo-update.key"))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("private key permissions", err)
		}
	}
	if err := keygen([]string{"-o", dir}, io.Discard); err == nil {
		t.Fatal("keys overwritten without force")
	}
	if !bytes.Equal(secretText, keyBytes(t, dir, "omnigo-update.key")) || !bytes.Equal(publicText, keyBytes(t, dir, "omnigo-update.pub")) {
		t.Fatal("refusal changed pair")
	}
	if err := keygen([]string{"-o", dir, "-force"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(secretText, keyBytes(t, dir, "omnigo-update.key")) || bytes.Equal(publicText, keyBytes(t, dir, "omnigo-update.pub")) {
		t.Fatal("force did not rotate pair")
	}
}

func TestKeygenRollsBackPartialInstallation(t *testing.T) {
	for _, existing := range []bool{false, true} {
		dir := t.TempDir()
		var oldSecret, oldPublic []byte
		if existing {
			if err := writeUpdateKeys(dir, false); err != nil {
				t.Fatal(err)
			}
			oldSecret = keyBytes(t, dir, "omnigo-update.key")
			oldPublic = keyBytes(t, dir, "omnigo-update.pub")
		}
		err := writeUpdateKeysWith(dir, existing, func(root *os.Root, name string, data []byte, mode fs.FileMode) (bool, error) {
			if strings.HasSuffix(name, ".pub") {
				return false, fs.ErrPermission
			}
			return installUpdateKey(root, name, data, mode)
		})
		if !errors.Is(err, fs.ErrPermission) {
			t.Fatal(err)
		}
		if existing {
			if !bytes.Equal(oldSecret, keyBytes(t, dir, "omnigo-update.key")) || !bytes.Equal(oldPublic, keyBytes(t, dir, "omnigo-update.pub")) {
				t.Fatal("rollback lost original pair")
			}
		} else {
			for _, name := range []string{"omnigo-update.key", "omnigo-update.pub"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, fs.ErrNotExist) {
					t.Fatal("partial key retained", err)
				}
			}
		}
		if _, err := os.Stat(filepath.Join(dir, keygenLock)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("lock retained", err)
		}
	}
}

func TestKeygenConcurrentAttemptCannotRotateActivePair(t *testing.T) {
	dir := t.TempDir()
	entered := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- writeUpdateKeysWith(dir, false, func(root *os.Root, name string, data []byte, mode fs.FileMode) (bool, error) {
			if strings.HasSuffix(name, ".key") {
				close(entered)
				<-resume
			}
			return installUpdateKey(root, name, data, mode)
		})
	}()
	<-entered
	err := writeUpdateKeys(dir, true)
	close(resume)
	first := <-done
	if err == nil || first != nil {
		t.Fatal(err, first)
	}
}

func TestKeygenRejectsNonRegularDestinationsAndExtraArguments(t *testing.T) {
	dir := t.TempDir()
	if err := keygen([]string{"-o", dir, "unexpected"}, io.Discard); err == nil {
		t.Fatal("positional argument ignored")
	}
	if err := os.Mkdir(filepath.Join(dir, "omnigo-update.pub"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeUpdateKeys(dir, true); err == nil {
		t.Fatal("directory replaced")
	}
	if _, err := os.Stat(filepath.Join(dir, "omnigo-update.key")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("private key partially written")
	}
}

func TestKeygenKeepsRecoveryBackupWhenAnotherWriterClaimsDestination(t *testing.T) {
	dir := t.TempDir()
	if err := writeUpdateKeys(dir, false); err != nil {
		t.Fatal(err)
	}
	oldPublic := keyBytes(t, dir, "omnigo-update.pub")
	oldSecret := keyBytes(t, dir, "omnigo-update.key")
	foreign := []byte("another writer owns this file\n")
	err := writeUpdateKeysWith(dir, true, func(root *os.Root, name string, data []byte, mode fs.FileMode) (bool, error) {
		if strings.HasSuffix(name, ".pub") {
			if _, err := installUpdateKey(root, name, foreign, mode); err != nil {
				return false, err
			}
			return false, fs.ErrExist
		}
		return installUpdateKey(root, name, data, mode)
	})
	if !errors.Is(err, fs.ErrExist) {
		t.Fatal(err)
	}
	if !bytes.Equal(oldSecret, keyBytes(t, dir, "omnigo-update.key")) {
		t.Fatal("private key not restored")
	}
	if !bytes.Equal(foreign, keyBytes(t, dir, "omnigo-update.pub")) {
		t.Fatal("unowned file overwritten")
	}
	if !bytes.Equal(oldPublic, keyBytes(t, filepath.Join(dir, keygenLock), "omnigo-update.pub")) {
		t.Fatal("recovery backup missing")
	}
	if err := writeUpdateKeys(dir, true); err == nil {
		t.Fatal("new command ignored unresolved recovery")
	}
}

func TestKeygenDoesNotFollowExistingKeySymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "omnigo-update.key")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := writeUpdateKeys(dir, true); err == nil {
		t.Fatal("key symlink accepted")
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "unchanged" {
		t.Fatal("symlink target altered", err)
	}
}
