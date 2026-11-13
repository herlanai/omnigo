package omnigo

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/herlanai/omnigo/omniweb"
)

// GenerateTypeScript snapshots registered unary RPC contracts without starting
// services or a window. Bind all application methods before generating.
func (a *Application) GenerateTypeScript() ([]byte, error) {
	return omniweb.GenerateTypeScript(a.router)
}

// WriteTypeScript updates a generated Services contract. Identical content is
// untouched; generation errors leave the previous file intact. Replacement uses
// a same-directory rename (atomic on Unix; other filesystems follow os.Rename).
func (a *Application) WriteTypeScript(path string) error {
	if path == "" {
		return fmt.Errorf("omnigo: TypeScript output path required")
	}
	data, err := a.GenerateTypeScript()
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
