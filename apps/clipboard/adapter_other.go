//go:build (!darwin && !windows) || (!amd64 && !arm64)

package clipboard

func openAdapter(Options) (adapter, error) { return nil, ErrUnsupported }
