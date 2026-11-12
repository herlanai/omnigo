//go:build !windows

package system

func nativeOpenWindows(string) error { return ErrUnavailable }
