// Package mobilebuild packages experimental Go control mobile applications.
// It never changes the desktop compiler environment or signing configuration.
package mobilebuild

import (
	"archive/zip"
	"context"
	"debug/elf"
	"debug/macho"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"golang.org/x/image/draw"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const ToolVersion = "v0.0.0-20261011034528-197770444867"

type Config struct {
	Package string `json:"package"`
	Output  string `json:"output"`
	Name    string `json:"name"`
	ID      string `json:"id"`
	Version string `json:"version"`
}
type Target struct{ Name, Arch string }

func (t Target) Validate() error {
	if t.Name != "ios-simulator" && t.Name != "ios" && t.Name != "android" {
		return errors.New("mobilebuild: target must be ios-simulator, ios or android")
	}
	if t.Arch != "arm64" && t.Arch != "amd64" {
		return errors.New("mobilebuild: arch must be arm64 or amd64")
	}
	if t.Name == "ios" && t.Arch != "arm64" {
		return errors.New("mobilebuild: iOS devices require arm64")
	}
	return nil
}
func (c Config) Validate() error {
	if !strings.HasPrefix(c.Package, "./") || strings.Contains(c.Package, "..") || c.Output == "" {
		return errors.New("mobilebuild: package must be a local ./ path and output is required")
	}
	if c.Name == "" || strings.ContainsAny(c.Name, "/\\:\x00\r\n") || c.Name == "." || c.Name == ".." {
		return errors.New("mobilebuild: invalid name")
	}
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9]*)+$`).MatchString(c.ID) {
		return errors.New("mobilebuild: id must be a dotted identifier")
	}
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`).MatchString(c.Version) {
		return errors.New("mobilebuild: invalid version")
	}
	return nil
}
func environment(values map[string]string) []string {
	out := []string{}
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if _, ok := values[key]; !ok {
			out = append(out, e)
		}
	}
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}
func command(ctx context.Context, dir string, env []string, out io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
func xcrun(ctx context.Context, sdk, flag string) (string, error) {
	out, err := exec.CommandContext(ctx, "xcrun", "--sdk", sdk, flag).Output()
	if err != nil {
		return "", fmt.Errorf("mobilebuild: Xcode %s: %w", sdk, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Build publishes a fresh target directory. Existing builds are never removed.
// iOS device output is unsigned; installation/export needs app-owned provisioning.
func Build(ctx context.Context, dir string, c Config, t Target, out io.Writer) (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	if err := t.Validate(); err != nil {
		return "", err
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	destination := c.Output
	if !filepath.IsAbs(destination) {
		destination = filepath.Join(dir, destination)
	}
	destination = filepath.Join(destination, t.Name+"-"+t.Arch)
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return "", errors.New("mobilebuild: target directory exists or cannot be inspected; choose a new output")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".mobile-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if t.Name == "android" {
		err = buildAndroid(ctx, dir, stage, c, t, out)
	} else {
		err = buildIOS(ctx, dir, stage, c, t, out)
	}
	if err != nil {
		return "", err
	}
	metadata, _ := json.MarshalIndent(map[string]any{"target": t.Name, "arch": t.Arch, "cgo": true, "id": c.ID, "version": c.Version, "distribution": "development; not store-ready"}, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, "build.json"), metadata, 0644); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		return "", errors.New("mobilebuild: output appeared while building")
	}
	if err = os.Rename(stage, destination); err != nil {
		return "", err
	}
	return destination, nil
}
func buildIOS(ctx context.Context, dir, stage string, c Config, t Target, out io.Writer) error {
	if runtime.GOOS != "darwin" {
		return errors.New("mobilebuild: iOS requires macOS and Xcode")
	}
	sdk, triple := "iphoneos", "arm64-apple-ios15.0"
	if t.Name == "ios-simulator" {
		sdk = "iphonesimulator"
		arch := t.Arch
		if arch == "amd64" {
			arch = "x86_64"
		}
		triple = arch + "-apple-ios15.0-simulator"
	}
	sdkpath, err := xcrun(ctx, sdk, "--show-sdk-path")
	if err != nil {
		return err
	}
	cc, err := exec.CommandContext(ctx, "xcrun", "--sdk", sdk, "--find", "clang").Output()
	if err != nil {
		return err
	}
	app := filepath.Join(stage, c.ID+".app")
	if err = os.Mkdir(app, 0755); err != nil {
		return err
	}
	flags := "-isysroot " + strconvQuote(sdkpath) + " -target " + triple
	env := environment(map[string]string{"GOOS": "ios", "GOARCH": t.Arch, "CGO_ENABLED": "1", "CC": strings.TrimSpace(string(cc)), "CGO_CFLAGS": flags, "CGO_LDFLAGS": flags, "GOWORK": "off"})
	binary := filepath.Join(app, "app")
	if err = command(ctx, dir, env, out, "go", "build", "-trimpath", "-ldflags=-linkmode=external -w", "-o", binary, c.Package); err != nil {
		return err
	}
	exe, err := macho.Open(binary)
	if err != nil {
		return err
	}
	cpu := exe.Cpu
	exe.Close()
	if t.Arch == "arm64" && cpu != macho.CpuArm64 || t.Arch == "amd64" && cpu != macho.CpuAmd64 {
		return errors.New("mobilebuild: incorrect executable architecture")
	}
	assets := filepath.Join(dir, c.Package, "assets")
	if _, err := os.Stat(assets); err == nil {
		if err = os.CopyFS(filepath.Join(app, "assets"), os.DirFS(assets)); err != nil {
			return err
		}
	}
	hasIcon := false
	if f, err := os.Open(filepath.Join(assets, "icon.png")); err == nil {
		icon, e := png.Decode(f)
		f.Close()
		if e != nil {
			return e
		}
		for _, size := range []int{120, 180, 152, 167} {
			dst := image.NewRGBA(image.Rect(0, 0, size, size))
			draw.CatmullRom.Scale(dst, dst.Bounds(), icon, icon.Bounds(), draw.Src, nil)
			f, e := os.Create(filepath.Join(app, fmt.Sprintf("Icon%d.png", size)))
			if e != nil {
				return e
			}
			e = png.Encode(f, dst)
			closeErr := f.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
		}
		hasIcon = true
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>`
	for _, kv := range [][2]string{{"CFBundleName", c.Name}, {"CFBundleDisplayName", c.Name}, {"CFBundleIdentifier", c.ID}, {"CFBundleVersion", c.Version}, {"CFBundleShortVersionString", c.Version}, {"CFBundleExecutable", "app"}, {"CFBundlePackageType", "APPL"}, {"MinimumOSVersion", "15.0"}} {
		plist += "<key>" + kv[0] + "</key><string>" + escape(kv[1]) + "</string>"
	}
	if hasIcon {
		plist += `<key>CFBundleIcons</key><dict><key>CFBundlePrimaryIcon</key><dict><key>CFBundleIconFiles</key><array><string>Icon120</string><string>Icon180</string></array></dict></dict><key>CFBundleIcons~ipad</key><dict><key>CFBundlePrimaryIcon</key><dict><key>CFBundleIconFiles</key><array><string>Icon152</string><string>Icon167</string></array></dict></dict>`
	}
	plist += `<key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array><key>UILaunchScreen</key><dict/><key>UIRequiresFullScreen</key><true/></dict></plist>`
	if err = os.WriteFile(filepath.Join(app, "Info.plist"), []byte(plist), 0644); err != nil {
		return err
	}
	if t.Name == "ios-simulator" {
		return command(ctx, dir, os.Environ(), out, "codesign", "--force", "--sign", "-", app)
	}
	return nil
}
func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// cgo flags are parsed by the Go tool, not a shell.
func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
func buildAndroid(ctx context.Context, dir, stage string, c Config, t Target, out io.Writer) error {
	if _, _, err := androidTools(); err != nil {
		return err
	}
	if _, err := exec.LookPath("gomobile"); err != nil {
		return fmt.Errorf("mobilebuild: install gomobile@%s and run gomobile init: %w", ToolVersion, err)
	}
	// Identity belongs to the checked-in manifest. Reject mismatches instead of
	// silently rewriting project files or producing an APK with the wrong ID.
	data, err := os.ReadFile(filepath.Join(dir, c.Package, "AndroidManifest.xml"))
	if err != nil {
		return fmt.Errorf("mobilebuild: AndroidManifest.xml required beside main.go: %w", err)
	}
	var manifest struct {
		Package string `xml:"package,attr"`
		Version string `xml:"versionName,attr"`
	}
	if err = xml.Unmarshal(data, &manifest); err != nil {
		return err
	}
	if manifest.Package != c.ID || manifest.Version != c.Version {
		return errors.New("mobilebuild: manifest package/version differs from mobile config")
	}
	apk := filepath.Join(stage, c.Name+".apk")
	if err = command(ctx, dir, environment(map[string]string{"CGO_ENABLED": "1", "GOWORK": "off"}), out, "gomobile", "build", "-target=android/"+t.Arch, "-androidapi=23", "-o", apk, c.Package); err != nil {
		return err
	}
	if err = finalizeAndroid(ctx, dir, apk, c, data, out); err != nil {
		return err
	}
	z, err := zip.OpenReader(apk)
	if err != nil {
		return err
	}
	defer z.Close()
	abi := "arm64-v8a"
	machine := elf.EM_AARCH64
	if t.Arch == "amd64" {
		abi = "x86_64"
		machine = elf.EM_X86_64
	}
	found := false
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "lib/"+abi+"/") && strings.HasSuffix(f.Name, ".so") {
			r, err := f.Open()
			if err != nil {
				return err
			}
			header := make([]byte, 64)
			_, err = io.ReadFull(r, header)
			r.Close()
			if err != nil {
				return err
			}
			if len(header) < 20 || string(header[:4]) != "\x7fELF" || header[4] != 2 || header[5] != 1 || uint16(header[18])|uint16(header[19])<<8 != uint16(machine) {
				return errors.New("mobilebuild: wrong Android native library architecture")
			}
			found = true
		}
	}
	if !found {
		return errors.New("mobilebuild: APK contains no native library for target ABI")
	}
	return nil
}
