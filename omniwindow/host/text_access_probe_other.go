//go:build !darwin || (!amd64 && !arm64)

package host

func CheckTextAccessibility() error { return ErrUnsupported }
