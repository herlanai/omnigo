//go:build !darwin

package permissionflow

func check(Permission, bool) (Status, error) { return Unsupported, ErrUnsupported }
