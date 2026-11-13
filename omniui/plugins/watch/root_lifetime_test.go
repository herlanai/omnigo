package watch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryRootMovedEndsWithOnlyRootRemoval(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	moved := filepath.Join(dir, "moved")
	mustDo(t, os.MkdirAll(filepath.Join(root, "sub"), 0700))
	mustDo(t, os.WriteFile(filepath.Join(root, "sub", "file"), nil, 0600))
	w := mustWatch(t, root)
	mustDo(t, os.Rename(root, moved))
	mustDo(t, os.WriteFile(filepath.Join(moved, "old-handle-write"), nil, 0600))
	expectEvents(t, w, Event{Op: "remove", Path: ".", IsDir: true})
	if _, err := w.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
func TestDirectoryRootReplacementDoesNotAdoptNewTree(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	mustDo(t, os.Mkdir(root, 0700))
	w := mustWatch(t, root)
	mustDo(t, os.Rename(root, filepath.Join(dir, "old")))
	mustDo(t, os.Mkdir(root, 0700))
	mustDo(t, os.WriteFile(filepath.Join(root, "new-tree"), nil, 0600))
	expectEvents(t, w, Event{Op: "remove", Path: ".", IsDir: true})
	if _, err := w.Poll(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
func TestFileParentMovedEndsAtOriginalFilePath(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	mustDo(t, os.Mkdir(parent, 0700))
	file := filepath.Join(parent, "file")
	mustDo(t, os.WriteFile(file, nil, 0600))
	w, err := Open(file, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	mustDo(t, os.Rename(parent, filepath.Join(dir, "moved")))
	expectEvents(t, w, Event{Op: "remove", Path: "."})
	if _, err = w.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
func TestRootReplacedDuringScanDiscardsRetainedHandleEvents(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	mustDo(t, os.Mkdir(root, 0700))
	mustDo(t, os.WriteFile(filepath.Join(root, "entry"), nil, 0600))
	move := false
	w, err := Open(root, Options{Ignore: func(path string, _ bool) bool {
		if move && path == "entry" {
			move = false
			mustDo(t, os.Rename(root, filepath.Join(dir, "moved")))
		}
		return false
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	mustDo(t, os.WriteFile(filepath.Join(root, "entry"), []byte("changed"), 0600))
	move = true
	expectEvents(t, w, Event{Op: "remove", Path: ".", IsDir: true})
}
func TestServiceSubdirectoryRemovalCompletesMethod(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	mustDo(t, os.Mkdir(sub, 0700))
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir, Recursive: true}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Poll(ctx) }
	batches := 0
	err = s.observe(t.Context(), Request{Root: "project", Path: "sub"}, func(_ context.Context, b Batch) error {
		batches++
		if b.Ready {
			return os.Remove(sub)
		}
		if len(b.Events) != 1 || b.Events[0] != (Event{Op: "remove", Path: ".", IsDir: true}) {
			t.Fatal(b)
		}
		return nil
	})
	if err != nil || batches != 2 || s.active != 0 {
		t.Fatal(err, batches, s.active)
	}
}
