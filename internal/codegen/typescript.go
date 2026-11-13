// Package codegen owns generation workflows used by application entry points.
package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/herlanai/omnigo/internal/runtime/rpc"
	"github.com/herlanai/omnigo/omniweb"
)

// WriteTypeScript generates before touching output and atomically replaces
// changed content where os.Rename supports it. Identical output is untouched.
func WriteTypeScript(router *rpc.Router, path string) error {
	if path == "" {
		return fmt.Errorf("omnigo: TypeScript output path required")
	}
	data, err := omniweb.GenerateTypeScript(router)
	if err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".omni-types-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
