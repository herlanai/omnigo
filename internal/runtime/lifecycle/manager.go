// Package lifecycle owns application lifetime, independent of a platform driver.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/herlanai/omnigo/omniui"
)

// Driver.Run owns the platform event loop and must return on cancellation.
// A native driver must be called on the OS main thread; the application does not
// move it to a background goroutine. omniwindow/nativewindow provides an optional
// experimental Win32/X11 software driver; omniwindow/host provides the Cocoa host.
type Driver interface{ Run(context.Context) error }

type Options struct {
	Registry        *omniui.Registry
	Extensions      []string
	Permissions     []string
	ShutdownTimeout time.Duration
}

type Manager struct {
	options Options
	running atomic.Bool
}

func New(o Options) *Manager {
	o.Extensions = append([]string(nil), o.Extensions...)
	o.Permissions = append([]string(nil), o.Permissions...)
	if o.ShutdownTimeout <= 0 {
		o.ShutdownTimeout = 5 * time.Second
	}
	return &Manager{options: o}
}

func guarded(fn func(context.Context) error, ctx context.Context) (err error) {
	if fn == nil {
		return nil
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("extension panic: %v", p)
		}
	}()
	return fn(ctx)
}

// Run is single-use. Cancellation precedes reverse-order cleanup. Shutdown gets
// an independent bounded context, so cancellation cannot skip cleanup.
func (a *Manager) Run(ctx context.Context, driver Driver) (err error) {
	if ctx == nil || driver == nil {
		return errors.New("lifecycle: context and driver are required")
	}
	if !a.running.CompareAndSwap(false, true) {
		return errors.New("lifecycle: application already used")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	registry := a.options.Registry
	if registry == nil {
		registry = &omniui.Registry{}
	}
	plan, err := registry.Resolve(a.options.Extensions, a.options.Permissions)
	if err != nil {
		return err
	}
	lifetime, cancel := context.WithCancel(ctx)
	started := 0
	defer func() {
		cancel()
		shutdown, done := context.WithTimeout(context.WithoutCancel(ctx), a.options.ShutdownTimeout)
		defer done()
		for i := started - 1; i >= 0; i-- {
			if e := guarded(plan[i].Stop, shutdown); e != nil {
				err = errors.Join(err, fmt.Errorf("stop %s: %w", plan[i].Descriptor.ID, e))
			}
		}
	}()
	for _, extension := range plan {
		if err := lifetime.Err(); err != nil {
			return err
		}
		if e := guarded(extension.Start, lifetime); e != nil {
			return fmt.Errorf("start %s: %w", extension.Descriptor.ID, e)
		}
		started++
	}
	return guarded(driver.Run, lifetime)
}
