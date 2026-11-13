//go:build !darwin || (!amd64 && !arm64)

package host

func CheckDesktopServices() error { return ErrUnsupported }
