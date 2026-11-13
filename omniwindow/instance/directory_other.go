//go:build !darwin && !linux && !windows

package instance

import "errors"

func prepareDirectory(string) error { return errors.New("instance: platform unsupported") }
