package omnigo

import (
	"context"
	"github.com/herlanai/omnigo/omniwindow"
)

type Display = omniwindow.Display
type ScreenRect = omniwindow.ScreenRect
type ScreenState = omniwindow.ScreenState

// Screen returns current monitors and cursor position after application readiness.
// It is worker-facing: do not synchronously call it from an OnReady/UI callback.
// Currently supported by the macOS host; it does not subscribe to display changes.
func (a *Application) Screen(ctx context.Context) (ScreenState, error) {
	h := a.host.Load()
	if h == nil {
		return ScreenState{}, ErrNotReady
	}
	return h.Screen(ctx)
}
