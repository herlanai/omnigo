package watch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveIsPerRequestAndCanBeRevoked(t *testing.T) {
	dir := t.TempDir()
	allowed := true
	calls := 0
	s, err := NewService(ServiceOptions{Resolve: func(_ context.Context, name string) (Root, bool) {
		calls++
		if name != "selected" {
			t.Fatal(name)
		}
		return Root{Path: dir, Recursive: true}, allowed
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stop := errors.New("observed ready")
	err = s.observe(t.Context(), Request{Root: "selected"}, func(context.Context, Batch) error { return stop })
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatal(err, calls)
	}
	allowed = false
	if err = s.observe(t.Context(), Request{Root: "selected"}, nil); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if calls != 2 || s.active != 0 || len(s.roots) != 0 {
		t.Fatal("dynamic root cached or quota retained", calls, s.active, len(s.roots))
	}
}
func TestConfiguredRootWinsAndAllowCanRefuseDynamicRoot(t *testing.T) {
	dir := t.TempDir()
	resolveCalls := 0
	allowCalls := 0
	s, err := NewService(ServiceOptions{
		Roots:   map[string]Root{"fixed": {Path: dir}},
		Resolve: func(context.Context, string) (Root, bool) { resolveCalls++; return Root{Path: dir}, true },
		Allow:   func(context.Context, Request) bool { allowCalls++; return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, name := range []string{"fixed", "selected"} {
		if err = s.observe(t.Context(), Request{Root: name}, nil); !errors.Is(err, ErrDenied) {
			t.Fatal(err)
		}
	}
	if resolveCalls != 1 || allowCalls != 2 || s.active != 0 {
		t.Fatal(resolveCalls, allowCalls, s.active)
	}
}
func TestCloseCancelsResolverAndQuotaIncludesResolution(t *testing.T) {
	entered := make(chan struct{})
	s, err := NewService(ServiceOptions{MaxWatches: 1, Resolve: func(ctx context.Context, _ string) (Root, bool) {
		close(entered)
		<-ctx.Done()
		return Root{}, false
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	done := make(chan error, 1)
	go func() { done <- s.observe(t.Context(), Request{Root: "selected"}, nil) }()
	<-entered
	if err = s.observe(t.Context(), Request{Root: "another"}, nil); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	s.Close()
	if err = <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.active != 0 {
		t.Fatal("resolving request retained quota")
	}
}
func TestCanceledResolutionCannotStartWatch(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s, err := NewService(ServiceOptions{Resolve: func(context.Context, string) (Root, bool) { cancel(); return Root{Path: dir}, true }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.observe(ctx, Request{Root: "selected"}, func(context.Context, Batch) error { t.Fatal("started canceled watch"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.active != 0 {
		t.Fatal("slot leaked")
	}
}
func TestDynamicRootStillEnforcesRecursionAndContainment(t *testing.T) {
	dir := t.TempDir()
	s, err := NewService(ServiceOptions{Resolve: func(context.Context, string) (Root, bool) { return Root{Path: dir}, true }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.observe(t.Context(), Request{Root: "selected", Recursive: true}, nil); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
	if err = os.Symlink(t.TempDir(), filepath.Join(dir, "escape")); err != nil {
		t.Skip(err)
	}
	if err = s.observe(t.Context(), Request{Root: "selected", Path: "escape"}, nil); !errors.Is(err, ErrDenied) {
		t.Fatal(err)
	}
}
func TestInvalidDynamicRootRejected(t *testing.T) {
	for _, path := range []string{"relative", ""} {
		s, err := NewService(ServiceOptions{Resolve: func(context.Context, string) (Root, bool) { return Root{Path: path}, true }})
		if err != nil {
			t.Fatal(err)
		}
		if err = s.observe(t.Context(), Request{Root: "selected"}, nil); !errors.Is(err, ErrDenied) {
			t.Fatal(path, err)
		}
		s.Close()
	}
}

func TestAllowCanCloseServiceWithoutDeadlockOrStartingWatch(t *testing.T) {
	var s *Service
	var err error
	s, err = NewService(ServiceOptions{
		Resolve: func(context.Context, string) (Root, bool) { return Root{Path: t.TempDir()}, true },
		Allow:   func(context.Context, Request) bool { s.Close(); return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.observe(t.Context(), Request{Root: "selected"}, func(context.Context, Batch) error { t.Fatal("watch started after Close"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
