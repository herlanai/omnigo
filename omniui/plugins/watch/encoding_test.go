package watch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestServiceRejectsUnrepresentableBatchAndReleasesWatch(t *testing.T) {
	for _, bad := range []Event{
		{Op: "create", Path: "bad\xff"},
		{Op: "rename", Path: "valid", OldPath: "bad\xff"},
	} {
		t.Run(bad.Op, func(t *testing.T) {
			s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: t.TempDir()}}, MaxWatches: 1, MaxWatchesPerSource: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			var watcher *Watcher
			s.next = func(_ context.Context, w *Watcher) ([]Event, error) {
				watcher = w
				return []Event{{Op: "create", Path: "valid"}, bad}, nil
			}
			batches := 0
			err = s.observe(t.Context(), Request{Root: "project"}, func(_ context.Context, b Batch) error {
				batches++
				if !b.Ready {
					t.Fatal("partially emitted invalid batch", b)
				}
				return nil
			})
			if !errors.Is(err, ErrEncoding) || batches != 1 {
				t.Fatal(err, batches)
			}
			if _, err := watcher.Poll(t.Context()); !errors.Is(err, ErrClosed) {
				t.Fatal("watcher retained", err)
			}
			if s.active != 0 || len(s.bySource) != 0 || len(s.reservations) != 0 {
				t.Fatal("reservation retained")
			}
			s.next = func(context.Context, *Watcher) ([]Event, error) { return nil, io.EOF }
			if err := s.observe(t.Context(), Request{Root: "project"}, func(context.Context, Batch) error { return nil }); err != nil {
				t.Fatal("replacement subscription denied", err)
			}
		})
	}
}

func TestFilesystemInvalidUTF8RenameIsPreservedLocallyAndRejectedByService(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows filenames use UTF-16")
	}
	dir := t.TempDir()
	old := "old\xff"
	if err := os.WriteFile(filepath.Join(dir, old), []byte("data"), 0600); err != nil {
		t.Skipf("filesystem does not accept invalid UTF-8: %v", err)
	}
	local, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer local.Close()
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: dir}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.next = func(ctx context.Context, w *Watcher) ([]Event, error) { return w.Poll(ctx) }
	err = s.observe(t.Context(), Request{Root: "project"}, func(_ context.Context, b Batch) error {
		if !b.Ready {
			t.Fatal("corrupted path sent", b)
		}
		return os.Rename(filepath.Join(dir, old), filepath.Join(dir, "new"))
	})
	if !errors.Is(err, ErrEncoding) {
		t.Fatal(err)
	}
	events, err := local.Poll(t.Context())
	if err != nil || len(events) != 1 || events[0].Op != "rename" || events[0].Path != "new" || events[0].OldPath != old {
		t.Fatal("local byte path was lost", events, err)
	}
}

func TestServicePreservesValidUnicodeAndReplacementCharacter(t *testing.T) {
	s, err := NewService(ServiceOptions{Roots: map[string]Root{"project": {Path: t.TempDir()}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	want := Event{Op: "rename", Path: "文件😀�", OldPath: "旧文件"}
	calls := 0
	s.next = func(context.Context, *Watcher) ([]Event, error) {
		calls++
		if calls == 1 {
			return []Event{want}, nil
		}
		return nil, io.EOF
	}
	batches := 0
	err = s.observe(t.Context(), Request{Root: "project"}, func(_ context.Context, b Batch) error {
		batches++
		if !b.Ready && (len(b.Events) != 1 || b.Events[0] != want) {
			t.Fatal(b)
		}
		return nil
	})
	if err != nil || batches != 2 {
		t.Fatal(err, batches)
	}
}
