//go:build !ios && !android

package omnigo

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/herlanai/omnigo/internal/apphost"
	"github.com/herlanai/omnigo/internal/runtime/lifecycle"
	"github.com/herlanai/omnigo/internal/startup"
	"github.com/herlanai/omnigo/omniwindow/host"
	"github.com/herlanai/omnigo/omniwindow/instance"
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
				go func() {
					if startup.Confirm(ctx) != nil {
						cancel()
					}
				}()
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

// ErrStopped means this single-use application has finished its run attempt.
var ErrStopped = errors.New("omnigo: application stopped")

// WhenReady waits for native readiness, cancellation, or application exit.
// Call from a worker, never from the main thread that must enter Run.
// Calls after shutdown return ErrStopped even if the host was ready earlier.
func (a *Application) WhenReady(ctx context.Context) error {
	if ctx == nil {
		return errors.New("omnigo: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-a.done:
		return ErrStopped
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.done:
		return ErrStopped
	case <-a.ready:
		select {
		case <-a.done:
			return ErrStopped
		default:
			return ctx.Err()
		}
	}
}

// Service starts before the native host and stops in reverse order after it.
// Start must return promptly and clean up partial work on error. Stop receives
// the independent, bounded shutdown context from lifecycle.Manager.
type Service struct {
	ID    string
	Start func(context.Context) error
	Stop  func(context.Context) error
}

// Background adapts cooperative background work to the application lifetime.
// Its result is reported when shutdown joins it. Work must return on cancellation;
// a shutdown timeout reports an error but cannot forcibly terminate a goroutine.
// Each returned service is single-use and belongs to one application.
func Background(id string, work func(context.Context) error) Service {
	start, stop := lifecycle.Background(work)
	return Service{ID: id, Start: start, Stop: stop}
}

type Launch = instance.Launch

// SingleInstanceOptions enables per-user instance coordination. ID should be a
// stable bundle/application identifier, distinct for development installations.
// OnLaunch runs on the UI goroutine for later launches, not the initial launch.
// It must not wait on worker-facing window methods. Returning an error stops Run.
type SingleInstanceOptions struct {
	ID string
	// StartupTimeout bounds ownership/endpoint handoff. Zero selects five seconds.
	StartupTimeout time.Duration
	Directory      string // Optional private directory; defaults to the user cache.
	OnLaunch       func(Launch) error
}

func (o SingleInstanceOptions) normalized() (SingleInstanceOptions, error) {
	normalized, err := apphost.InstanceOptions(o).Normalize()
	return SingleInstanceOptions(normalized), err
}
func (a *Application) acquireInstance(ctx context.Context) (*instance.Lease, bool, error) {
	if a.options.SingleInstance == nil {
		return nil, false, nil
	}
	return apphost.AcquireInstance(ctx, apphost.InstanceOptions(*a.options.SingleInstance))
}
