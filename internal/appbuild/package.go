package appbuild

import (
	"bytes"
	"context"
	"debug/macho"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type BundleConfig struct {
	Name        string   `json:"name"`
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Icon        string   `json:"icon"`
	Licenses    []string `json:"licenses"`
	Destination string   `json:"destination"`
}

func plistString(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func copyRegular(src, dst string, mode os.FileMode) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", src)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}

// PackageMac builds a fresh release directory containing a signed app and DMG.
// Existing destinations are rejected, never deleted or partially overwritten.
// Signing is ad-hoc for local validation; no notarization is claimed.
func PackageMac(ctx context.Context, config string, output io.Writer) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("appbuild: macOS packaging requires a macOS host")
	}
	absolute, err := filepath.Abs(config)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return "", err
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c); err != nil {
		return "", err
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return "", errors.New("appbuild: expected one JSON object")
	}
	b := c.Bundle
	if b == nil || c.GOOS != "darwin" || b.Name == "" || strings.ContainsAny(b.Name, "/\\:\x00") || b.Name == "." || b.Name == ".." || b.Destination == "" || b.Icon == "" || len(b.Licenses) == 0 {
		return "", errors.New("appbuild: bundle requires name, destination, ICNS icon and license files for darwin")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`).MatchString(b.ID) || !regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`).MatchString(b.Version) {
		return "", errors.New("appbuild: invalid bundle id or numeric version")
	}
	dir := filepath.Dir(absolute)
	resolve := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(dir, p)
	}
	destination := resolve(b.Destination)
	if _, err = os.Lstat(destination); err == nil {
		return "", errors.New("appbuild: release destination exists; choose a new destination")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	icon, err := os.ReadFile(resolve(b.Icon))
	if err != nil {
		return "", err
	}
	if len(icon) < 8 || string(icon[:4]) != "icns" {
		return "", errors.New("appbuild: icon must be ICNS")
	}
	binary, err := Build(ctx, absolute, output, nil)
	if err != nil {
		return "", err
	}
	executable, err := macho.Open(binary)
	if err != nil {
		return "", err
	}
	cpu := executable.Cpu
	executable.Close()
	if (c.GOARCH == "arm64" && cpu != macho.CpuArm64) || (c.GOARCH == "amd64" && cpu != macho.CpuAmd64) {
		return "", errors.New("appbuild: executable architecture mismatch")
	}
	if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".omni-package-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	release := filepath.Join(stage, "release")
	app := filepath.Join(release, b.Name+".app")
	contents := filepath.Join(app, "Contents")
	for _, sub := range []string{"MacOS", "Resources/licenses"} {
		if err = os.MkdirAll(filepath.Join(contents, sub), 0755); err != nil {
			return "", err
		}
	}
	if err = copyRegular(binary, filepath.Join(contents, "MacOS", "app"), 0755); err != nil {
		return "", err
	}
	if err = copyRegular(resolve(b.Icon), filepath.Join(contents, "Resources", "AppIcon.icns"), 0644); err != nil {
		return "", err
	}
	for _, license := range b.Licenses {
		if err = copyRegular(resolve(license), filepath.Join(contents, "Resources/licenses", filepath.Base(license)), 0644); err != nil {
			return "", err
		}
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict>`
	for _, pair := range [][2]string{{"CFBundleName", b.Name}, {"CFBundleDisplayName", b.Name}, {"CFBundleIdentifier", b.ID}, {"CFBundleShortVersionString", b.Version}, {"CFBundleVersion", b.Version}, {"CFBundleExecutable", "app"}, {"CFBundleIconFile", "AppIcon.icns"}, {"CFBundlePackageType", "APPL"}} {
		plist += "<key>" + pair[0] + "</key><string>" + plistString(pair[1]) + "</string>"
	}
	plist += `<key>NSHighResolutionCapable</key><true/></dict></plist>`
	if err = os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0644); err != nil {
		return "", err
	}
	run := func(args ...string) error { return Execute(ctx, dir, args, os.Environ(), output) }
	if err = run("codesign", "--force", "--sign", "-", app); err != nil {
		return "", err
	}
	if err = run("codesign", "--verify", "--strict", app); err != nil {
		return "", err
	}
	imageRoot := filepath.Join(stage, "image")
	if err = os.Mkdir(imageRoot, 0755); err != nil {
		return "", err
	}
	if err = run("ditto", app, filepath.Join(imageRoot, b.Name+".app")); err != nil {
		return "", err
	}
	if err = os.Symlink("/Applications", filepath.Join(imageRoot, "Applications")); err != nil {
		return "", err
	}
	dmg := filepath.Join(release, b.Name+"-darwin-"+c.GOARCH+".dmg")
	if err = run("hdiutil", "create", "-volname", b.Name, "-srcfolder", imageRoot, "-format", "UDZO", dmg); err != nil {
		return "", err
	}
	if err = run("hdiutil", "verify", dmg); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		return "", errors.New("appbuild: release destination appeared during packaging")
	}
	if err = os.Rename(release, destination); err != nil {
		return "", err
	}
	return destination, nil
}
