// Package scaffold creates the default TanStack Start + shadcn Web UI project.
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/herlanai/omnigo/omniweb/client"
	"golang.org/x/mod/modfile"
	modmodule "golang.org/x/mod/module"
)

//go:embed all:template
var templates embed.FS

func validateModule(name string) error {
	if err := modmodule.CheckImportPath(name); err != nil {
		return fmt.Errorf("scaffold: invalid module path: %w", err)
	}
	if name == "go" || name == "toolchain" {
		return errors.New("scaffold: module path is reserved for the Go toolchain")
	}
	if _, _, ok := modmodule.SplitPathVersion(name); !ok {
		return errors.New("scaffold: invalid module major-version suffix")
	}
	return nil
}

func Create(destination, module, framework string) (result error) {
	if err := validateModule(module); err != nil {
		return err
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
	parsed, err := modfile.Parse(filepath.Join(framework, "go.mod"), mod, nil)
	if err != nil {
		return fmt.Errorf("scaffold: invalid framework go.mod: %w", err)
	}
	if parsed.Module == nil || parsed.Module.Mod.Path != "github.com/herlanai/omnigo" {
		return errors.New("scaffold: framework is not an OmniGo checkout")
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("scaffold: destination must be an empty directory: %w", err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("scaffold: destination must be empty")
	}
	// Serialize cooperating initializers after the emptiness check. All writes
	// are exclusive so another writer cannot have a file silently replaced.
	const lock = ".omnigo-init.lock"
	if err := root.Mkdir(lock, 0700); err != nil {
		return fmt.Errorf("scaffold: initialization already in progress: %w", err)
	}
	defer func() {
		if err := root.Remove(lock); err != nil {
			result = errors.Join(result, fmt.Errorf("scaffold: project retained at %s; removing initialization lock: %w", destination, err))
		}
	}()
	write := func(name string, data []byte) error {
		local := filepath.FromSlash(name)
		if err := root.MkdirAll(filepath.Dir(local), 0755); err != nil {
			return err
		}
		file, err := root.OpenFile(local, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(data)
		return errors.Join(writeErr, file.Close())
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
		return fmt.Errorf("scaffold: partial project retained at %s: %w", destination, err)
	}
	if err := write("go.mod", []byte(fmt.Sprintf("module %s\n\ngo 1.26.0\n\nrequire github.com/herlanai/omnigo v0.0.0\n\nreplace github.com/herlanai/omnigo => %s\n", module, strconv.Quote(filepath.ToSlash(framework))))); err != nil {
		return fmt.Errorf("scaffold: partial project retained at %s: %w", destination, err)
	}
	return nil
}
