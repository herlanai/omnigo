package mobilebuild

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSimulators(t *testing.T) {
	list := `== Devices ==
-- iOS 18.2 --
    iPhone 17 Pro (0A2B3C4D-5E6F-4A1B-8C2D-9E0F1A2B3C4D) (Shutdown)
    iPad Pro (ABCDEF01-2345-4C6D-8E9F-0A1B2C3D4E5F) (Booted)
    Watch Series 10 (11112222-3333-4444-5555-6666777788889999) (Shutdown)
-- Unavailable: com.apple.CoreSimulator.SimRuntime.iOS 17.4 --
    Old iPhone (99998888-7777-4666-8555-4444333322221111) (Shutdown)
`
	booted, available := parseSimulators(list)
	if len(booted) != 1 || booted[0] != "ABCDEF01-2345-4C6D-8E9F-0A1B2C3D4E5F" {
		t.Fatalf("booted = %v", booted)
	}
	if len(available) != 1 || available[0] != "0A2B3C4D-5E6F-4A1B-8C2D-9E0F1A2B3C4D" {
		t.Fatalf("available = %v (watch and unavailable devices must be excluded)", available)
	}
}

func TestParseADBDevices(t *testing.T) {
	list := "List of devices attached\nemulator-5554\tdevice\ndeadbeef\tunauthorized\n\n"
	serials := parseADBDevices(list)
	if len(serials) != 1 || serials[0] != "emulator-5554" {
		t.Fatalf("serials = %v", serials)
	}
	if got := parseADBDevices("List of devices attached\n"); len(got) != 0 {
		t.Fatalf("empty list = %v", got)
	}
}

func TestActivityFromManifest(t *testing.T) {
	data := []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app"><uses-permission android:name="android.permission.INTERNET"/><application><activity android:name="org.golang.app.GoNativeActivity" android:configChanges="a|b"/></application></manifest>`)
	activity, err := activityFromManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if activity != "org.golang.app.GoNativeActivity" {
		t.Fatalf("activity = %q (uses-permission android:name must not match)", activity)
	}
	if _, err := activityFromManifest([]byte(`<manifest/>`)); err == nil {
		t.Fatal("missing activity accepted")
	}
}

type recordingDevice struct {
	installed, launched []string
	failInstall         bool
}

func (d *recordingDevice) Install(_ context.Context, artifact string, _ io.Writer) error {
	if d.failInstall {
		return errors.New("install refused")
	}
	d.installed = append(d.installed, filepath.Base(artifact))
	return nil
}
func (d *recordingDevice) Launch(_ context.Context, _ io.Writer) error {
	d.launched = append(d.launched, d.installed[len(d.installed)-1])
	return nil
}
func (d *recordingDevice) Describe() string { return "fake device" }

func TestDevRejectsDeviceAndInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	o := DevOptions{Dir: dir, Config: Config{Package: "./app", Output: "out", Name: "App", ID: "com.example.app", Version: "1.0"}, Target: Target{"ios", "arm64"}}
	if err := Dev(t.Context(), o); err == nil {
		t.Fatal("ios device dev accepted")
	}
	o.Target = Target{"ios-simulator", "arm64"}
	o.Config.ID = "not-dotted"
	if err := Dev(t.Context(), o); err == nil {
		t.Fatal("invalid config accepted")
	}
}

func TestDevLoopRebuildsOnSourceChange(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "app"), 0755)
	main := filepath.Join(dir, "app", "main.go")
	os.WriteFile(main, []byte("package main\nfunc main() {}\n"), 0644)
	// Fake a cheap build: the loop must not invoke real toolchains in tests.
	previous := buildForDev
	defer func() { buildForDev = previous }()
	var builds int
	buildForDev = func(ctx context.Context, dir, destination string, c Config, tgt Target, out io.Writer) error {
		builds++
		if tgt.Name == "ios-simulator" {
			return os.MkdirAll(filepath.Join(destination, c.ID+".app"), 0755)
		}
		return os.WriteFile(filepath.Join(destination, c.Name+".apk"), []byte("apk"), 0644)
	}
	device := &recordingDevice{}
	previousDevice := deviceFactory
	defer func() { deviceFactory = previousDevice }()
	deviceFactory = func(ctx context.Context, t Target, c Config) (devDevice, error) { return device, nil }

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- Dev(ctx, DevOptions{Dir: dir, Config: Config{Package: "./app", Output: filepath.Join(dir, "out"), Name: "App", ID: "com.example.app", Version: "1.0"}, Target: Target{"ios-simulator", "arm64"}, Out: io.Discard, Poll: 20 * time.Millisecond, Quiet: 20 * time.Millisecond})
	}()
	deadline := time.Now().Add(5 * time.Second)
	for builds == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if builds == 0 {
		cancel()
		t.Fatal("initial build never ran")
	}
	os.WriteFile(main, []byte("package main\nfunc main() { changed }\n"), 0644)
	for builds < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if builds < 2 {
		t.Fatalf("change did not trigger rebuild, builds = %d", builds)
	}
	if len(device.installed) < 2 || len(device.launched) < 2 {
		t.Fatalf("install/launch sequence = %v / %v", device.installed, device.launched)
	}
	if got := device.launched[1]; got != device.installed[1] {
		t.Fatalf("launch followed a stale artifact: %v vs %v", got, device.installed[1])
	}
}
