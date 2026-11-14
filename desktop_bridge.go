package omnigo

import (
	"context"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
)

// DesktopBridge explicitly exposes existing native services through OmniWeb.
// Every method requires desktop.read or desktop.dialog permission. The caller
// must grant these in Lifecycle.Permissions for trusted application content.
// No service is exposed merely by importing the framework. Handlers execute on
// RPC workers and use the host's cancelable UI dispatch path.
func DesktopBridge() Registration {
	return func(a *Application) error {
		return Group(
			Query("desktop.platform", omniwindow.CurrentPlatform, "desktop.read"),
			Method("desktop.screen", func(ctx context.Context, _ struct{}) (ScreenState, error) { return a.Screen(ctx) }, "desktop.read"),
			Method("desktop.appearance", func(ctx context.Context, _ struct{}) (AppearanceState, error) { return a.Appearance(ctx) }, "desktop.read"),
			Method("desktop.titlebar", func(ctx context.Context, _ struct{}) (host.TitleBarState, error) { return a.TitleBarState(ctx) }, "desktop.read"),
			Method("desktop.dialog.open", a.OpenDialog, "desktop.dialog"),
			Method("desktop.dialog.save", a.SaveDialog, "desktop.dialog"),
			Method("desktop.dialog.message", a.MessageDialog, "desktop.dialog"),
		)(a)
	}
}
