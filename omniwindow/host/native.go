// Package host hosts Go control windows and an optional same-origin Web UI shell.
// Applications supply content; the framework owns native resources and dispatch.
package host

import (
	"context"
	"errors"
	"sync"

	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
)

var ErrUnsupported = errors.New("host requires macOS amd64 or arm64; Windows/Linux WebView hosts are unavailable (software windows: omniwindow/nativewindow)")

type Controller struct {
	subscriptions   map[*SystemSubscription]struct{}
	watchSystem     func(func(omniwindow.SystemEventKind)) (func(), error)
	stopSystemWatch func()
	systemSignals   []omniwindow.SystemEventKind
	systemScheduled bool
	systemMissed    uint64
	systemSequence  uint64
	trays           map[*Tray]struct{}
	newTray         func(omniwindow.TrayOptions) (trayDriver, error)
	mu              sync.Mutex
	closed          bool
	controls        []func()
	dialog          *dialogOperation
	startDialog     func(dialogRequest, func(dialogResult)) (func(), error)
	queue           *dispatch.Queue
	ctx             context.Context
	wake            func()
	open            func(string, bool) error
	activate        func() error
	titlebar        func(TitleBarRequest) (TitleBarState, error)
	screen          func() (omniwindow.ScreenState, error)
	appearance      func(omniwindow.AppearanceSource) (omniwindow.AppearanceState, error)
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
func (h *Controller) close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	controls := h.controls
	h.controls = nil
	h.mu.Unlock()
	h.queue.Close()
	for _, fn := range controls {
		fn()
	}
	h.closeDialog()
	for sub := range h.subscriptions {
		h.removeSubscription(sub)
	}
	for tray := range h.trays {
		tray.closeUI()
	}
}

// Post schedules work on the UI goroutine. Do not wait for its result from a UI
// callback. Cancellation and shutdown use the same queue as window operations.
func (h *Controller) Post(ctx context.Context, fn func() error) (<-chan error, error) {
	return h.post(ctx, fn)
}

// Activate requests foreground activation and restoration of the front window.
// It is safe to enqueue from UI callbacks, but never wait on the result there.
// OS focus policies can deny activation even after accepting the request.
func (h *Controller) Activate(ctx context.Context) (<-chan error, error) {
	return h.post(ctx, func() error {
		if h.activate == nil {
			return ErrUnsupported
		}
		return h.activate()
	})
}
