// Package system contains platform shell integration without a browser or CGO.
package system

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrUnavailable = errors.New("system integration unavailable")

// OpenURL asks the user's configured OS handler to open an explicit URL.
// Only web and mail handlers are allowed; file and executable schemes are rejected.
func OpenURL(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || strings.ContainsAny(raw, "\x00\r\n") {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto" {
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if (u.Scheme == "http" || u.Scheme == "https") && u.Host == "" {
		return fmt.Errorf("URL host required")
	}
	return launch(ctx, raw)
}
func OpenPath(ctx context.Context, path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if strings.ContainsRune(absolute, 0) {
		return fmt.Errorf("invalid path")
	}
	if _, err = os.Stat(absolute); err != nil {
		return err
	}
	return launch(ctx, absolute)
}
func launch(ctx context.Context, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nativeOpenWindows(target)
	}
	name, args, err := opener(runtime.GOOS, target, exec.LookPath)
	if err != nil {
		return err
	}
	// No shell interpolation: characters in a URL/path remain a single argument.
	cmd := exec.CommandContext(ctx, name, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("open handler: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func opener(goos, target string, find func(string) (string, error)) (string, []string, error) {
	switch goos {
	case "darwin":
		return "/usr/bin/open", []string{target}, nil
	case "linux":
		if path, err := find("xdg-open"); err == nil {
			return path, []string{target}, nil
		}
		if path, err := find("gio"); err == nil {
			return path, []string{"open", target}, nil
		}
		return "", nil, fmt.Errorf("%w: install xdg-utils or gio", ErrUnavailable)
	default:
		return "", nil, ErrUnavailable
	}
}
