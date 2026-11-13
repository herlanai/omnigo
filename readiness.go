package omnigo

import (
	"context"
	"errors"
)

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
