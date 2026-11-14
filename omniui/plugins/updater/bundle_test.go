package updater

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func bundleFixture(t *testing.T, entry string) (string, string, []byte, []byte, ed25519.PublicKey) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "Test.app")
	if err := os.MkdirAll(filepath.Join(target, "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(target, "Contents", "Info.plist"), []byte("old"), 0644)
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	for name, value := range map[string]string{"Test.app/Contents/Info.plist": "new", entry: "payload"} {
		h := &zip.FileHeader{Name: name}
		h.SetMode(0755)
		f, e := z.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		f.Write([]byte(value))
	}
	z.Close()
	source := filepath.Join(dir, "update.zip")
	os.WriteFile(source, data.Bytes(), 0600)
	sum := sha256.Sum256(data.Bytes())
	m, _ := json.Marshal(Release{Version: "2", URL: "https://example.com/update.zip", SHA256: hex.EncodeToString(sum[:]), Size: int64(data.Len())})
	key, private, _ := ed25519.GenerateKey(rand.Reader)
	return target, source, m, ed25519.Sign(private, m), key
}
func TestBundleInstallRecovery(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{true: "rollback", false: "confirm"}[rollback], func(t *testing.T) {
			target, source, m, s, k := bundleFixture(t, "Test.app/Contents/MacOS/app")
			tx, err := installBundle(t.Context(), target, source, m, s, k, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			if _, err = RecoverBundleInstallation(t.Context(), target, k); err == nil {
				t.Fatal("accepted live owner")
			}
			tx.Close()
			tx, err = RecoverBundleInstallation(t.Context(), target, k)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Close()
			if rollback {
				err = tx.Rollback()
			} else {
				err = tx.Confirm()
			}
			if err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(filepath.Join(target, "Contents", "Info.plist"))
			expected := "new"
			if rollback {
				expected = "old"
			}
			if string(data) != expected {
				t.Fatal(string(data))
			}
		})
	}
}
func TestBundleRejectEscape(t *testing.T) {
	for _, entry := range []string{"Test.app/../../escape", "Other.app/Contents/file", "Test.app/Contents/../file", "Test.app/Contents\\file"} {
		t.Run(entry, func(t *testing.T) {
			target, source, m, s, k := bundleFixture(t, entry)
			if _, err := installBundle(t.Context(), target, source, m, s, k, func(string) error { return nil }); err == nil {
				t.Fatal("unsafe archive accepted")
			}
			data, _ := os.ReadFile(filepath.Join(target, "Contents", "Info.plist"))
			if string(data) != "old" {
				t.Fatal("target changed")
			}
		})
	}
}
func TestBundleFailedStartupRollsBack(t *testing.T) {
	target, source, m, s, k := bundleFixture(t, "Test.app/Contents/MacOS/app")
	tx, err := installBundle(t.Context(), target, source, m, s, k, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	if _, err = tx.RestartAndConfirm(t.Context(), "missing"); err == nil {
		t.Fatal("missing executable succeeded")
	}
	data, _ := os.ReadFile(filepath.Join(target, "Contents", "Info.plist"))
	if string(data) != "old" {
		t.Fatal("rollback failed")
	}
}

func TestBundleStartupChild(t *testing.T) {
	mode := os.Getenv("OMNIGO_TEST_STARTUP")
	if mode == "" {
		return
	}
	if mode == "exit" {
		os.Exit(7)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		os.Exit(8)
	}
	defer listener.Close()
	if err = ConfirmStartup(context.Background()); err != nil {
		os.Exit(9)
	}
	listener.Accept() // Keep the successfully started child alive until the supervisor test stops it.
	os.Exit(0)
}
func TestBundleStartupHandshake(t *testing.T) {
	for _, mode := range []string{"ready", "exit"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("OMNIGO_TEST_STARTUP", mode)
			target, source, m, s, k := bundleFixture(t, "Test.app/Contents/MacOS/app")
			tx, err := installBundle(t.Context(), target, source, m, s, k, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Close()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(executable)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(target, "Contents", "MacOS", "app"), data, 0755); err != nil {
				t.Fatal(err)
			}
			tx.digest, err = bundleHash(target)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			process, err := tx.RestartAndConfirm(ctx, "app", "-test.run=^TestBundleStartupChild$")
			if mode == "ready" {
				if err != nil {
					t.Fatal(err)
				}
				if err = process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					process.Kill()
					t.Fatal("unhealthy child confirmed")
				}
				old, _ := os.ReadFile(filepath.Join(target, "Contents", "Info.plist"))
				if string(old) != "old" {
					t.Fatal("failed startup did not restore previous bundle")
				}
			}
		})
	}
}
