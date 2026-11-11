//go:build !darwin || !arm64

package native

import "context"

const Available = false

func Run(ctx context.Context, url string, quit func(), ready func(*Host)) error {
	return ErrUnsupported
}
