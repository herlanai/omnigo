//go:build !windows

package system

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApplicationDirectoriesCreatedOnDemand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	p, err := NewPaths("TestApp")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []PathName{PathUserData, PathCache, PathLogs} {
		path, err := p.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Fatal(path, err)
		}
		if info.Mode().Perm()&0077 != 0 {
			t.Fatal("private directory is accessible to others")
		}
	}
	desktop, err := p.Get(PathDesktop)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(desktop); !os.IsNotExist(err) {
		t.Fatal("user folder was created")
	}
	if err = p.Set(PathCache, filepath.Join(home, "override")); err != nil {
		t.Fatal(err)
	}
	p.Set(PathCache, "")
	restored, err := p.Get(PathCache)
	if err != nil || filepath.Base(restored) != "TestApp" {
		t.Fatal(restored, err)
	}
	if runtime.GOOS == "linux" {
		config, _ := os.UserConfigDir()
		if err := os.WriteFile(filepath.Join(config, "user-dirs.dirs"), []byte(`XDG_DOCUMENTS_DIR="$HOME/Custom Docs"`), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := p.Get(PathDocuments)
		if err != nil || got != filepath.Join(home, "Custom Docs") {
			t.Fatal(got, err)
		}
	}
}
