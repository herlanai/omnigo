package system

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Reveal shows and selects an existing item in the system file manager.
// Symlinks are revealed themselves, including dangling symlinks. On Linux,
// an org.freedesktop.FileManager1 service and gdbus are required; opening the
// parent folder is deliberately not substituted for selecting the item.
func Reveal(ctx context.Context, path string) error {
	if ctx == nil {
		return fmt.Errorf("reveal: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if path == "" || strings.ContainsRune(path, 0) {
		return fmt.Errorf("reveal: invalid path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(absolute); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nativeRevealWindows(absolute)
	}
	name, args, err := revealCommand(runtime.GOOS, absolute, exec.LookPath)
	if err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("reveal: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func revealCommand(goos, path string, find func(string) (string, error)) (string, []string, error) {
	switch goos {
	case "darwin":
		return "/usr/bin/open", []string{"-R", "--", path}, nil
	case "linux":
		command, err := find("gdbus")
		if err != nil {
			return "", nil, fmt.Errorf("%w: gdbus is required for FileManager1", ErrUnavailable)
		}
		uri := (&url.URL{Scheme: "file", Path: path}).String()
		// GVariant strings have their own quoting rules, separate from shell quoting.
		uri = strings.ReplaceAll(strings.ReplaceAll(uri, "\\", "\\\\"), "'", "\\'")
		return command, []string{"call", "--session", "--dest", "org.freedesktop.FileManager1", "--object-path", "/org/freedesktop/FileManager1", "--method", "org.freedesktop.FileManager1.ShowItems", "['" + uri + "']", ""}, nil
	default:
		return "", nil, ErrUnavailable
	}
}
