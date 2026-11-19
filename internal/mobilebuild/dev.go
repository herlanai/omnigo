package mobilebuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Dev is a full rebuild loop on a booted iOS simulator or a running Android
// emulator or device: it rebuilds, reinstalls and relaunches whenever project
// sources change. It is not a hot code swap; the application restarts and
// loses in-memory state on every change. iOS device targets are rejected
// because installation needs app-owned signing.
func Dev(ctx context.Context, o DevOptions) error {
	if o.Poll <= 0 {
		o.Poll = 500 * time.Millisecond
	}
	if o.Quiet <= 0 {
		o.Quiet = 300 * time.Millisecond
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	if err := o.Config.Validate(); err != nil {
		return err
	}
	if err := o.Target.Validate(); err != nil {
		return err
	}
	if o.Target.Name == "ios" {
		return errors.New("mobilebuild: dev requires ios-simulator or android; iOS device install needs app-owned signing")
	}
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return err
	}
	device, err := deviceFactory(ctx, o.Target, o.Config)
	if err != nil {
		return err
	}
	skip := map[string]bool{}
	if resolved, err := filepath.Abs(o.Config.Output); err == nil {
		skip[resolved] = true
	}
	started := time.Now()
	if err := devIterate(ctx, o, device, dir); err != nil {
		fmt.Fprintf(o.Out, "mobilebuild: initial build failed: %v (fix the error or save a file)\n", err)
	} else {
		fmt.Fprintf(o.Out, "Installed on %s in %s; watching %s for changes (Ctrl+C stops)\n", device.Describe(), round(time.Since(started)), dir)
	}
	last, err := scanTree(dir, skip)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(o.Poll):
		}
		next, err := scanTree(dir, skip)
		if err != nil {
			fmt.Fprintln(o.Out, "mobilebuild: watch:", err)
			continue
		}
		if !changed(last, next) {
			continue
		}
		last = next
		// Let editors finish their write bursts before compiling.
		time.Sleep(o.Quiet)
		if stable, err := scanTree(dir, skip); err == nil {
			last = stable
		}
		started = time.Now()
		fmt.Fprintf(o.Out, "Change detected; rebuilding...\n")
		if err := devIterate(ctx, o, device, dir); err != nil {
			fmt.Fprintf(o.Out, "mobilebuild: rebuild failed: %v (fix the error or save again)\n", err)
			continue
		}
		fmt.Fprintf(o.Out, "Relaunched on %s in %s\n", device.Describe(), round(time.Since(started)))
	}
}

// Hook points so tests can fake the toolchain and the target device.
var (
	buildForDev   = buildInto
	deviceFactory = deviceFor
)

// DevOptions configures the mobile dev loop.
type DevOptions struct {
	Dir    string
	Config Config
	Target Target
	Out    io.Writer
	Poll   time.Duration
	Quiet  time.Duration
}

func round(d time.Duration) string { return d.Round(100 * time.Millisecond).String() }

func devIterate(ctx context.Context, o DevOptions, device devDevice, dir string) error {
	stage, err := os.MkdirTemp("", "omni-mobile-dev-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	destination := filepath.Join(stage, o.Target.Name+"-"+o.Target.Arch)
	if err := buildForDev(ctx, dir, destination, o.Config, o.Target, o.Out); err != nil {
		return err
	}
	artifact, err := findArtifact(destination, o.Target.Name, o.Config.Name)
	if err != nil {
		return err
	}
	if err := device.Install(ctx, artifact, o.Out); err != nil {
		return err
	}
	return device.Launch(ctx, o.Out)
}

// buildInto is Build without the fresh-destination guard; the dev loop owns a
// private temporary destination per iteration.
func buildInto(ctx context.Context, dir, destination string, c Config, t Target, out io.Writer) error {
	stage := destination + ".stage"
	// The platform builders create their bundle inside the stage directory.
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	if t.Name == "android" {
		if err := buildAndroid(ctx, dir, stage, c, t, out); err != nil {
			return err
		}
	} else {
		if err := buildIOS(ctx, dir, stage, c, t, out); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Rename(stage, destination)
}

func findArtifact(destination, target, name string) (string, error) {
	suffix := ".app"
	if target == "android" {
		suffix = ".apk"
	}
	found := ""
	err := filepath.WalkDir(destination, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// The iOS artifact is a bundle directory; the Android artifact is a file.
		if strings.HasSuffix(d.Name(), suffix) && d.IsDir() == (target != "android") {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("mobilebuild: built %s not found in %s", suffix, destination)
	}
	return found, nil
}

type devDevice interface {
	Install(ctx context.Context, artifact string, out io.Writer) error
	Launch(ctx context.Context, out io.Writer) error
	Describe() string
}

func deviceFor(ctx context.Context, t Target, c Config) (devDevice, error) {
	switch t.Name {
	case "ios-simulator":
		if runtime.GOOS != "darwin" {
			return nil, errors.New("mobilebuild: iOS Simulator requires macOS and Xcode")
		}
		udid, booted, err := pickSimulator(ctx)
		if err != nil {
			return nil, err
		}
		if !booted {
			if err := run(ctx, "", nil, io.Discard, "xcrun", "simctl", "boot", udid); err != nil && !strings.Contains(err.Error(), "already booted") {
				return nil, fmt.Errorf("mobilebuild: boot simulator %s: %w", udid, err)
			}
			if err := run(ctx, "", nil, io.Discard, "xcrun", "simctl", "bootstatus", udid); err != nil {
				return nil, fmt.Errorf("mobilebuild: wait for simulator %s: %w", udid, err)
			}
		}
		return simulatorDevice{udid: udid, bundleID: c.ID}, nil
	case "android":
		serial, err := pickAndroidDevice(ctx)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(".", c.Package, "AndroidManifest.xml"))
		if err != nil {
			return nil, fmt.Errorf("mobilebuild: AndroidManifest.xml required beside main.go: %w", err)
		}
		activity, err := activityFromManifest(data)
		if err != nil {
			return nil, err
		}
		return androidDevice{serial: serial, component: c.ID + "/" + activity}, nil
	}
	return nil, fmt.Errorf("mobilebuild: dev does not support target %q", t.Name)
}

type simulatorDevice struct{ udid, bundleID string }

func (d simulatorDevice) Describe() string { return "iOS Simulator " + d.udid }
func (d simulatorDevice) Install(ctx context.Context, artifact string, out io.Writer) error {
	return run(ctx, "", nil, out, "xcrun", "simctl", "install", d.udid, artifact)
}
func (d simulatorDevice) Launch(ctx context.Context, out io.Writer) error {
	return run(ctx, "", nil, out, "xcrun", "simctl", "launch", d.udid, d.bundleID)
}

type androidDevice struct{ serial, component string }

func (d androidDevice) Describe() string { return "Android device " + d.serial }
func (d androidDevice) Install(ctx context.Context, artifact string, out io.Writer) error {
	return run(ctx, "", nil, out, "adb", "-s", d.serial, "install", "-r", artifact)
}
func (d androidDevice) Launch(ctx context.Context, out io.Writer) error {
	return run(ctx, "", nil, out, "adb", "-s", d.serial, "shell", "am", "start", "-n", d.component)
}

func run(ctx context.Context, dir string, env []string, out io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

var simulatorUDID = regexp.MustCompile(`\(([0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12})\)`)

// parseSimulators keeps the order of `xcrun simctl list devices` sections and
// skips devices under unavailable sections.
func parseSimulators(list string) (booted, available []string) {
	section := ""
	for _, line := range strings.Split(list, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			section = strings.ToLower(trimmed)
			continue
		}
		match := simulatorUDID.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		if strings.Contains(section, "unavailable") || strings.Contains(strings.ToLower(trimmed), "unavailable") {
			continue
		}
		if strings.Contains(trimmed, "(Booted)") {
			booted = append(booted, match[1])
			continue
		}
		if strings.Contains(trimmed, "(Shutdown)") {
			available = append(available, match[1])
		}
	}
	return booted, available
}

func pickSimulator(ctx context.Context) (udid string, booted bool, err error) {
	list, e := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices").Output()
	if e != nil {
		return "", false, fmt.Errorf("mobilebuild: list simulators: %w", e)
	}
	bootedDevices, shutdown := parseSimulators(string(list))
	if len(bootedDevices) > 0 {
		return bootedDevices[0], true, nil
	}
	if len(shutdown) == 0 {
		return "", false, errors.New("mobilebuild: no iOS Simulator available; create one in Xcode")
	}
	return shutdown[0], false, nil
}

// parseADBDevices returns serials in the "device" state.
func parseADBDevices(list string) []string {
	serials := []string{}
	for _, line := range strings.Split(list, "\n")[1:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[1] == "device" {
			serials = append(serials, fields[0])
		}
	}
	return serials
}

func pickAndroidDevice(ctx context.Context) (string, error) {
	list, err := exec.CommandContext(ctx, "adb", "devices").Output()
	if err != nil {
		return "", fmt.Errorf("mobilebuild: adb devices: %w", err)
	}
	serials := parseADBDevices(string(list))
	if len(serials) == 0 {
		return "", errors.New("mobilebuild: no Android emulator or device; start one and run again")
	}
	return serials[0], nil
}

// activityFromManifest reads the launch activity declared by the application.
func activityFromManifest(data []byte) (string, error) {
	for _, m := range activityName.FindAllSubmatch(data, -1) {
		if name := string(m[1]); name != "" {
			return name, nil
		}
	}
	return "", errors.New("mobilebuild: AndroidManifest.xml declares no activity android:name")
}

// Only activity elements carry the launch component; uses-permission and
// other tags also carry android:name and must not match.
var activityName = regexp.MustCompile(`(?s)<activity\b[^>]*android:name="([^"]+)"`)

type fileStamp struct {
	mod  time.Time
	size int64
}
type snapshot map[string]fileStamp

func scanTree(root string, skip map[string]bool) (snapshot, error) {
	result := snapshot{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if path != root {
			if strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if resolved, e := filepath.Abs(path); e == nil && skip[resolved] {
					return filepath.SkipDir
				}
			}
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		result[path] = fileStamp{mod: info.ModTime(), size: info.Size()}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("mobilebuild: watch %s: %w", root, err)
	}
	return result, nil
}

func changed(before, after snapshot) bool {
	if len(before) != len(after) {
		return true
	}
	for path, old := range before {
		next, ok := after[path]
		if !ok || old != next {
			return true
		}
	}
	return false
}
