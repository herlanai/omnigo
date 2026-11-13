//go:build !windows

package system

import (
	"os"
	"path/filepath"
	"runtime"
)

func userFolder(name PathName) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "linux" {
		config, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		file, err := os.Open(filepath.Join(config, "user-dirs.dirs"))
		if err == nil {
			defer file.Close()
			if path := xdgFolder(file, name, home); path != "" {
				return path, nil
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	folder := folders[name]
	if runtime.GOOS == "darwin" && name == PathVideos {
		folder = "Movies"
	}
	return filepath.Join(home, folder), nil
}
