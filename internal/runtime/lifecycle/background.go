package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

// Background returns a single-use worker's start/stop callbacks. The owning
// lifecycle cancels the work context before joining with a bounded stop context.
func Background(work func(context.Context) error) (start, stop func(context.Context) error) {
	done := make(chan error, 1)
	var started atomic.Bool
	start = func(ctx context.Context) error {
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
	}
	stop = func(ctx context.Context) error {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return start, stop
}
