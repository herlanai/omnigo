// Package lucide embeds official Lucide SVGs. It performs no runtime IO outside
// the binary and parses only requested icons. Import icon alone for custom SVGs.
package lucide

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"omnigo.dev/omnigo/omniui/icon"
)

const Version = "1.71.0"

//go:embed icons/*.svg
var assets embed.FS
var parsed sync.Map

// Names returns an independent, sorted list, including the upstream aliases.
func Names() []string {
	entries, _ := fs.ReadDir(assets, "icons")
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, strings.TrimSuffix(e.Name(), ".svg"))
	}
	sort.Strings(out)
	return out
}

// SVG returns owned source bytes, suitable for applications which export icons.
func SVG(name string) ([]byte, error) {
	if name == "" || strings.ContainsAny(name, "/\\.") {
		return nil, fmt.Errorf("lucide: invalid icon name %q", name)
	}
	data, e := assets.ReadFile("icons/" + name + ".svg")
	if e != nil {
		return nil, fmt.Errorf("lucide: unknown icon %q", name)
	}
	return data, nil
}
func Get(name string) (*icon.SVG, error) {
	if v, ok := parsed.Load(name); ok {
		return v.(*icon.SVG), nil
	}
	data, e := SVG(name)
	if e != nil {
		return nil, e
	}
	v, e := icon.ParseSVG(data)
	if e != nil {
		return nil, fmt.Errorf("lucide %s: %w", name, e)
	}
	actual, _ := parsed.LoadOrStore(name, v)
	return actual.(*icon.SVG), nil
}
func Must(name string) *icon.SVG {
	v, e := Get(name)
	if e != nil {
		panic(e)
	}
	return v
}
func Search() *icon.SVG       { return Must("search") }
func Check() *icon.SVG        { return Must("check") }
func X() *icon.SVG            { return Must("x") }
func Plus() *icon.SVG         { return Must("plus") }
func Settings() *icon.SVG     { return Must("settings") }
func Menu() *icon.SVG         { return Must("menu") }
func ChevronDown() *icon.SVG  { return Must("chevron-down") }
func ChevronRight() *icon.SVG { return Must("chevron-right") }
func Copy() *icon.SVG         { return Must("copy") }
func Trash2() *icon.SVG       { return Must("trash-2") }
func Download() *icon.SVG     { return Must("download") }
func Upload() *icon.SVG       { return Must("upload") }
