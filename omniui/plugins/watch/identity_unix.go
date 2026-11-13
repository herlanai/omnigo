//go:build darwin || linux

package watch

import (
	"io/fs"
	"syscall"
)

// Stat data is already captured by Lstat on these platforms; no path lookup is
// deferred until pairing. Include type so reused inode numbers across types do
// not turn a replacement into a rename.
func fileIdentity(info fs.FileInfo) (identity, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || stat.Ino == 0 {
		return identity{}, false
	}
	return identity{device: uint64(stat.Dev), inode: uint64(stat.Ino), kind: info.Mode().Type()}, true
}
