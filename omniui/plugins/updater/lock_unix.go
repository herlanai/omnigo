//go:build darwin || linux

package updater

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
)

func tryInstallLock(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return false, nil
	}
	return err == nil, err
}
func unlockInstall(f *os.File) { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }
