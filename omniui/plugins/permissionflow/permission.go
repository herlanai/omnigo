// Package permissionflow checks explicit native permissions. Importing this
// package never prompts. Request must be invoked deliberately by the application.
package permissionflow

import (
	"context"
	"errors"
)

type Permission string

const (
	Accessibility   Permission = "accessibility"
	ScreenRecording Permission = "screen-recording"
)

type Status string

const (
	Granted     Status = "granted"
	NotGranted  Status = "not-granted"
	Unsupported Status = "unsupported"
)

var ErrUnsupported = errors.New("permissionflow: permission or platform not implemented")

func Check(p Permission) (Status, error) { return check(p, false) }
func Request(ctx context.Context, p Permission) (Status, error) {
	if ctx == nil {
		return Unsupported, errors.New("permissionflow: nil context")
	}
	if err := ctx.Err(); err != nil {
		return NotGranted, err
	}
	return check(p, true)
}
