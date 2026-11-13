//go:build !linux && !windows && !(darwin && (amd64 || arm64))

package system

import "context"

func nativeTrash(context.Context, string) error { return ErrUnavailable }
