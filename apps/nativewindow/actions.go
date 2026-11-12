package nativewindow

import (
	"context"
	"errors"
	"math"
	"strings"
)

// Bounds uses client-area pixels in apps coordinates. Per-monitor DIP
// conversion is not yet part of the software driver.
type Bounds struct{ X, Y, Width, Height int }
type action struct {
	kind, text string
	bounds     Bounds
	flag       bool
	opacity    float64
}
type actionBackend interface{ apply(action) error }

func (w *Window) act(ctx context.Context, a action) (<-chan error, error) {
	return w.Post(ctx, func() error {
		b, ok := w.driver.(actionBackend)
		if !ok {
			return errors.New("nativewindow: window actions unavailable")
		}
		return b.apply(a)
	})
}

// SetTitle and the other actions are asynchronous and safe for worker callers.
// The returned channel reports the native operation's result. Do not wait for
// that channel from Ready/Event/Paint or any other UI callback.
func (w *Window) SetTitle(ctx context.Context, title string) (<-chan error, error) {
	if strings.ContainsRune(title, 0) {
		return nil, errors.New("nativewindow: title contains NUL")
	}
	return w.act(ctx, action{kind: "title", text: title})
}
func (w *Window) SetContentBounds(ctx context.Context, b Bounds) (<-chan error, error) {
	if !validSize(b.Width, b.Height) || b.X < -1<<30 || b.X > 1<<30 || b.Y < -1<<30 || b.Y > 1<<30 {
		return nil, errors.New("nativewindow: invalid bounds")
	}
	return w.act(ctx, action{kind: "bounds", bounds: b})
}
func (w *Window) Show(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "show"})
}
func (w *Window) ShowInactive(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "show-inactive"})
}
func (w *Window) Hide(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "hide"})
}
func (w *Window) Focus(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "focus"})
}
func (w *Window) Minimize(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "minimize"})
}
func (w *Window) Maximize(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "maximize"})
}
func (w *Window) Restore(ctx context.Context) (<-chan error, error) {
	return w.act(ctx, action{kind: "restore"})
}
func (w *Window) SetAlwaysOnTop(ctx context.Context, value bool) (<-chan error, error) {
	return w.act(ctx, action{kind: "top", flag: value})
}
func (w *Window) SetFullScreen(ctx context.Context, value bool) (<-chan error, error) {
	return w.act(ctx, action{kind: "fullscreen", flag: value})
}
func (w *Window) SetOpacity(ctx context.Context, value float64) (<-chan error, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return nil, errors.New("nativewindow: opacity must be between 0 and 1")
	}
	return w.act(ctx, action{kind: "opacity", opacity: value})
}

// Close closes without consulting ShouldClose, matching an explicit programmatic
// close. Native titlebar close requests consult ShouldClose on the UI goroutine.
func (w *Window) Close(ctx context.Context) (<-chan error, error) {
	return w.Post(ctx, func() error {
		w.closeRequested = true
		w.mu.Lock()
		defer w.mu.Unlock()
		w.closed = true
		w.queue.Close()
		return nil
	})
}
