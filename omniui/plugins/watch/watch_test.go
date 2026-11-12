package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiffRenameAndLifetime(t *testing.T) {
	dir := t.TempDir()
	w, e := Open(dir, Options{Recursive: true})
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	file := filepath.Join(dir, "a")
	if e = os.WriteFile(file, []byte("a"), 0600); e != nil {
		t.Fatal(e)
	}
	events, e := w.Poll(context.Background())
	if e != nil || len(events) != 1 || events[0].Op != "create" {
		t.Fatalf("%v %v", events, e)
	}
	if e = os.Rename(file, filepath.Join(dir, "b")); e != nil {
		t.Fatal(e)
	}
	events, e = w.Poll(context.Background())
	if e != nil || len(events) != 1 || events[0].Op != "rename" || events[0].OldPath != "a" {
		t.Fatalf("%v %v", events, e)
	}
	w.Close()
	if _, e = w.Next(context.Background()); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}
func TestSymlinkNotTraversedAndLimits(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0600)
	if e := os.Symlink(outside, filepath.Join(dir, "link")); e != nil {
		t.Skip(e)
	}
	w, e := Open(dir, Options{Recursive: true, MaxEntries: 1})
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	if len(w.previous) != 1 {
		t.Fatal("followed symlink")
	}
	os.WriteFile(filepath.Join(dir, "extra"), nil, 0600)
	if _, e = w.Poll(context.Background()); e == nil {
		t.Fatal("unbounded scan")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = w.Next(ctx); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
