//go:build !darwin && !linux && !windows

package instance

import (
	"errors"
	"os"
)

func tryLock(*os.File) (bool, error) { return false, errors.New("instance: platform unsupported") }
func unlock(*os.File)                {}
