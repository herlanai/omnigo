package omnigo

import (
	"github.com/herlanai/omnigo/internal/codegen"
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
	return codegen.WriteTypeScript(a.router, path)
}
