package omnigo

import (
	"context"
	"time"

	"github.com/herlanai/omnigo/internal/apphost"
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
	normalized, err := apphost.InstanceOptions(o).Normalize()
	return SingleInstanceOptions(normalized), err
}
func (a *Application) acquireInstance(ctx context.Context) (*instance.Lease, bool, error) {
	if a.options.SingleInstance == nil {
		return nil, false, nil
	}
	return apphost.AcquireInstance(ctx, apphost.InstanceOptions(*a.options.SingleInstance))
}
