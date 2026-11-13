package watch

import (
	"os"
	"path/filepath"
	"strings"
)

// openRequestedTarget returns an owned directory handle and, for file watches,
// a basename. It never follows page-selected links. Application root links have
// already been canonicalized by openTarget during configuration/resolution.
func openRequestedTarget(root namedRoot, local string) (*os.Root, string, error) {
	current, err := root.root.OpenRoot(".")
	if err != nil {
		return nil, "", err
	}
	success := false
	defer func() {
		if !success {
			current.Close()
		}
	}()
	if root.file != "" || local == "." {
		success = true
		return current, root.file, nil
	}
	components := strings.Split(local, string(filepath.Separator))
	for index, name := range components {
		info, err := current.Lstat(name)
		if err != nil {
			return nil, "", err
		}
		if index == len(components)-1 && info.Mode().IsRegular() {
			success = true
			return current, name, nil
		}
		if !info.IsDir() {
			return nil, "", ErrDenied
		}
		child, err := current.OpenRoot(name)
		if err != nil {
			return nil, "", err
		}
		// Check both the opened object and its directory entry: an intervening link
		// to the same inode must not pass merely because identity still matches.
		entry, entryErr := current.Lstat(name)
		handle, openErr := child.Open(".")
		matches := false
		if openErr == nil {
			actual, statErr := handle.Stat()
			handle.Close()
			matches = statErr == nil && os.SameFile(info, actual)
		}
		if entryErr != nil || !entry.IsDir() || !os.SameFile(info, entry) || !matches {
			child.Close()
			return nil, "", ErrDenied
		}
		current.Close()
		current = child
	}
	success = true
	return current, "", nil
}
