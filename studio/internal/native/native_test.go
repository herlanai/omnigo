package native

import (
	"context"
	"errors"
	"omnigo.dev/omnigo/runtime/dispatch"
	"sync"
	"testing"
)

func TestShutdownPreventsReleasedControllerWake(t *testing.T) {
	h := &Host{queue: dispatch.New(256), ctx: context.Background()}
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
