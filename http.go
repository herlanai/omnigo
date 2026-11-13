package omnigo

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/herlanai/omnigo/internal/apphost"
	"github.com/herlanai/omnigo/omniweb"
	"github.com/herlanai/omnigo/omniwindow/host"
)

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
