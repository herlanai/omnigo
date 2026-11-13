package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRevealRejectsBeforeLaunching(t *testing.T) {
	for _, p := range []string{"", "bad\x00path", filepath.Join(t.TempDir(), "missing")} {
		if Reveal(context.Background(), p) == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	if Reveal(nil, "anything") == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Reveal(ctx, "anything"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestRevealCommandsPreserveItemAndSelection(t *testing.T) {
	path := "/tmp/a 'quoted' $(touch nope)\n#?.txt"
	name, args, err := revealCommand("darwin", path, nil)
	if err != nil || name != "/usr/bin/open" || !reflect.DeepEqual(args, []string{"-R", "--", path}) {
		t.Fatalf("%s %q %v", name, args, err)
	}
	name, args, err = revealCommand("linux", path, func(string) (string, error) { return "/usr/bin/gdbus", nil })
	want := []string{"call", "--session", "--dest", "org.freedesktop.FileManager1", "--object-path", "/org/freedesktop/FileManager1", "--method", "org.freedesktop.FileManager1.ShowItems", "['file:///tmp/a%20%27quoted%27%20$%28touch%20nope%29%0A%23%3F.txt']", ""}
	if err != nil || name != "/usr/bin/gdbus" || !reflect.DeepEqual(args, want) {
		t.Fatalf("%s %q %v", name, args, err)
	}
	_, _, err = revealCommand("linux", path, func(string) (string, error) { return "", os.ErrNotExist })
	if !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	_, _, err = revealCommand("unknown", path, nil)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
