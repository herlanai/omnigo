//go:build !darwin || (!amd64 && !arm64)

package gpu

func openRenderer() (renderer, error) { return nil, ErrUnavailable }
