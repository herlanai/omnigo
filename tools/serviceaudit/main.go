// Command serviceaudit partitions a fingerprint/API inventory without discarding
// mixed files. It never imports the reference or copies implementation bodies.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Group  string `json:"group"`
}
type API struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	Signature string `json:"signature"`
	Group     string `json:"group"`
}
type Inventory struct {
	Schema   int    `json:"schema"`
	Files    []File `json:"files"`
	API      []API  `json:"api"`
	Deferred []File `json:"deferred_controls"`
}

func classify(path string) string {
	for _, prefix := range []string{"backend/ui/", "backend/internal/gpu/", "backend/internal/text/", "backend/internal/layout/", "backend/internal/paint/"} {
		if strings.HasPrefix(path, prefix) {
			return "deferred-controls"
		}
	}
	if strings.HasPrefix(path, "backend/plugins/") {
		parts := strings.Split(path, "/")
		if len(parts) > 2 {
			switch parts[2] {
			case "borderbeam", "glass", "loading", "game", "scene3d", "sfsymbols":
				return "deferred-controls"
			}
			if strings.Contains(path, "/native/") {
				return "deferred-controls"
			}
			return "plugins"
		}
	}
	if strings.HasPrefix(path, "backend/cmd/") || strings.HasPrefix(path, "cli/") || strings.Contains(path, "/template/") {
		return "toolchain"
	}
	if strings.HasPrefix(path, "frontend/") {
		return "omniweb"
	}
	if strings.HasPrefix(path, "backend/internal/") {
		return "desktop"
	}
	if strings.HasPrefix(path, "backend/") {
		name := filepath.Base(path)
		for _, prefix := range []string{"app.", "active", "single_instance", "deeplink", "events", "signal", "session"} {
			if strings.HasPrefix(name, prefix) {
				return "lifecycle"
			}
		}
		for _, prefix := range []string{"bind", "bridge", "channel", "ipc", "typescript", "modules", "frontend", "webview", "page", "cookies", "content_rules"} {
			if strings.HasPrefix(name, prefix) {
				return "omniweb"
			}
		}
		if strings.Contains(path, "/examples/") {
			return "examples"
		}
		return "desktop"
	}
	// Root configuration, scripts and docs require explicit manual review rather
	// than disappearing because they are not exported Go APIs.
	return "toolchain"
}
func main() {
	input := flag.String("inventory", "migration/reference.json", "fresh tools/parity inventory")
	output := flag.String("out", "migration/noncontrols/reference.json", "partitioned output")
	flag.Parse()
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(input, output string) error {
	raw, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var inv Inventory
	if err = json.Unmarshal(raw, &inv); err != nil {
		return err
	}
	out := Inventory{Schema: 1}
	for _, file := range inv.Files {
		file.Group = classify(file.Path)
		if file.Group == "deferred-controls" {
			out.Deferred = append(out.Deferred, file)
		} else {
			out.Files = append(out.Files, file)
		}
	}
	for _, api := range inv.API {
		api.Group = classify(api.File)
		if api.Group != "deferred-controls" {
			out.API = append(out.API, api)
		}
	}
	sort.Slice(out.Files, func(i, j int) bool { return out.Files[i].Path < out.Files[j].Path })
	raw, err = json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(output, append(raw, '\n'), 0644)
}
