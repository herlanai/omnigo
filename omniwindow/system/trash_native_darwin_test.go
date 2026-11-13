//go:build darwin && (amd64 || arm64)

package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTrashRestore(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_TEST") != "1" {
		t.Skip("explicit native trash test; only owned temporary items")
	}
	root := t.TempDir()
	for _, kind := range []string{"file", "directory", "symlink", "dangling"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(root, kind)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "中文 'quoted' item")
			target := filepath.Join(dir, "target")
			data := []byte("owned test bytes\x00\xff")
			switch kind {
			case "file":
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "child"), data, 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink", "dangling":
				if kind == "symlink" {
					if err := os.WriteFile(target, data, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			destination, err := trashMac(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if destination == "" || destination == path {
				t.Fatalf("invalid destination %q", destination)
			}
			// Restore exactly this test-owned item even when an assertion fails.
			t.Cleanup(func() {
				if _, err := os.Lstat(destination); err == nil {
					if err := os.Rename(destination, path); err != nil {
						t.Errorf("restore %q to %q: %v", destination, path, err)
					}
				}
			})
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("source remains: %v", err)
			}
			if _, err := os.Lstat(destination); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" || kind == "dangling" {
				if link, err := os.Readlink(destination); err != nil || link != target {
					t.Fatalf("link %q: %v", link, err)
				}
				if kind == "symlink" {
					if got, err := os.ReadFile(target); err != nil || string(got) != string(data) {
						t.Fatalf("target changed: %q %v", got, err)
					}
				}
			} else {
				item := destination
				if kind == "directory" {
					item = filepath.Join(item, "child")
				}
				if got, err := os.ReadFile(item); err != nil || string(got) != string(data) {
					t.Fatalf("content changed: %q %v", got, err)
				}
			}
			if err := os.Rename(destination, path); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeTrashErrorsAndCancellation(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_TEST") != "1" {
		t.Skip("explicit native trash test")
	}
	path := filepath.Join(t.TempDir(), "missing")
	if _, err := trashMac(context.Background(), path); err == nil {
		t.Fatal("missing file succeeded")
	}
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := trashMac(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "keep" {
		t.Fatalf("%q %v", b, err)
	}
}
