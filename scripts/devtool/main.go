// Command devtool provides repository maintenance and Studio packaging in Go.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, terminationSignal())
	defer cancel()
	if err := execute(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func execute(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: go run ./scripts/devtool <dev|build|start|package|catalog|audit-ui|chromium> [options]")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "migration/controls/reference.json")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			return errors.New("run inside the OmniGo repository")
		}
		root = parent
	}
	if err = os.Chdir(root); err != nil {
		return err
	}
	switch args[0] {
	case "catalog":
		return catalog()
	case "audit-ui":
		return audit(args[1:])
	case "chromium":
		a := append([]string{"run", "./omniui/plugins/chromium/cmd/setup"}, args[1:]...)
		if !hasFlag(a, "cef-dir") && os.Getenv("CEF_ROOT") != "" {
			a = append(a, "--cef-dir", os.Getenv("CEF_ROOT"))
		}
		if !hasFlag(a, "output") {
			a = append(a, "--output", "build/chromium-runtime")
		}
		return run(ctx, "go", a...)
	case "dev", "build", "start", "package":
		return studio(ctx, root, args[0])
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
func hasFlag(args []string, name string) bool {
	for _, a := range args {
		a = strings.TrimLeft(a, "-")
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}
func run(ctx context.Context, name string, args ...string) error {
	if runtime.GOOS == "windows" && name == "pnpm" {
		name = "pnpm.cmd"
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	configureProcess(cmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
func readJSON(path string, value any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, value)
}
func writeJSON(path string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}

// copyTree preserves executable permissions and symlinks in native bundles.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, e := os.Readlink(src)
		if e != nil {
			return e
		}
		return os.Symlink(target, dst)
	}
	if info.IsDir() {
		if err = os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, e := os.ReadDir(src)
		if e != nil {
			return e
		}
		for _, entry := range entries {
			if e = copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); e != nil {
				return e
			}
		}
		return nil
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, info.Mode().Perm())
}
