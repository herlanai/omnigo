package omnigo

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

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
	done := make(chan error, 1)
	var started atomic.Bool
	return Service{ID: id, Start: func(ctx context.Context) error {
		if !started.CompareAndSwap(false, true) {
			return errors.New("omnigo: background service already used")
		}
		if work == nil {
			return errors.New("omnigo: nil background work")
		}
		go func() {
			var err error
			defer func() {
				if p := recover(); p != nil {
					err = fmt.Errorf("background panic: %v", p)
				}
				done <- err
			}()
			err = work(ctx)
			if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
				err = nil
			}
		}()
		return nil
	}, Stop: func(ctx context.Context) error {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
}
