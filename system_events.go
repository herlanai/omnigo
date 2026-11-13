package omnigo

import (
	"context"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
)

type SystemEvent = omniwindow.SystemEvent
type SystemEventKind = omniwindow.SystemEventKind
type SystemSubscription = host.SystemSubscription

const (
	SystemDisplaysChanged   = omniwindow.SystemDisplaysChanged
	SystemAppearanceChanged = omniwindow.SystemAppearanceChanged
	SystemWillSleep         = omniwindow.SystemWillSleep
	SystemDidWake           = omniwindow.SystemDidWake
	SystemScreensDidSleep   = omniwindow.SystemScreensDidSleep
	SystemScreensDidWake    = omniwindow.SystemScreensDidWake
	SystemSessionActive     = omniwindow.SystemSessionActive
	SystemSessionInactive   = omniwindow.SystemSessionInactive
)

// WatchSystemEvents subscribes to native macOS display, appearance, accessibility,
// sleep/wake and session notifications. Worker-facing; context owns lifetime.
// Query Screen/Appearance for initial state; subscriptions report later events.
func (a *Application) WatchSystemEvents(ctx context.Context) (*SystemSubscription, error) {
	h := a.host.Load()
	if h == nil {
		return nil, ErrNotReady
	}
	return h.WatchSystemEvents(ctx)
}
