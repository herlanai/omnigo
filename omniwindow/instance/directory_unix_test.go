//go:build darwin || linux

package instance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateDirectoryPermissionsAndSymlinks(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "private")
	if err := prepareDirectory(private); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(private, 0755); err != nil {
		t.Fatal(err)
	}
	if err := prepareDirectory(private); err == nil {
		t.Fatal("shared directory accepted")
	}
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	if err := prepareDirectory(link); err == nil {
		t.Fatal("symlink accepted")
	}
	if info, err := os.Stat(private); err != nil || info.Mode().Perm() != 0700 {
		t.Fatal(info, err)
	}
}
