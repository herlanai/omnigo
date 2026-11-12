//go:build !darwin || (!amd64 && !arm64)

package host

import "fmt"

func CheckTitlebar() error { return fmt.Errorf("native titlebar probe requires macOS amd64 or arm64") }
