package watch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestShallowRootCannotGrantChildDirectoryOrDescendantFile(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		name := "configured"
		if dynamic {
			name = "resolved"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			mustDo(t, os.Mkdir(filepath.Join(dir, "sub"), 0700))
			mustDo(t, os.WriteFile(filepath.Join(dir, "file"), nil, 0600))
			mustDo(t, os.WriteFile(filepath.Join(dir, "sub", "file"), nil, 0600))
			allowCalls := 0
			root := Root{Path: dir}
			options := ServiceOptions{Allow: func(context.Context, Request) bool { allowCalls++; return true }}
			if dynamic {
				options.Resolve = func(context.Context, string) (Root, bool) { return root, true }
			} else {
				options.Roots = map[string]Root{"project": root}
			}
			s, err := NewService(options)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.next = func(context.Context, *Watcher) ([]Event, error) { return nil, io.EOF }
			for _, path := range []string{"sub", "sub/file"} {
				err = s.observe(t.Context(), Request{Root: "project", Path: path}, func(context.Context, Batch) error { t.Fatal("denied watch started"); return nil })
				if !errors.Is(err, ErrDenied) {
					t.Fatal(path, err)
				}
			}
			if allowCalls != 0 {
				t.Fatal("hook was allowed to widen root grant", allowCalls)
			}
			for _, path := range []string{".", "file"} {
				ready := false
				err = s.observe(t.Context(), Request{Root: "project", Path: path}, func(_ context.Context, b Batch) error { ready = b.Ready; return nil })
				if err != nil || !ready {
					t.Fatal("shallow target refused", path, err)
				}
			}
			if allowCalls != 2 || s.active != 0 {
				t.Fatal(allowCalls, s.active)
			}
		})
	}
}
