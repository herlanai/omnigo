package watch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileRootWriteAtomicSaveAndRemoval(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	mustDo(t, os.WriteFile(file, []byte("old"), 0600))
	stamp := time.Unix(100, 0)
	mustDo(t, os.Chtimes(file, stamp, stamp))
	w, err := Open(file, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	temp := filepath.Join(dir, "temporary")
	mustDo(t, os.WriteFile(temp, []byte("new"), 0600))
	mustDo(t, os.Chtimes(temp, stamp, stamp))
	mustDo(t, os.Rename(temp, file))
	expectEvents(t, w, Event{Op: "write", Path: "."})
	mustDo(t, os.WriteFile(filepath.Join(dir, "sibling"), nil, 0600))
	expectEvents(t, w)
	mustDo(t, os.Remove(file))
	expectEvents(t, w, Event{Op: "remove", Path: "."})
	if _, err = w.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	mustDo(t, os.WriteFile(file, nil, 0600))
	if _, err = w.Poll(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal("watch resurrected", err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestFileRootReplacementSymlinkEndsWithoutFollowing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	mustDo(t, os.WriteFile(file, nil, 0600))
	outside := filepath.Join(t.TempDir(), "secret")
	mustDo(t, os.WriteFile(outside, nil, 0600))
	w, err := Open(file, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	mustDo(t, os.Remove(file))
	if err = os.Symlink(outside, file); err != nil {
		t.Skip(err)
	}
	expectEvents(t, w, Event{Op: "remove", Path: "."})
	if _, err = w.Poll(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
func TestServiceFileRootsAndFileInsideDirectory(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		dir := t.TempDir()
		file := filepath.Join(dir, "file")
		mustDo(t, os.WriteFile(file, nil, 0600))
		roots := map[string]Root{"directory": {Path: dir}}
		o := ServiceOptions{Roots: roots}
		if dynamic {
			o.Resolve = func(context.Context, string) (Root, bool) { return Root{Path: file}, true }
		} else {
			roots["file"] = Root{Path: file}
		}
		s, err := NewService(o)
		if err != nil {
			t.Fatal(err)
		}
		s.next = func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Poll(ctx) }
		for _, r := range []Request{{Root: "file", Path: "sibling"}, {Root: "file", Recursive: true}} {
			if err = s.observe(t.Context(), r, nil); !errors.Is(err, ErrDenied) {
				t.Fatal(r, err)
			}
		}
		for _, r := range []Request{{Root: "file"}, {Root: "directory", Path: "file"}} {
			mustDo(t, os.WriteFile(file, nil, 0600))
			batches := 0
			err = s.observe(t.Context(), r, func(_ context.Context, b Batch) error {
				batches++
				if b.Ready {
					return os.Remove(file)
				}
				if len(b.Events) != 1 || b.Events[0].Op != "remove" || b.Events[0].Path != "." {
					t.Fatal(b)
				}
				return nil
			})
			if err != nil || batches != 2 || s.active != 0 {
				t.Fatal(err, batches, s.active)
			}
		}
		s.Close()
	}
}
func TestFileIgnoreAndMissingInitialTarget(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if w, err := Open(file, Options{}); err == nil {
		w.Close()
		t.Fatal("missing file accepted")
	}
	mustDo(t, os.WriteFile(file, nil, 0600))
	w, err := Open(file, Options{Ignore: func(path string, isDir bool) bool {
		if path != "." || isDir {
			t.Fatal(path, isDir)
		}
		return true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	mustDo(t, os.WriteFile(file, []byte("changed"), 0600))
	expectEvents(t, w)
	mustDo(t, os.Remove(file))
	expectEvents(t, w)
	if _, err = w.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}
