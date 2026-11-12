// Package studio embeds the built directory application inside the Go binary.
package studio

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var files embed.FS

func Assets() fs.FS {
	assets, err := fs.Sub(files, "web")
	if err != nil {
		panic(err)
	}
	return assets
}

//go:embed assets/app-icon.png
var appIcon []byte

// ApplicationIcon returns owned PNG bytes for explicit native Dock registration.
func ApplicationIcon() []byte { return append([]byte(nil), appIcon...) }
