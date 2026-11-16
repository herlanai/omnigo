//go:build !ios && !android

package omnigo

import (
	"context"

	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
	"github.com/herlanai/omnigo/omniwindow/system"
)

type PathName = system.PathName

const (
	PathHome      = system.PathHome
	PathAppData   = system.PathAppData
	PathUserData  = system.PathUserData
	PathCache     = system.PathCache
	PathLogs      = system.PathLogs
	PathResources = system.PathResources
	PathTemp      = system.PathTemp
	PathExe       = system.PathExe
	PathDesktop   = system.PathDesktop
	PathDocuments = system.PathDocuments
	PathDownloads = system.PathDownloads
	PathMusic     = system.PathMusic
	PathPictures  = system.PathPictures
	PathVideos    = system.PathVideos
)

// Path resolves a standard application location. It works before Run and after
// shutdown; user-data, cache and log directories are created on demand.
func (a *Application) Path(name PathName) (string, error) { return a.paths.Get(name) }

// SetPath overrides a location without creating it. Empty restores the default.
func (a *Application) SetPath(name PathName, path string) error { return a.paths.Set(name, path) }

// Name is the stable application name used to scope its writable directories.
func (a *Application) Name() string { return a.options.Name }

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
