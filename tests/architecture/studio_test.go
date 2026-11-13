package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestStudioUsesFrameworkInsteadOfNativeBindings(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("..", "..", "examples", "studio"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		tree, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range tree.Imports {
			dependency, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(dependency, "github.com/ebitengine/purego") || strings.Contains(dependency, "examples/studio/internal/native") {
				t.Errorf("application contains native implementation: %s imports %s", path, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
