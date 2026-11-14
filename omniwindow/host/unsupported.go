//go:build (!darwin && (!linux || !omnigo_webkitgtk) && (!windows || !omnigo_webview2)) || (!amd64 && !arm64)

package host

import "context"

const Available = false

func runNative(ctx context.Context, options Options) error {
	return ErrUnsupported
}
