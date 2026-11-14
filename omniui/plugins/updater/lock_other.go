//go:build !darwin && !linux && !windows

package updater

import (
	"errors"
	"os"
)

func tryInstallLock(*os.File) (bool, error) {
	return false, errors.New("updater: process locks unavailable on this platform")
}
func unlockInstall(*os.File) {}
