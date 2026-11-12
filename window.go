package omnigo

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/herlanai/omnigo/apps/host"
)

// ConfigureMainWindow configures the primary Web UI window. URL is an application-relative
// path, resolved against the framework's private loopback resource server.
// Additional Go content windows are opened through OpenContentWindow after startup.
func (a *Application) ConfigureMainWindow(options WindowOptions) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.used {
		return errors.New("omnigo: configure the primary window before Run")
	}
	if a.window != nil {
		return errors.New("omnigo: primary window already configured")
	}
	if a.options.Assets == nil {
		return errors.New("omnigo: Web UI window requires assets")
	}
	if options.URL == "" {
		options.URL = "/"
	}
	u, err := url.Parse(options.URL)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(options.URL, "/") || strings.HasPrefix(options.URL, "//") {
		return errors.New("omnigo: window URL must be an application-relative path")
	}
	if err = options.Validate(); err != nil {
		return err
	}
	if p := options.TrafficLightPosition; p != nil {
		copy := *p
		options.TrafficLightPosition = &copy
	}
	a.window = &options
	return nil
}

// OpenContentWindow and SetTitleBar are worker-facing calls. Do not wait for them from
// a content factory, UI callback or BeforeRender callback.
func (a *Application) OpenContentWindow(ctx context.Context, key string, dark bool) error {
	h := a.host.Load()
	if h == nil {
		return ErrNotReady
	}
	return h.Open(ctx, key, dark)
}
func (a *Application) SetTitleBar(ctx context.Context, r host.TitleBarRequest) (host.TitleBarState, error) {
	h := a.host.Load()
	if h == nil {
		return host.TitleBarState{}, ErrNotReady
	}
	return h.SetTitleBar(ctx, r)
}

// ContentWindowOptions describes native Go content; its callbacks run on the UI goroutine.
type ContentWindowOptions = host.ContentWindowOptions

// WindowEnvironment provides capabilities scoped to the content window. RunTask
// dispatches completion to the live window; do not retain transient paint state.
type WindowEnvironment struct {
	host.WindowServices
	Dark bool
}

// TitleBarState reads the primary window state without modifying it.
func (a *Application) TitleBarState(ctx context.Context) (host.TitleBarState, error) {
	h := a.host.Load()
	if h == nil {
		return host.TitleBarState{}, ErrNotReady
	}
	return h.TitleBarState(ctx)
}
