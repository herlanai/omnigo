//go:build (!windows && !linux) || (!amd64 && !arm64)

package nativewindow

import (
	"errors"

	"github.com/herlanai/omnigo/apps"
)

func openBackend(apps.WindowOptions) (backend, error) {
	return nil, errors.New("nativewindow: this optional driver implements Windows and Linux amd64/arm64; use Studio's Cocoa host on macOS")
}
