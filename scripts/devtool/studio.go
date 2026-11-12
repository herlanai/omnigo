package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func studio(ctx context.Context, root, mode string) error {
	mac := runtime.GOOS == "darwin"
	output := filepath.Join(root, "build/studio")
	bundle := filepath.Join(output, "omnigo-studio")
	if mac {
		bundle = filepath.Join(output, "Omni Studio.app")
	}
	contents := bundle
	if mac {
		contents = filepath.Join(bundle, "Contents")
	}
	binary := filepath.Join(contents, "MacOS", "omnigo-studio")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if mode == "start" {
		return run(ctx, binary)
	}
	if err := catalog(); err != nil {
		return err
	}
	if err := copyTree("omniweb/client/index.ts", "examples/studio/ui/src/lib/omniweb.ts"); err != nil {
		return err
	}
	if err := run(ctx, "pnpm", "--filter", "@omnigo/studio-ui", "build"); err != nil {
		return err
	}
	if err := os.RemoveAll("examples/studio/web/assets"); err != nil {
		return err
	}
	if err := copyTree("examples/studio/ui/dist/client", "examples/studio/web"); err != nil {
		return err
	}
	if err := run(ctx, "go", "run", "./examples/studio/cmd/appicon", "-out", filepath.Join(output, "AppIcon.icns"), "-preview", "examples/studio/assets/app-icon.png"); err != nil {
		return err
	}
	if err := os.RemoveAll(bundle); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		return err
	}
	if err := run(ctx, "go", "build", "-trimpath", "-o", binary, "./examples/studio/cmd/studio"); err != nil {
		return err
	}
	resources := filepath.Join(contents, "Resources")
	if err := licenses(ctx, filepath.Join(resources, "licenses")); err != nil {
		return err
	}
	if mac {
		if err := copyTree(filepath.Join(output, "AppIcon.icns"), filepath.Join(resources, "AppIcon.icns")); err != nil {
			return err
		}
		cef := filepath.Join(root, "build/chromium-runtime/Omni Browser.app")
		if _, err := os.Stat(cef); os.IsNotExist(err) {
			fmt.Println("CEF runtime not installed; run go run ./scripts/devtool chromium to include the browser.")
		} else if err != nil {
			return err
		} else {
			if err = copyTree(cef, filepath.Join(resources, "chromium/Omni Browser.app")); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"README.md", "THIRD_PARTY_NOTICES.md"} {
		if err := copyTree(filepath.Join("examples/studio", name), filepath.Join(resources, name)); err != nil {
			return err
		}
	}
	if mac {
		if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(studioPlist), 0644); err != nil {
			return err
		}
		if err := run(ctx, "codesign", "--force", "--sign", "-", bundle); err != nil {
			return err
		}
		if err := run(ctx, "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister", "-f", bundle); err != nil {
			return err
		}
	}
	fmt.Printf("Studio built: %s\nRun with go run ./scripts/devtool start. Native host: macOS amd64/arm64; Windows/Linux hosts remain unavailable.\n", bundle)
	if mode == "dev" {
		return run(ctx, binary)
	}
	if mode == "package" {
		platform := runtime.GOOS
		arch := runtime.GOARCH
		if platform == "windows" {
			platform = "win32"
		}
		if arch == "amd64" {
			arch = "x64"
		}
		archive := filepath.Join(output, "omnigo-studio-"+platform+"-"+arch)
		if mac {
			archive += ".zip"
			if err := run(ctx, "ditto", "-c", "-k", "--keepParent", bundle, archive); err != nil {
				return err
			}
		} else {
			archive += ".tar.gz"
			if err := run(ctx, "tar", "-czf", archive, "-C", output, filepath.Base(bundle)); err != nil {
				return err
			}
		}
		fmt.Printf("Package: %s\nLocal unsigned build; signing and notarization are not configured.\n", archive)
	}
	return nil
}

const studioPlist = `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>omnigo-studio</string><key>CFBundleIdentifier</key><string>dev.omnigo.studio</string><key>CFBundleName</key><string>Omni Studio</string><key>CFBundleDisplayName</key><string>Omni Studio</string><key>CFBundleVersion</key><string>2</string><key>CFBundleShortVersionString</key><string>0.1.0</string><key>CFBundleIconFile</key><string>AppIcon.icns</string><key>CFBundlePackageType</key><string>APPL</string><key>NSHighResolutionCapable</key><true/></dict></plist>`
