//go:build !darwin && !linux

package appbuild

import (
	"context"
	"fmt"
	"io"
)

func launchDev(context.Context, string, string, []string, io.Writer) (devChild, error) {
	return devChild{}, fmt.Errorf("appbuild: development process host unsupported")
}
