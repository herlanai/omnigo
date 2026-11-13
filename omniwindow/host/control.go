package host

import (
	"context"
	"errors"
)

// control schedules bounded-lifetime resource cleanup independently of the
// ordinary work queue's capacity. Each operation enqueues cancellation at most
// once. Wake and shutdown are serialized exactly as for ordinary requests.
func (h *Controller) control(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.controls = append(h.controls, fn)
	if h.wake != nil {
		h.wake()
	}
}
func (h *Controller) drain() {
	h.mu.Lock()
	controls := h.controls
	h.controls = nil
	h.mu.Unlock()
	for _, fn := range controls {
		fn()
	}
	h.queue.Drain(64)
}

// createUI reconciles a creation that completes after its worker stops waiting.
// Rollback is queued only after admission, so a full request queue cannot cause
// an unbounded stream of unnecessary cleanup tasks.
func (h *Controller) createUI(ctx context.Context, create func() error, rollback func()) error {
	if ctx == nil {
		return errors.New("host: nil context")
	}
	done, err := h.post(ctx, create)
	if err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		h.control(rollback)
		return ctx.Err()
	}
}

// callUI waits for ordinary UI work. Cancellation does not undo a mutation
// already running; creation must instead use createUI for rollback.
func (h *Controller) callUI(ctx context.Context, fn func() error) error {
	if ctx == nil {
		return errors.New("host: nil context")
	}
	done, err := h.post(ctx, fn)
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
