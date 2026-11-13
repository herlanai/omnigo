package watch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func requireRootLinkSupport(t *testing.T, dir string) {
	t.Helper()
	probe := filepath.Join(dir, "probe-link")
	if err := os.Symlink(dir, probe); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	mustDo(t, os.Remove(probe))
}

func TestNewSubscriptionRejectsRootReintroducedThroughLink(t *testing.T) {
	for _, kind := range []string{"directory", "ancestor", "file-parent"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			requireRootLinkSupport(t, dir)
			parent := filepath.Join(dir, "parent")
			root := filepath.Join(parent, "root")
			mustDo(t, os.MkdirAll(root, 0700))
			if kind == "file-parent" {
				root = filepath.Join(root, "file")
				mustDo(t, os.WriteFile(root, nil, 0600))
			}
			s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: root, Recursive: true}}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			original := root
			if kind != "directory" {
				original = parent
			}
			moved := filepath.Join(dir, "moved")
			mustDo(t, os.Rename(original, moved))
			mustDo(t, os.Symlink(moved, original))
			err = s.observe(t.Context(), Request{Root: "project"}, func(context.Context, Batch) error { t.Fatal("replaced root started"); return nil })
			if err == nil {
				t.Fatal("link adopted")
			}
			if s.active != 0 || len(s.reservations) != 0 || len(s.bySource) != 0 {
				t.Fatal("reservation retained")
			}
		})
	}
}

func TestActiveWatchEndsWhenWatchedDirectoryBecomesLinkToOriginalTree(t *testing.T) {
	for _, file := range []bool{false, true} {
		name := "directory"
		if file {
			name = "file"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			requireRootLinkSupport(t, dir)
			parent := filepath.Join(dir, "parent")
			root := filepath.Join(parent, "root")
			mustDo(t, os.MkdirAll(root, 0700))
			if file {
				root = filepath.Join(root, "file")
				mustDo(t, os.WriteFile(root, nil, 0600))
			}
			w, err := Open(root, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			moved := filepath.Join(dir, "moved")
			watchedDirectory := root
			if file {
				watchedDirectory = filepath.Dir(root)
			}
			mustDo(t, os.Rename(watchedDirectory, moved))
			mustDo(t, os.Symlink(moved, watchedDirectory))
			expectEvents(t, w, Event{Op: "remove", Path: ".", IsDir: !file})
			if _, err = w.Next(t.Context()); !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
		})
	}
}

func TestServiceFileRootAllowsAtomicSaveBeforeNewSubscription(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	mustDo(t, os.WriteFile(file, []byte("old"), 0600))
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: file}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	replacement := filepath.Join(dir, "replacement")
	mustDo(t, os.WriteFile(replacement, []byte("new"), 0600))
	mustDo(t, os.Rename(replacement, file))
	s.next = func(context.Context, *Watcher) ([]Event, error) { return nil, io.EOF }
	ready := false
	err = s.observe(t.Context(), Request{Root: "project"}, func(_ context.Context, b Batch) error { ready = b.Ready; return nil })
	if err != nil || !ready {
		t.Fatal(err, ready)
	}
}

func TestExistingWatchRetainsDirectoryIdentityAcrossAncestorLinkChange(t *testing.T) {
	dir := t.TempDir()
	requireRootLinkSupport(t, dir)
	parent := filepath.Join(dir, "parent")
	root := filepath.Join(parent, "root")
	mustDo(t, os.MkdirAll(root, 0700))
	w, err := Open(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	moved := filepath.Join(dir, "moved")
	mustDo(t, os.Rename(parent, moved))
	mustDo(t, os.Symlink(moved, parent))
	mustDo(t, os.WriteFile(filepath.Join(moved, "root", "entry"), nil, 0600))
	expectEvents(t, w, Event{Op: "create", Path: "entry"})
}
