package omnigo

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/herlanai/omnigo/apps/host"
)

type driverFunc func(context.Context) error

func (f driverFunc) Run(ctx context.Context) error { return f(ctx) }
func (a *Application) Run(ctx context.Context) error {
	if !host.Available {
		return host.ErrUnsupported
	}
	return a.run(ctx, host.Run)
}
func (a *Application) run(ctx context.Context, native func(context.Context, host.Options) error) error {
	if ctx == nil {
		return errors.New("omnigo: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	if a.used {
		a.mu.Unlock()
		return errors.New("omnigo: application is single-use")
	}
	if a.window == nil && a.options.InitialWindow == "" {
		a.mu.Unlock()
		return errors.New("omnigo: configure a window before Run")
	}
	a.used = true
	a.mu.Unlock()
	defer a.host.Store(nil)
	return a.lifecycle.Run(ctx, driverFunc(func(ctx context.Context) error {
		options := host.Options{Icon: a.options.Icon, CreateWindow: a.options.CreateWindow, InitialWindow: a.options.InitialWindow, OnReady: func(h *host.Controller) { a.host.Store(h) }}
		if a.window == nil {
			return native(ctx, options)
		}
		return a.serveWindow(ctx, native, options)
	}))
}

// RunMain handles process termination signals and enters Run on the calling
// main thread. Embedded hosts can continue to use Run with their own context.
func (a *Application) RunMain() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}

// Run constructs and runs an application on the process main thread. Configuration
// errors are returned before any services start. Run handles termination signals.
func Run(options Options) error {
	app, err := New(options)
	if err != nil {
		return err
	}
	return app.RunMain()
}
