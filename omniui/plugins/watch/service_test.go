package watch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceSnapshotEventsAndCancellation(t *testing.T) {
	dir := t.TempDir()
	roots := map[string]Root{"project": {Path: dir, Recursive: true}}
	s, err := NewService(ServiceOptions{Roots: roots})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	delete(roots, "project") // configuration must not retain mutable map
	s.next = func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Poll(ctx) }
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var batches int
	err = s.observe(ctx, Request{Root: "project"}, func(_ context.Context, b Batch) error {
		batches++
		if b.Ready {
			if len(b.Events) != 0 {
				t.Fatal(b)
			}
			return os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0600)
		}
		if len(b.Events) != 1 || b.Events[0].Path != "hello.txt" || b.Events[0].Op != "create" {
			t.Fatal(b)
		}
		raw, _ := json.Marshal(b.Events[0])
		if string(raw) != `{"isDir":false,"op":"create","path":"hello.txt"}` {
			t.Fatal(string(raw))
		}
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) || batches != 2 || s.active != 0 {
		t.Fatal(err, batches, s.active)
	}
}
func TestServiceRootPermissionAndTraversal(t *testing.T) {
	dir := t.TempDir()
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, r := range []Request{{Root: "missing"}, {Root: "project", Recursive: true}, {Root: "project", Path: "../outside"}, {Root: "project", Path: "/tmp"}, {Root: "project", Path: `..\outside`}} {
		err = s.observe(t.Context(), r, func(context.Context, Batch) error { t.Fatal("unauthorized watch started"); return nil })
		if !errors.Is(err, ErrDenied) {
			t.Fatal(r, err)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "escape")); err == nil {
		if err = s.observe(t.Context(), Request{Root: "project", Path: "escape"}, func(context.Context, Batch) error { t.Fatal("symlink escaped"); return nil }); !errors.Is(err, ErrDenied) {
			t.Fatal(err)
		}
	}
	s.Close()
	if err = s.observe(t.Context(), Request{Root: "project"}, nil); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}
func TestServiceQuotaAndCloseWakeActiveWatch(t *testing.T) {
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: t.TempDir()}}, MaxWatches: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	entered := make(chan struct{})
	s.next = func(ctx context.Context, _ *Watcher) ([]Event, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() {
		done <- s.observe(t.Context(), Request{Root: "project"}, func(context.Context, Batch) error { return nil })
	}()
	<-entered
	if err = s.observe(t.Context(), Request{Root: "project"}, nil); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	s.Close()
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.active != 0 {
		t.Fatal("quota leaked")
	}
}
func TestServiceAllowHookCannotGrantUnknownRoots(t *testing.T) {
	s, err := NewService(ServiceOptions{Allow: func(context.Context, Request) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.observe(t.Context(), Request{Root: "unknown"}, nil); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
}
