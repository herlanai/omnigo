// Command nativeprobe verifies native runtime evidence produced by CI builds.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: nativeprobe ios PROJECT [DEVICE [ARCH]] | android PROJECT | desktop PROJECT WINDOW_PROBE go|webview | require LOG TEST...")
	}
	switch args[0] {
	case "require":
		if len(args) < 3 {
			return fmt.Errorf("require expects LOG TEST...")
		}
		f, e := os.Open(args[1])
		if e != nil {
			return e
		}
		defer f.Close()
		return verify(f, args[2:])
	case "ios":
		if len(args) < 2 || len(args) > 4 {
			return fmt.Errorf("ios expects PROJECT [DEVICE [ARCH]]")
		}
		return ios(args[1:])
	case "android":
		if len(args) != 2 {
			return fmt.Errorf("android expects PROJECT")
		}
		return android(args[1])
	case "desktop":
		if len(args) != 4 {
			return fmt.Errorf("desktop expects PROJECT WINDOW_PROBE go|webview")
		}
		return desktop(args[1], args[2], args[3])
	}
	return fmt.Errorf("unknown probe %q", args[0])
}
func verify(r io.Reader, required []string) error {
	passed, failed := map[string]bool{}, map[string]bool{}
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 4096), 16<<20)
	for scan.Scan() {
		var e struct{ Action, Test, Package string }
		if err := json.Unmarshal(scan.Bytes(), &e); err != nil {
			return err
		}
		if e.Action == "pass" && e.Test != "" {
			passed[e.Test] = true
		}
		if (e.Action == "fail" || e.Action == "skip") && slices.Contains(required, e.Test) {
			failed[e.Test] = true
		}
		if e.Action == "fail" && e.Test == "" {
			failed["package:"+e.Package] = true
		}
	}
	if err := scan.Err(); err != nil {
		return err
	}
	var missing, failures []string
	for _, n := range required {
		if !passed[n] {
			missing = append(missing, n)
		}
	}
	for n := range failed {
		failures = append(failures, n)
	}
	slices.Sort(missing)
	slices.Sort(failures)
	if len(missing)+len(failures) > 0 {
		return fmt.Errorf("native verification failed: missing=%v, failed=%v", missing, failures)
	}
	return nil
}
func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	b, e := c.CombinedOutput()
	if e != nil {
		return b, fmt.Errorf("%s %v: %w\n%s", name, args, e, b)
	}
	return b, nil
}
func invoke(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	b, e := command(ctx, name, args...)
	fmt.Print(string(b))
	return e
}

type config struct {
	Mobile struct{ Output, Name, ID string }
	Bundle struct{ Destination, Name string }
}

func readConfig(project string) (config, error) {
	var c config
	b, e := os.ReadFile(filepath.Join(project, "omnigo.json"))
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
func wait(ctx context.Context, interval time.Duration, check func() (bool, error)) error {
	for {
		ok, e := check()
		if e != nil {
			return e
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("native readiness deadline: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}
func screenshot(name, path string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	c := exec.CommandContext(ctx, name, args...)
	c.Stdout = f
	c.Stderr = os.Stderr
	e = c.Run()
	closeErr := f.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func ios(args []string) error {
	project, e := filepath.Abs(args[0])
	if e != nil {
		return e
	}
	cfg, e := readConfig(project)
	if e != nil {
		return e
	}
	device := ""
	if len(args) > 1 {
		device = args[1]
	}
	arch := runtime.GOARCH
	if len(args) > 2 {
		arch = args[2]
	}
	if arch != "arm64" && arch != "amd64" {
		return fmt.Errorf("unsupported simulator architecture %s", arch)
	}
	if device == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		b, e := command(ctx, "xcrun", "simctl", "list", "devices", "available", "-j")
		if e != nil {
			return e
		}
		var list struct {
			Devices map[string][]struct {
				UDID, Name  string
				IsAvailable bool
			}
		}
		if e = json.Unmarshal(b, &list); e != nil {
			return e
		}
		var keys []string
		for k := range list.Devices {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			for _, d := range list.Devices[k] {
				if d.IsAvailable && strings.HasPrefix(d.Name, "iPhone") {
					device = d.UDID
					break
				}
			}
			if device != "" {
				break
			}
		}
		if device == "" {
			return fmt.Errorf("no available iPhone simulator")
		}
	}
	if e = invoke("xcrun", "simctl", "bootstatus", device, "-b"); e != nil {
		return e
	}
	app := filepath.Join(project, cfg.Mobile.Output, "ios-simulator-"+arch, cfg.Mobile.ID+".app")
	if e = invoke("xcrun", "simctl", "install", device, app); e != nil {
		return e
	}
	logPath := filepath.Join(project, "ios-native.log")
	log, e := os.Create(logPath)
	if e != nil {
		return e
	}
	defer log.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "xcrun", "simctl", "launch", "--terminate-running-process", "--console", device, cfg.Mobile.ID)
	c.Stdout = log
	c.Stderr = log
	if e = c.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	defer func() { _ = invoke("xcrun", "simctl", "terminate", device, cfg.Mobile.ID); cancel(); <-done }()
	e = wait(ctx, 100*time.Millisecond, func() (bool, error) {
		select {
		case err := <-done:
			done <- err
			return false, fmt.Errorf("simulator launch exited before first frame: %v", err)
		default:
		}
		b, err := os.ReadFile(logPath)
		return strings.Contains(string(b), "OMNIGO_MOBILE_READY"), err
	})
	if e != nil {
		return e
	}
	if e = invoke("xcrun", "simctl", "io", device, "screenshot", filepath.Join(project, "ios-native.png")); e != nil {
		return e
	}
	fmt.Println("PASS: iOS simulator installed and presented Go controls")
	return nil
}
func android(project string) error {
	project, e := filepath.Abs(project)
	if e != nil {
		return e
	}
	cfg, e := readConfig(project)
	if e != nil {
		return e
	}
	apk := filepath.Join(project, cfg.Mobile.Output, "android-amd64", cfg.Mobile.Name+".apk")
	if e = invoke("adb", "install", "-r", apk); e != nil {
		return e
	}
	if e = invoke("adb", "logcat", "-c"); e != nil {
		return e
	}
	defer func() { _ = invoke("adb", "shell", "am", "force-stop", cfg.Mobile.ID) }()
	if e = invoke("adb", "shell", "am", "start", "-W", "-n", cfg.Mobile.ID+"/org.golang.app.GoNativeActivity"); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	e = wait(ctx, 200*time.Millisecond, func() (bool, error) {
		b, err := command(ctx, "adb", "logcat", "-d")
		if err != nil {
			return false, err
		}
		if err = os.WriteFile(filepath.Join(project, "android-native.log"), b, 0600); err != nil {
			return false, err
		}
		return strings.Contains(string(b), "OMNIGO_MOBILE_READY"), nil
	})
	if e != nil {
		return e
	}
	if e = screenshot("adb", filepath.Join(project, "android-native.png"), "exec-out", "screencap", "-p"); e != nil {
		return e
	}
	fmt.Println("PASS: Android emulator installed and presented Go controls")
	return nil
}
func desktop(project, probe, kind string) error {
	if kind != "go" && kind != "webview" {
		return fmt.Errorf("expected go or webview")
	}
	project, e := filepath.Abs(project)
	if e != nil {
		return e
	}
	probe, e = filepath.Abs(probe)
	if e != nil {
		return e
	}
	cfg, e := readConfig(project)
	if e != nil {
		return e
	}
	temp, e := os.MkdirTemp("", "omnigo-installed-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	source := filepath.Join(project, cfg.Bundle.Destination, cfg.Bundle.Name+".app")
	app := filepath.Join(temp, filepath.Base(source))
	if e = invoke("ditto", source, app); e != nil {
		return e
	}
	if e = invoke("codesign", "--verify", "--strict", app); e != nil {
		return e
	}
	log, e := os.Create(filepath.Join(project, "native-startup.log"))
	if e != nil {
		return e
	}
	defer log.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, filepath.Join(app, "Contents", "MacOS", "app"))
	child.Dir = "/"
	child.Stdout = log
	child.Stderr = log
	if e = child.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	defer func() {
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			cancel()
			<-done
		}
	}()
	ready, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	e = wait(ready, 100*time.Millisecond, func() (bool, error) {
		select {
		case err := <-done:
			done <- err
			return false, fmt.Errorf("starter exited before visible window: %v", err)
		default:
		}
		attempt, end := context.WithTimeout(ready, 5*time.Second)
		defer end()
		b, err := command(attempt, probe, strconv.Itoa(child.Process.Pid))
		if err == nil {
			fmt.Print(string(b))
			return true, nil
		}
		return false, nil
	})
	if e != nil {
		return e
	}
	if kind == "webview" {
		b, err := command(ctx, "lsof", "-Pan", "-p", strconv.Itoa(child.Process.Pid), "-iTCP", "-sTCP:LISTEN", "-Fn")
		if err != nil {
			return err
		}
		match := regexp.MustCompile(`n127\.0\.0\.1:(\d+)`).FindSubmatch(b)
		if len(match) != 2 {
			return fmt.Errorf("no loopback listener")
		}
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + string(match[1]) + "/")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Scratchpad") || !strings.Contains(html.UnescapeString(string(body)), cfg.Bundle.Name) {
			return fmt.Errorf("missing embedded starter page")
		}
		fmt.Println("PASS: embedded WebView assets served outside checkout")
	}
	return nil
}
