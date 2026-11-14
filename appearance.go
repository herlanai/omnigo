//go:build !ios && !android

package omnigo

import (
	"context"
	"github.com/herlanai/omnigo/omniwindow"
)

type AppearanceSource = omniwindow.AppearanceSource
type AppearanceState = omniwindow.AppearanceState

const (
	AppearanceSystem = omniwindow.AppearanceSystem
	AppearanceLight  = omniwindow.AppearanceLight
	AppearanceDark   = omniwindow.AppearanceDark
)

// Appearance reads the macOS application's native appearance and accessibility
// settings. Worker-only; wait for readiness before calling.
func (a *Application) Appearance(ctx context.Context) (AppearanceState, error) {
	h := a.host.Load()
	if h == nil {
		return AppearanceState{}, ErrNotReady
	}
	return h.Appearance(ctx)
}

// SetAppearance overrides native appearance for this application. System clears
// the override. This does not update Go surface themes or frontend CSS. Call on
// a worker, never synchronously from a UI callback. Currently macOS-only.
func (a *Application) SetAppearance(ctx context.Context, source AppearanceSource) (AppearanceState, error) {
	h := a.host.Load()
	if h == nil {
		return AppearanceState{}, ErrNotReady
	}
	return h.SetAppearance(ctx, source)
}
