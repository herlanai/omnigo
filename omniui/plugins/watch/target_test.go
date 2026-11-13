package watch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRequestedTargetRejectsLinksWithinAndOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	mustDo(t, os.MkdirAll(filepath.Join(dir, "real", "nested"), 0700))
	mustDo(t, os.WriteFile(filepath.Join(dir, "real", "file"), nil, 0600))
	for _, link := range []struct{ name, target string }{
		{"alias", filepath.Join(dir, "real")},
		{"file-alias", filepath.Join(dir, "real", "file")},
		{"outside", t.TempDir()},
	} {
		if err := os.Symlink(link.target, filepath.Join(dir, link.name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir, Recursive: true}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(context.Context, *Watcher) ([]Event, error) { return nil, io.EOF }
	for _, name := range []string{"alias", "alias/nested", "alias/file", "file-alias", "outside"} {
		err = s.observe(t.Context(), Request{Root: "project", Path: name}, func(context.Context, Batch) error { t.Fatal("link watch started", name); return nil })
		if !errors.Is(err, ErrDenied) {
			t.Fatal(name, err)
		}
		if s.active != 0 {
			t.Fatal("denied request retained quota")
		}
	}
	for _, name := range []string{"real", "real/nested", "real/file"} {
		ready := false
		err = s.observe(t.Context(), Request{Root: "project", Path: name}, func(_ context.Context, b Batch) error { ready = b.Ready; return nil })
		if err != nil || !ready {
			t.Fatal(name, err)
		}
	}
}

func TestApplicationRootLinkIsCanonicalizedButPageLinkIsDenied(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual")
	mustDo(t, os.Mkdir(actual, 0700))
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, dynamic := range []bool{false, true} {
		opts := ServiceOptions{}
		root := Root{Path: alias, Recursive: true}
		if dynamic {
			opts.Resolve = func(context.Context, string) (Root, bool) { return root, true }
		} else {
			opts.Roots = map[string]Root{"project": root}
		}
		s, err := NewService(opts)
		if err != nil {
			t.Fatal(err)
		}
		s.next = func(context.Context, *Watcher) ([]Event, error) { return nil, io.EOF }
		ready := false
		err = s.observe(t.Context(), Request{Root: "project"}, func(_ context.Context, b Batch) error { ready = b.Ready; return nil })
		s.Close()
		if err != nil || !ready {
			t.Fatal(dynamic, err)
		}
	}
}

func TestAllowHookCannotSubstituteLinkForRequestedDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	mustDo(t, os.Mkdir(target, 0700))
	// Probe platform permission to create links before entering the Allow hook.
	probe := filepath.Join(dir, "probe")
	if err := os.Symlink(target, probe); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	mustDo(t, os.Remove(probe))
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir, Recursive: true}}, Allow: func(context.Context, Request) bool {
		mustDo(t, os.Rename(target, filepath.Join(dir, "moved")))
		mustDo(t, os.Symlink(filepath.Join(dir, "moved"), target))
		return true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	err = s.observe(t.Context(), Request{Root: "project", Path: "target"}, func(context.Context, Batch) error { t.Fatal("substituted link watched"); return nil })
	if !errors.Is(err, ErrDenied) || s.active != 0 {
		t.Fatal(err, s.active)
	}
}

func TestRecursiveGrantAllowsFileTargetWithRecursiveRequest(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	mustDo(t, os.WriteFile(file, nil, 0600))
	s, err := NewService(ServiceOptions{Roots: map[string]Root{
		"directory": {Path: dir, Recursive: true},
		"single":    {Path: file, Recursive: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Poll(ctx) }
	for _, request := range []Request{{Root: "single", Recursive: true}, {Root: "directory", Path: "file", Recursive: true}} {
		mustDo(t, os.WriteFile(file, nil, 0600))
		batches := 0
		err = s.observe(t.Context(), request, func(_ context.Context, b Batch) error {
			batches++
			if b.Ready {
				return os.Remove(file)
			}
			if len(b.Events) != 1 || b.Events[0].Op != "remove" || b.Events[0].Path != "." {
				t.Fatal(b)
			}
			return nil
		})
		if err != nil || batches != 2 {
			t.Fatal(request, err, batches)
		}
	}
}
