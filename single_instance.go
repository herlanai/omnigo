package omnigo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/herlanai/omnigo/omniwindow/instance"
)

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
	if o.StartupTimeout < 0 {
		return o, errors.New("omnigo: negative single-instance startup timeout")
	}
	if o.StartupTimeout == 0 {
		o.StartupTimeout = 5 * time.Second
	}
	if o.ID == "" {
		return o, errors.New("omnigo: single-instance ID required")
	}
	if o.Directory == "" {
		root, err := os.UserCacheDir()
		if err != nil {
			return o, err
		}
		o.Directory = filepath.Join(root, "omnigo", "instances")
	}
	absolute, err := filepath.Abs(o.Directory)
	o.Directory = absolute
	return o, err
}
func (a *Application) acquireInstance(ctx context.Context) (*instance.Lease, bool, error) {
	o := a.options.SingleInstance
	if o == nil {
		return nil, false, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, false, err
	}
	token := os.Getenv("XDG_ACTIVATION_TOKEN")
	if token == "" {
		token = os.Getenv("DESKTOP_STARTUP_ID")
	}
	launch := Launch{Args: append([]string(nil), os.Args[1:]...), WorkingDir: wd, ActivationToken: token}
	bounded, cancel := context.WithTimeout(ctx, o.StartupTimeout)
	defer cancel()
	return acquireOrForward(bounded,
		func() (*instance.Lease, error) { return instance.Open(context.WithoutCancel(ctx), o.Directory, o.ID) },
		func() error { return instance.Notify(bounded, o.Directory, o.ID, launch) },
		func(ctx context.Context) error {
			timer := time.NewTimer(10 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		})

}

type launchDispatcher interface {
	Post(context.Context, func() error) (<-chan error, error)
}

func deliverLaunches(ctx context.Context, l *instance.Lease, ready <-chan launchDispatcher, callback func(Launch) error) error {
	var controller launchDispatcher
	select {
	case <-ctx.Done():
		return nil
	case controller = <-ready:
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-l.Messages():
			if !ok {
				return nil
			}
			done, err := controller.Post(ctx, func() error {
				if callback != nil {
					return callback(message)
				}
				return nil
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			case err := <-done:
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
			}
		}
	}
}

// Retry only failures known to precede sending. An ambiguous acknowledgement
// failure must surface rather than risk executing launch side effects twice.
func acquireOrForward(ctx context.Context, open func() (*instance.Lease, error), notify func() error, wait func(context.Context) error) (*instance.Lease, bool, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		lease, err := open()
		if err == nil {
			return lease, false, nil
		}
		if !errors.Is(err, instance.ErrRunning) {
			return nil, false, err
		}
		err = notify()
		if err == nil {
			return nil, true, nil
		}
		if !errors.Is(err, instance.ErrNotReady) {
			return nil, false, err
		}
		if err := wait(ctx); err != nil {
			return nil, false, err
		}
	}
}
