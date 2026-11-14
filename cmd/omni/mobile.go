package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"

	"github.com/herlanai/omnigo/internal/appbuild"
	"github.com/herlanai/omnigo/internal/mobilebuild"
)

func mobileCommand(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: omni mobile build|doctor --target ios-simulator|ios|android")
	}
	if args[0] == "-h" || args[0] == "--help" {
		_, err := fmt.Fprintln(out, "Usage: omni mobile build|doctor [-config omnigo.json] [-target ios-simulator|ios|android] [-arch arm64|amd64]\nMobile native linking uses cgo; desktop builds remain CGO_ENABLED=0. Device iOS output is unsigned; Android APK is for development.")
		return err
	}
	action := args[0]
	if action != "build" && action != "doctor" {
		return fmt.Errorf("unknown mobile command %q", action)
	}
	f := flag.NewFlagSet("mobile "+action, flag.ContinueOnError)
	config := f.String("config", "omnigo.json", "application configuration")
	target := f.String("target", "ios-simulator", "ios-simulator, ios or android")
	arch := f.String("arch", "arm64", "arm64 or amd64")
	if err := parseFlags(f, args[1:], out); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("mobile command takes no positional arguments")
	}
	t := mobilebuild.Target{Name: *target, Arch: *arch}
	if err := t.Validate(); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if action == "doctor" {
		if _, err := exec.LookPath("go"); err != nil {
			return err
		}
		if t.Name == "android" {
			if _, err := exec.LookPath("gomobile"); err != nil {
				return fmt.Errorf("install golang.org/x/mobile/cmd/gomobile@%s, run gomobile init, and configure Android SDK/NDK", mobilebuild.ToolVersion)
			}
			if os.Getenv("ANDROID_NDK_HOME") == "" && os.Getenv("ANDROID_HOME") == "" {
				return errors.New("set ANDROID_HOME (SDK with NDK); ANDROID_NDK_HOME can override the NDK location")
			}
			if err := mobilebuild.CheckAndroidTools(); err != nil {
				return err
			}
		} else {
			if runtime.GOOS != "darwin" {
				return errors.New("iOS requires macOS and Xcode")
			}
			sdk := "iphoneos"
			if t.Name == "ios-simulator" {
				sdk = "iphonesimulator"
			}
			c := exec.CommandContext(ctx, "xcrun", "--sdk", sdk, "--show-sdk-path")
			c.Stdout = out
			c.Stderr = out
			if err := c.Run(); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(out, "Mobile tool locations found; compile, signing and native execution were not tested. See docs/mobile.md.")
		return err
	}
	path, err := filepath.Abs(*config)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	d := json.NewDecoder(io.LimitReader(file, 1<<20))
	d.DisallowUnknownFields()
	var c appbuild.Config
	if err = d.Decode(&c); err != nil {
		return err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return errors.New("expected one configuration object")
	}
	if c.Mobile == nil {
		return errors.New("mobile configuration missing; create with omni init --template mobile")
	}
	result, err := mobilebuild.Build(ctx, filepath.Dir(path), *c.Mobile, t, out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Built", result, "(experimental; iOS device signing and store release are application-owned)")
	return err
}
