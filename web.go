//go:build !ios && !android

package omnigo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/herlanai/omnigo/internal/apphost"
	"github.com/herlanai/omnigo/internal/codegen"
	"github.com/herlanai/omnigo/internal/runtime/rpc"
	"github.com/herlanai/omnigo/omniweb"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/host"
)

// Registration registers an application's typed RPC methods and routes before Run.
// Registration must not allocate live resources; use Services for those.
type Registration func(*Application) error

// Group groups registrations and returns the first configuration error. Like
// New, it performs no resource startup; a failed application should be discarded.
func Group(modules ...Registration) Registration {
	return func(app *Application) error {
		for _, module := range modules {
			if module == nil {
				return fmt.Errorf("omnigo: nil registration")
			}
			if err := module(app); err != nil {
				return err
			}
		}
		return nil
	}
}

// Bind registers a typed service before Run. Existing RPC validation, concurrency
// limits and permission checks are shared with OmniWeb.
func Bind[Request, Response any](app *Application, name, permission string, handler func(context.Context, Request) (Response, error)) error {
	if app == nil {
		return errors.New("omnigo: nil application")
	}
	app.mu.Lock()
	defer app.mu.Unlock()
	if app.used {
		return errors.New("omnigo: configure services before Run")
	}
	return rpc.Bind(app.router, name, permission, handler)
}

// Method binds one explicitly named, statically typed RPC method. No reflection
// or automatic export of an object's other methods is involved.
func Method[Request, Response any](name string, handler func(context.Context, Request) (Response, error), permission ...string) Registration {
	return func(app *Application) error {
		if len(permission) > 1 {
			return fmt.Errorf("omnigo: %s accepts one permission", name)
		}
		grant := ""
		if len(permission) == 1 {
			grant = permission[0]
		}
		return Bind(app, name, grant, handler)
	}
}

// Query binds a read operation with an empty JSON object request.
func Query[Response any](name string, read func() Response, permission ...string) Registration {
	return func(app *Application) error {
		if read == nil {
			return fmt.Errorf("omnigo: %s requires a reader", name)
		}
		return Method(name, func(_ context.Context, _ struct{}) (Response, error) { return read(), nil }, permission...)(app)
	}
}

func (a *Application) Handle(pattern string, handler http.Handler) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.used {
		return errors.New("omnigo: configure routes before Run")
	}
	if pattern == "/" || strings.HasPrefix(pattern, omniweb.Prefix) {
		return errors.New("omnigo: reserved framework route")
	}
	defer func() {
		if recover() != nil {
			err = errors.New("omnigo: invalid or conflicting HTTP route")
		}
	}()
	a.routes.Handle(pattern, handler)
	return nil
}

// Route adds an application HTTP handler, retaining framework route validation.
func Route(pattern string, handler http.Handler) Registration {
	return func(app *Application) error { return app.Handle(pattern, handler) }
}

func (a *Application) serveWindow(ctx context.Context, native func(context.Context, host.Options) error, options host.Options) error {
	return apphost.ServeWindow(ctx, native, options, apphost.WebOptions{
		Assets: a.options.Assets, Router: a.router, Routes: a.routes,
		Streams: a.options.Streams, Events: a.options.Events,
		Permissions: a.options.Lifecycle.Permissions, Window: *a.window,
	})
}

// GenerateTypeScript snapshots registered unary RPC contracts without starting
// services or a window. Bind all application methods before generating.
func (a *Application) GenerateTypeScript() ([]byte, error) {
	return omniweb.GenerateTypeScript(a.router)
}

// WriteTypeScript updates a generated Services contract. Identical content is
// untouched; generation errors leave the previous file intact. Replacement uses
// a same-directory rename (atomic on Unix; other filesystems follow os.Rename).
func (a *Application) WriteTypeScript(path string) error {
	return codegen.WriteTypeScript(a.router, path)
}

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
