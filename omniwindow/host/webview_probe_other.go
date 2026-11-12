//go:build !darwin || (!amd64 && !arm64)

package host

func CheckEmbeddedWeb() error { return ErrUnsupported }
