package host

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/herlanai/omnigo/internal/runtime/dispatch"
)

func TestShutdownPreventsReleasedControllerWake(t *testing.T) {
	h := &Controller{queue: dispatch.New(256), ctx: context.Background()}
	released := false
	h.wake = func() {
		if released {
			t.Error("message sent to released controller")
		}
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() { _, _ = h.post(context.Background(), func() error { return nil }) })
	}
	h.close()
	h.mu.Lock()
	released = true
	h.mu.Unlock()
	wg.Wait()
	if _, err := h.post(context.Background(), func() error { return nil }); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatalf("got %v", err)
	}
}

func TestPublicPostRunsOnlyWhenUIDrains(t *testing.T) {
	q := dispatch.New(1)
	h := &Controller{queue: q, wake: func() {}}
	defer h.close()
	called := false
	done, err := h.Post(t.Context(), func() error { called = true; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("callback ran on submitting thread")
	}
	q.Drain(1)
	if err := <-done; err != nil || !called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

func TestActivationDispatchCancellationAndClose(t *testing.T) {
	q := dispatch.New(4)
	calls := 0
	h := &Controller{queue: q, wake: func() {}, activate: func() error { calls++; return nil }}
	done, err := h.Activate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("activation not queued")
	}
	q.Drain(1)
	if err := <-done; err != nil || calls != 1 {
		t.Fatal(err, calls)
	}
	ctx, cancel := context.WithCancel(t.Context())
	canceled, err := h.Activate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	q.Drain(1)
	if err := <-canceled; !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(err, calls)
	}
	h.close()
	if _, err := h.Activate(t.Context()); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatal(err)
	}
	if _, err := (&Controller{queue: dispatch.New(1), wake: func() {}}).Activate(nil); err == nil {
		t.Fatal("nil context accepted")
	}
}
