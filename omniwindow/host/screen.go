package host

import (
	"context"
	"errors"
	"slices"

	"github.com/herlanai/omnigo/omniwindow"
)

// Screen reads current monitor geometry and cursor position on the UI queue.
// Call from a worker; never wait on the UI goroutine from a UI callback.
func (h *Controller) Screen(ctx context.Context) (omniwindow.ScreenState, error) {
	if ctx == nil {
		return omniwindow.ScreenState{}, errors.New("host: nil context")
	}
	var result omniwindow.ScreenState
	done, err := h.post(ctx, func() error {
		if h.screen == nil {
			return ErrUnsupported
		}
		var e error
		result, e = h.screen()
		result.Displays = slices.Clone(result.Displays)
		return e
	})
	if err != nil {
		return omniwindow.ScreenState{}, err
	}
	select {
	case err := <-done:
		if err != nil {
			return omniwindow.ScreenState{}, err
		}
		return result, nil
	case <-ctx.Done():
		return omniwindow.ScreenState{}, ctx.Err()
	}
}

// AppKit uses a bottom-left desktop origin; convert both frames and visible
// frames using the primary screen's top edge, not each display's own height.
func screenRect(x, y, width, height, primaryTop float64) omniwindow.ScreenRect {
	return omniwindow.ScreenRect{X: x, Y: primaryTop - y - height, Width: width, Height: height}
}
