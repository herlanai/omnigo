//go:build !darwin || !arm64

package native

func CheckEmbeddedWeb() error { return ErrUnsupported }
