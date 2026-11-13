package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/herlanai/omnigo/omniwindow"
)

type environmentCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}
type environmentReport struct {
	Ready     bool                         `json:"ready"`
	Checks    []environmentCheck           `json:"checks"`
	Platforms []omniwindow.PlatformSupport `json:"platforms"`
}
type environmentProbe struct {
	run    func(context.Context, string, ...string) (string, error)
	lookup func(string) (string, error)
	getenv func(string) string
}

// Bounded output prevents a broken tool from consuming unlimited memory. Each
// process also has a deadline. No package installation is performed.
type probeOutput struct{ data []byte }

func (b *probeOutput) Write(p []byte) (int, error) {
	const max = 16 << 10
	if len(p) > max-len(b.data) {
		return 0, errors.New("tool output exceeds 16 KiB")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func localEnvironmentProbe() environmentProbe {
	return environmentProbe{lookup: exec.LookPath, getenv: os.Getenv, run: func(ctx context.Context, name string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		command := name
		if runtime.GOOS == "windows" && name == "npm.cmd" {
			// npm is a batch launcher on Windows. These are fixed diagnostic
			// arguments, never a user-provided shell command.
			command = "cmd.exe"
			args = append([]string{"/d", "/c", "npm.cmd"}, args...)
		}
		cmd := exec.CommandContext(ctx, command, args...)
		if name == "go" {
			// A diagnostic must not trigger automatic toolchain downloads.
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
		}
		cmd.WaitDelay = time.Second
		var output probeOutput
		cmd.Stdout = &output
		cmd.Stderr = &output
		err := cmd.Run()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return strings.TrimSpace(string(output.data)), err
	}}
}

var toolVersion = regexp.MustCompile(`^(?:go version go|v)?([0-9]+)\.([0-9]+)(?:\.[0-9]+)?(?:[\s+-].*)?$`)

func versionAtLeast(value string, major, minor int) bool {
	match := toolVersion.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return false
	}
	a, e1 := strconv.Atoi(match[1])
	b, e2 := strconv.Atoi(match[2])
	return e1 == nil && e2 == nil && (a > major || a == major && b >= minor)
}

func inspectEnvironment(ctx context.Context, goos, goarch string, probe environmentProbe) environmentReport {
	report := environmentReport{Ready: true, Platforms: []omniwindow.PlatformSupport{omniwindow.Platform(goos, goarch)}}
	add := func(name, status, detail string) {
		report.Checks = append(report.Checks, environmentCheck{name, status, detail})
		if status == "fail" {
			report.Ready = false
		}
	}
	version := func(name string, args []string, major, minor int) {
		if ctx.Err() != nil {
			add(name, "fail", "Check canceled")
			return
		}
		output, err := probe.run(ctx, name, args...)
		if err != nil {
			add(name, "fail", fmt.Sprintf("Cannot run %s; install it and ensure it is on PATH (%v)", name, err))
			return
		}
		if !versionAtLeast(output, major, minor) {
			add(name, "fail", fmt.Sprintf("Need version >= %d.%d; reported %q", major, minor, output))
			return
		}
		add(name, "pass", output)
	}
	version("go", []string{"version"}, 1, 26)
	version("node", []string{"--version"}, 24, 0)
	npm := "npm"
	if goos == "windows" {
		npm = "npm.cmd"
	}
	version(npm, []string{"--version"}, 1, 0)
	optional := func(name, feature string) {
		location, err := probe.lookup(name)
		if err != nil {
			add(name, "warn", "Not on PATH; optional for "+feature)
			return
		}
		add(name, "pass", location+" (found on PATH; "+feature+" was not exercised)")
	}
	optional("bun", "alternate frontend workflows")
	optional("pnpm", "the repository Studio workflow")
	optional("makensis", "Windows installer packaging")
	optional("gh", "GitHub release upload")
	switch goos {
	case "darwin":
		output, err := probe.run(ctx, "sw_vers", "-productVersion")
		if err != nil {
			add("macOS", "warn", "Could not read the operating system version")
		} else {
			add("macOS", "pass", output+"; native window operation is not tested by doctor")
		}
		optional("codesign", "macOS application signing; signing identities are not checked")
		optional("osslsigncode", "Windows signing from this host")
	case "linux":
		if probe.getenv("DISPLAY") == "" {
			add("DISPLAY", "warn", "Unset; the optional X11 host requires an X server")
		} else {
			add("DISPLAY", "pass", "Set; display connectivity was not tested")
		}
		output, err := probe.run(ctx, "ldconfig", "-p")
		if err == nil && strings.Contains(output, "libX11.so.6 ") {
			add("X11", "pass", "libX11.so.6 listed in the loader cache; loading it was not tested")
		} else {
			add("X11", "warn", "libX11.so.6 not confirmed in loader cache; required only by the optional X11 host")
		}
		optional("osslsigncode", "Windows signing from this host")
	case "windows":
		optional("signtool", "Windows application signing")
	default:
		add("platform", "fail", "No desktop target for "+goos)
	}
	platform := omniwindow.Platform(goos, goarch)
	if !platform.CoreBuild {
		add("architecture", "fail", "Unsupported desktop target: "+goos+"/"+goarch)
	}
	add("native-host", "warn", platform.NativeWindow+"; WebView: "+platform.WebView+". "+platform.Evidence)
	return report
}

func printEnvironment(w io.Writer, report environmentReport) error {
	for _, check := range report.Checks {
		if _, err := fmt.Fprintf(w, "[%s] %s: %s\n", check.Status, check.Name, check.Detail); err != nil {
			return err
		}
	}
	return nil
}
