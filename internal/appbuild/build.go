// Package appbuild builds trusted local application projects without shell expansion.
package appbuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/herlanai/omnigo/internal/mobilebuild"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	Mobile    *mobilebuild.Config `json:"mobile,omitempty"`
	Toolchain string              `json:"toolchain,omitempty"`
	Dev       *DevOptions         `json:"dev,omitempty"`
	Package   string              `json:"package"`
	Output    string              `json:"output"`
	GOOS      string              `json:"goos"`
	GOARCH    string              `json:"goarch"`
	Before    [][]string          `json:"before,omitempty"`
	Bundle    *BundleConfig       `json:"bundle,omitempty"`
}

type Runner func(context.Context, string, []string, []string, io.Writer) error

func Execute(ctx context.Context, dir string, argv, env []string, output io.Writer) error {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, output, output
	return cmd.Run()
}

// Build resolves paths relative to the config, stages on the output filesystem,
// and publishes only after the compiler has produced a regular executable.
// Before commands are trusted application code, not a shell or a sandbox.
func Build(ctx context.Context, configPath string, output io.Writer, run Runner) (string, error) {
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	f, err := os.Open(absolute)
	if err != nil {
		return "", err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var c Config
	if err = decoder.Decode(&c); err != nil {
		return "", err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return "", errors.New("appbuild: expected one JSON object")
	}
	if c.Toolchain != "" && c.Toolchain != "go" && c.Toolchain != "web" && c.Toolchain != "studio" {
		return "", errors.New("appbuild: toolchain must be go, web or studio")
	}
	if c.Package == "" || strings.HasPrefix(c.Package, "-") || c.Output == "" {
		return "", errors.New("appbuild: package and output are required")
	}
	switch c.GOOS {
	case "darwin", "windows", "linux":
	default:
		return "", errors.New("appbuild: goos must be darwin, windows or linux")
	}
	if c.GOARCH != "arm64" && c.GOARCH != "amd64" {
		return "", errors.New("appbuild: goarch must be arm64 or amd64")
	}
	for _, command := range c.Before {
		if len(command) == 0 || command[0] == "" {
			return "", errors.New("appbuild: empty before command")
		}
	}
	dir := filepath.Dir(absolute)
	target := c.Output
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	if !strings.HasPrefix(filepath.Clean(c.Package), "..") && strings.HasPrefix(c.Package, ".") {
		source := filepath.Join(dir, c.Package)
		rel, e := filepath.Rel(target, source)
		if e == nil && (rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return "", errors.New("appbuild: output contains source package")
		}
	}
	if info, e := os.Lstat(target); e == nil && !info.Mode().IsRegular() {
		return "", errors.New("appbuild: output must be a regular file")
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	if run == nil {
		run = Execute
	}
	// Frontend commands execute for the host; target overrides only affect Go.
	for _, command := range c.Before {
		if err = run(ctx, dir, command, os.Environ(), output); err != nil {
			return "", fmt.Errorf("appbuild: preparation: %w", err)
		}
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".omni-build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	binary := filepath.Join(stage, "app")
	if c.GOOS == "windows" {
		binary += ".exe"
	}
	env := []string{}
	for _, v := range os.Environ() {
		name, _, _ := strings.Cut(v, "=")
		if !strings.EqualFold(name, "GOOS") && !strings.EqualFold(name, "GOARCH") && !strings.EqualFold(name, "CGO_ENABLED") {
			env = append(env, v)
		}
	}
	env = append(env, "CGO_ENABLED=0", "GOOS="+c.GOOS, "GOARCH="+c.GOARCH)
	if err = run(ctx, dir, []string{"go", "build", "-trimpath", "-o", binary, c.Package}, env, output); err != nil {
		return "", fmt.Errorf("appbuild: compile: %w", err)
	}
	info, err := os.Lstat(binary)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", errors.New("appbuild: compiler produced no executable")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = os.Rename(binary, target); err != nil {
		return "", fmt.Errorf("appbuild: publish (previous output retained): %w", err)
	}
	return target, nil
}
