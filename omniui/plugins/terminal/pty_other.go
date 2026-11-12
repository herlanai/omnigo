//go:build !darwin && !linux

package terminal

import "context"

func start(context.Context, Options) (backend, error) { return nil, ErrUnsupported }
