package host

import (
	"go/build"
	"strings"
	"testing"
)

// Prevent a successful cross-build from silently selecting the unsupported stub.
func TestDarwinArchitecturesSelectNativeHost(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		c := build.Default
		c.GOOS = "darwin"
		c.GOARCH = arch
		c.CgoEnabled = false
		p, err := c.ImportDir(".", 0)
		if err != nil {
			t.Fatal(err)
		}
		files := strings.Join(p.GoFiles, " ")
		if !strings.Contains(files, "cocoa_darwin.go") || strings.Contains(files, "unsupported.go") {
			t.Fatalf("%s selected wrong host: %s", arch, files)
		}
	}
}

func TestLinuxWebKitOptInSelectsRealHost(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		c := build.Default
		c.GOOS = "linux"
		c.GOARCH = arch
		c.CgoEnabled = false
		c.BuildTags = []string{"omnigo_webkitgtk"}
		p, err := c.ImportDir(".", 0)
		if err != nil {
			t.Fatal(err)
		}
		files := strings.Join(p.GoFiles, " ")
		if !strings.Contains(files, "webkit_linux.go") || strings.Contains(files, "unsupported.go") {
			t.Fatalf("%s selected wrong host: %s", arch, files)
		}
	}
}

func TestWindowsWebView2OptInSelectsRealHost(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		c := build.Default
		c.GOOS = "windows"
		c.GOARCH = arch
		c.CgoEnabled = false
		c.BuildTags = []string{"omnigo_webview2"}
		p, err := c.ImportDir(".", 0)
		if err != nil {
			t.Fatal(err)
		}
		files := strings.Join(p.GoFiles, " ")
		if !strings.Contains(files, "webview2_windows.go") || strings.Contains(files, "unsupported.go") {
			t.Fatalf("%s selected wrong host: %s", arch, files)
		}
	}
}
