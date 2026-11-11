// Package native is Studio's experimental platform host. It is deliberately
// internal until native lifecycle, IME and accessibility contracts are complete.
package native

import (
	"context"
	"errors"
	"omnigo.dev/omnigo/runtime/dispatch"
	"sync"
)

var ErrUnsupported = errors.New("native Studio demos currently require macOS arm64")

type Host struct {
	mu     sync.Mutex
	closed bool
	queue  *dispatch.Queue
	ctx    context.Context
	wake   func()
	open   func(string, bool) error
}

func (h *Host) Open(ctx context.Context, id string, dark bool) error {
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
func (h *Host) post(ctx context.Context, fn func() error) (<-chan error, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, dispatch.ErrClosed
	}
	done, err := h.queue.Submit(ctx, fn)
	if err == nil {
		h.wake()
	}
	return done, err
}
func (h *Host) close() { h.mu.Lock(); defer h.mu.Unlock(); h.closed = true; h.queue.Close() }
