//go:build !darwin && !linux && !windows

package download

import (
	"errors"
	"os"
)

func tryLock(*os.File) (bool, error) {
	return false, errors.New("download: process locks unavailable on this platform")
}
func unlock(*os.File) {}
