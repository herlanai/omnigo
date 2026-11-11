// Package scaffold creates the default TanStack Start + shadcn Web UI project.
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"omnigo.dev/omnigo/omniweb/client"
)

//go:embed all:template
var templates embed.FS

var moduleName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)+$`)

func Create(destination, module, framework string) error {
	if !moduleName.MatchString(module) {
		return errors.New("scaffold: supply a valid module path, e.g. example.com/myapp")
	}
	if framework == "" {
		return errors.New("scaffold: --framework must point to the local OmniGo repository until published")
	}
	framework, err := filepath.Abs(framework)
	if err != nil {
		return err
	}
	mod, err := os.ReadFile(filepath.Join(framework, "go.mod"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(mod), "module omnigo.dev/omnigo\n") {
		return errors.New("scaffold: framework is not an OmniGo checkout")
	}
	// Mkdir, not MkdirAll: refuse an existing destination, including an empty one.
	if err := os.Mkdir(destination, 0755); err != nil {
		return fmt.Errorf("scaffold: destination must not exist: %w", err)
	}
	write := func(name string, data []byte) error {
		path := filepath.Join(destination, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		return os.WriteFile(path, data, 0644)
	}
	err = fs.WalkDir(templates, "template", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := templates.ReadFile(path)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(path, "template/"), ".tmpl")
		return write(name, data)
	})
	if err != nil {
		return fmt.Errorf("scaffold: partial project retained at %s: %w", destination, err)
	}
	if err := write("ui/src/lib/omniweb.ts", []byte(client.TypeScript)); err != nil {
		return err
	}
	return write("go.mod", []byte(fmt.Sprintf("module %s\n\ngo 1.26.0\n\nrequire omnigo.dev/omnigo v0.0.0\n\nreplace omnigo.dev/omnigo => %s\n", module, strconv.Quote(filepath.ToSlash(framework)))))
}
