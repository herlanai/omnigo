// Package native is Studio's experimental platform host. It is deliberately
// internal until native lifecycle, IME and accessibility contracts are complete.
package native

import (
	"context"
	"errors"
	"omnigo.dev/omnigo/runtime/dispatch"
)

var ErrUnsupported = errors.New("native Studio demos currently require macOS arm64")

type Host struct {
	queue *dispatch.Queue
	ctx   context.Context
	wake  func()
	open  func(string, bool) error
}

func (h *Host) Open(ctx context.Context, id string, dark bool) error {
	if h.open == nil {
		return ErrUnsupported
	}
	done, err := h.queue.Submit(ctx, func() error { return h.open(id, dark) })
	if err != nil {
		return err
	}
	h.wake()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
