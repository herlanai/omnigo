//go:build (!darwin && (!linux || !omnigo_webkitgtk)) || (!amd64 && !arm64)

package host

import "context"

const Available = false

func runNative(ctx context.Context, options Options) error {
	return ErrUnsupported
}
