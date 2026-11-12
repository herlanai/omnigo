// Package host hosts Go control windows and an optional same-origin Web UI shell.
// Applications supply content; the framework owns native resources and dispatch.
package host

import (
	"context"
	"errors"
	"sync"

	"github.com/herlanai/omnigo/runtime/dispatch"
)

var ErrUnsupported = errors.New("host requires macOS amd64 or arm64; Windows/Linux WebView hosts are unavailable (software windows: omniwindow/nativewindow)")

type Controller struct {
	mu       sync.Mutex
	closed   bool
	queue    *dispatch.Queue
	ctx      context.Context
	wake     func()
	open     func(string, bool) error
	titlebar func(TitleBarRequest) (TitleBarState, error)
}

func (h *Controller) Open(ctx context.Context, id string, dark bool) error {
	if ctx == nil {
		return errors.New("host: nil context")
	}
	if h.open == nil {
		return ErrUnsupported
	}
	done, err := h.post(ctx, func() error { return h.open(id, dark) })
	if err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Serialize scheduling with shutdown so no worker can message a released native controller.
func (h *Controller) post(ctx context.Context, fn func() error) (<-chan error, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, dispatch.ErrClosed
	}
	if h.queue == nil || h.wake == nil {
		return nil, ErrUnsupported
	}
	done, err := h.queue.Submit(ctx, fn)
	if err == nil {
		h.wake()
	}
	return done, err
}
func (h *Controller) close() { h.mu.Lock(); defer h.mu.Unlock(); h.closed = true; h.queue.Close() }
