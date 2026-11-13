package system

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPathsOverridesAndValidation(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../escape", "a/b", "a\\b", " spaced", "a:", "bad\x00", "a\tb", "CON", "com1.log", "LPT9"} {
		if _, err := NewPaths(name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	p, err := NewPaths("OmniTest")
	if err != nil {
		t.Fatal(err)
	}
	location := filepath.Join(t.TempDir(), "not-created")
	if err = p.Set(PathUserData, location); err != nil {
		t.Fatal(err)
	}
	got, err := p.Get(PathUserData)
	if err != nil || got != location {
		t.Fatal(got, err)
	}
	if _, err = os.Stat(location); !os.IsNotExist(err) {
		t.Fatal("override created a directory")
	}
	if err = p.Set(PathUserData, "bad\x00"); err == nil {
		t.Fatal("NUL accepted")
	}
	if got, _ = p.Get(PathUserData); got != location {
		t.Fatal("failed override mutated mapping")
	}
	if _, err = p.Get("invalid"); err == nil {
		t.Fatal("unknown path accepted")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				if err := p.Set(PathCache, location); err != nil {
					t.Error(err)
				}
				if _, err := p.Get(PathCache); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
}
func TestResourceLocations(t *testing.T) {
	wd := func() (string, error) { return filepath.Join(string(filepath.Separator), "project"), nil }
	for _, tc := range []struct{ exe, want string }{{"/Applications/Demo.app/Contents/MacOS/demo", "/Applications/Demo.app/Contents/Resources"}, {"/opt/demo/run", "/opt/demo"}, {"/tmp/go-build123/b001/exe/demo", "/project/resources"}, {"/tmp/go-build123/b001/demo.test", "/project/resources"}} {
		if filepath.Separator != '/' {
			continue
		}
		got, err := resourceDirectory(tc.exe, wd)
		if err != nil || got != tc.want {
			t.Fatalf("%s => %s %v", tc.exe, got, err)
		}
	}
	failure := errors.New("cwd unavailable")
	_, err := resourceDirectory(filepath.Join(os.TempDir(), "go-build123", "b001", "demo"), func() (string, error) { return "", failure })
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
}
func TestXDGUserFolders(t *testing.T) {
	if filepath.Separator != '/' {
		t.Skip("XDG paths use Unix syntax")
	}
	for _, tc := range []struct{ input, want string }{
		{`XDG_DOWNLOADS_DIR="$HOME/Downloads"`, "/home/test/Downloads"},
		{`XDG_DOWNLOADS_DIR="${HOME}/My Files" # comment`, "/home/test/My Files"},
		{`XDG_DOWNLOADS_DIR="/mnt/shared"`, "/mnt/shared"},
		{`XDG_DOWNLOADS_DIR="$HOME"`, "/home/test"},
		{`XDG_DOWNLOADS_DIR="$HOME" # disabled`, "/home/test"},
		{`XDG_DOWNLOADS_DIR="relative"`, ""},
		{`XDG_DOWNLOADS_DIR="$(touch /tmp/unwanted)"`, ""},
		{`XDG_DOWNLOADS_DIR="\$HOME/escaped"`, ""},
		{`XDG_DOWNLOADS_DIR="/mnt/a\"b"`, "/mnt/a\"b"},
		{`XDG_DOWNLOADS_DIR="/mnt/ok"; bad`, ""},
	} {
		if got := xdgFolder(strings.NewReader(tc.input), PathDownloads, "/home/test"); got != tc.want {
			t.Errorf("%s => %q want %q", tc.input, got, tc.want)
		}
	}
}
