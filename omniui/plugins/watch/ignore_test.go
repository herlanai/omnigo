package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIgnorePrunesDirectoriesAndFiltersAllEventKinds(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hidden", "deep"), 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "hidden", "deep", "secret"), nil, 0600)
	w, err := Open(dir, Options{Recursive: true, Ignore: func(path string, isDir bool) bool {
		if strings.HasPrefix(path, "hidden/") {
			t.Fatal("entered ignored directory", path)
		}
		return (path == "hidden" && isDir) || strings.HasSuffix(path, ".tmp")
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	os.WriteFile(filepath.Join(dir, "draft.tmp"), []byte("private"), 0600)
	os.WriteFile(filepath.Join(dir, "public"), []byte("public"), 0600)
	events, err := w.Poll(t.Context())
	if err != nil || len(events) != 1 || events[0].Path != "public" {
		t.Fatal(events, err)
	}
	os.Rename(filepath.Join(dir, "public"), filepath.Join(dir, "public.tmp"))
	events, err = w.Poll(t.Context())
	if err != nil || len(events) != 1 || events[0].Op != "remove" || events[0].Path != "public" {
		t.Fatal(events, err)
	}
	os.Rename(filepath.Join(dir, "draft.tmp"), filepath.Join(dir, "published"))
	events, err = w.Poll(t.Context())
	if err != nil || len(events) != 1 || events[0].Op != "create" || events[0].Path != "published" {
		t.Fatal(events, err)
	}
}
func TestIgnoreFailurePreservesSnapshot(t *testing.T) {
	dir := t.TempDir()
	fail := false
	w, err := Open(dir, Options{Ignore: func(string, bool) bool {
		if fail {
			panic("private details")
		}
		return false
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	os.WriteFile(filepath.Join(dir, "new"), nil, 0600)
	fail = true
	if _, err = w.Poll(t.Context()); err == nil || strings.Contains(err.Error(), "private details") {
		t.Fatal(err)
	}
	fail = false
	events, err := w.Poll(t.Context())
	if err != nil || len(events) != 1 || events[0].Op != "create" {
		t.Fatal(events, err)
	}
}
func TestServiceIgnoreUsesNamedRootRelativePaths(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "sub"), 0700)
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {
		Path: dir, Recursive: true, Ignore: func(path string, isDir bool) bool {
			if path == "secret" {
				t.Fatal("lost request prefix")
			}
			return path == "sub/secret"
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Poll(ctx) }
	stop := errors.New("test complete")
	err = s.observe(t.Context(), Request{Root: "project", Path: "sub", Recursive: true}, func(_ context.Context, b Batch) error {
		if b.Ready {
			os.WriteFile(filepath.Join(dir, "sub", "secret"), nil, 0600)
			return os.WriteFile(filepath.Join(dir, "sub", "visible"), nil, 0600)
		}
		if len(b.Events) != 1 || b.Events[0].Path != "visible" {
			t.Fatal(b)
		}
		return stop
	})
	if !errors.Is(err, stop) || s.active != 0 {
		t.Fatal(err, s.active)
	}
}
func TestIgnoringRequestedDirectoryDoesNotScanDescendants(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "private"), 0700)
	w, err := Open(dir, Options{Recursive: true, Ignore: func(path string, _ bool) bool {
		if path != "." {
			t.Fatal("scanned descendants")
		}
		return true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if events, err := w.Poll(t.Context()); err != nil || len(events) != 0 {
		t.Fatal(events, err)
	}
}
