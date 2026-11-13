package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTrashValidationAndCancellationPreserveFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a 'quoted' file.txt")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Trash(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := Trash(nil, path); err == nil {
		t.Fatal("nil context accepted")
	}
	for _, p := range []string{"", "bad\x00path", filepath.VolumeName(dir) + string(os.PathSeparator), filepath.Join(dir, "absent")} {
		if _, err := trashPath(p); err == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "keep" {
		t.Fatalf("%q %v", got, err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "dangling")
		if err := os.Symlink(filepath.Join(dir, "missing-target"), link); err != nil {
			t.Fatal(err)
		}
		if got, err := trashPath(link); err != nil || got != link {
			t.Fatalf("%q %v", got, err)
		}
	}
}
