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
