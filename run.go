package omnigo

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/herlanai/omnigo/internal/apphost"
	"github.com/herlanai/omnigo/omniwindow/host"
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
	defer func() { a.host.Store(nil); close(a.done) }()
	lease, forwarded, err := a.acquireInstance(ctx)
	if err != nil {
		return err
	}
	if forwarded {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ready := make(chan apphost.LaunchDispatcher, 1)
	var delivered chan error
	if lease != nil {
		defer lease.Close()
		delivered = make(chan error, 1)
		go func() {
			err := apphost.DeliverLaunches(runCtx, lease, ready, a.options.SingleInstance.OnLaunch)
			delivered <- err
			if err != nil {
				cancel()
			}
		}()
	}
	err = a.lifecycle.Run(runCtx, driverFunc(func(ctx context.Context) error {
		options := host.Options{Icon: a.options.Icon, CreateWindow: a.options.CreateWindow, InitialWindow: a.options.InitialWindow, OnReady: func(h *host.Controller) {
			a.host.Store(h)
			a.readyOnce.Do(func() {
				close(a.ready)
				if a.options.OnReady != nil {
					a.options.OnReady(a)
				}
			})
			if lease != nil {
				ready <- h
			}
		}}
		if a.window == nil {
			return native(ctx, options)
		}
		return a.serveWindow(ctx, native, options)
	}))
	cancel()
	if delivered != nil {
		err = errors.Join(err, <-delivered)
	}
	return err
}

// RunMain handles process termination signals and enters Run on the calling
// main thread. Embedded hosts can continue to use Run with their own context.
func (a *Application) RunMain() error {
	if output := os.Getenv("OMNIGO_GENERATE"); output != "" {
		return a.WriteTypeScript(output)
	}
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
