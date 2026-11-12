// Command featureaudit inventories reference internals and tests without copying implementations.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Path         string   `json:"path"`
	SHA256       string   `json:"sha256"`
	Declarations []string `json:"declarations,omitempty"`
	Tests        []string `json:"tests,omitempty"`
}
type Feature struct {
	ID    string `json:"id"`
	Files []File `json:"files"`
}

func groups(p string) []string {
	var out []string
	name := filepath.Base(p)
	if strings.Contains(p, "child_window") {
		out = append(out, "childwindow")
	}
	if strings.Contains(name, "clipboard") || strings.HasPrefix(p, "backend/transfer/") {
		out = append(out, "clipboard")
	}
	if strings.HasPrefix(p, "backend/internal/diff/") || strings.Contains(name, "diff_view") {
		out = append(out, "diff")
	}
	if strings.HasPrefix(p, "backend/internal/e2e/") {
		out = append(out, "e2e")
	}
	if strings.HasPrefix(p, "backend/i18n/") || strings.Contains(name, "i18n") || strings.Contains(name, "locale") {
		out = append(out, "i18n")
	}
	for _, dir := range []string{"gpu", "metal", "d3d11", "opengl", "gl", "shader", "shaderir"} {
		if strings.HasPrefix(p, "backend/internal/"+dir+"/") {
			out = append(out, "gpu")
			return out
		}
	}
	if strings.Contains(name, "gpu") || strings.Contains(name, "metal") || strings.Contains(name, "d3d11") || strings.HasPrefix(p, "backend/ui/shader/") {
		out = append(out, "gpu")
	}
	return out
}
func scan(root string) ([]Feature, error) {
	return scanMode(root, false)
}
func scanMode(root string, internal bool) ([]Feature, error) {
	all := map[string]*Feature{}
	ids := []string{"childwindow", "clipboard", "diff", "e2e", "gpu", "i18n"}
	if internal {
		ids = nil
	}
	for _, id := range ids {
		all[id] = &Feature{ID: id, Files: []File{}}
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "build" || d.Name() == "dist") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ids := groups(rel)
		if internal {
			ids = nil
			if rest, ok := strings.CutPrefix(rel, "backend/internal/"); ok {
				id, _, _ := strings.Cut(rest, "/")
				ids = []string{id}
				if all[id] == nil {
					all[id] = &Feature{ID: id, Files: []File{}}
				}
			}
		}
		if len(ids) == 0 {
			return nil
		}
		switch filepath.Ext(rel) {
		case ".go", ".md", ".ts", ".json", ".metal", ".glsl", ".hlsl":
		default:
			if !internal {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		item := File{Path: rel, SHA256: hex.EncodeToString(sum[:])}
		if strings.HasSuffix(rel, ".go") {
			f, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
			if err != nil {
				return err
			}
			for _, decl := range f.Decls {
				switch v := decl.(type) {
				case *ast.FuncDecl:
					name := v.Name.Name
					if v.Recv != nil {
						name = "method:" + name
					}
					if v.Recv == nil && strings.HasSuffix(rel, "_test.go") && (strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Fuzz")) {
						item.Tests = append(item.Tests, name)
					} else {
						item.Declarations = append(item.Declarations, name)
					}
				case *ast.GenDecl:
					for _, s := range v.Specs {
						if t, ok := s.(*ast.TypeSpec); ok {
							item.Declarations = append(item.Declarations, "type:"+t.Name.Name)
						}
					}
				}
			}
			sort.Strings(item.Declarations)
			sort.Strings(item.Tests)
		}
		for _, id := range ids {
			all[id].Files = append(all[id].Files, item)
		}
		return nil
	})
	var result []Feature
	if internal {
		for id := range all {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}
	for _, id := range ids {
		result = append(result, *all[id])
	}
	return result, err
}
func main() {
	source := flag.String("source", "", "read-only reference repository")
	out := flag.String("out", "migration/features/reference.json", "inventory output")
	internal := flag.Bool("internal", false, "inventory every file and package under backend/internal")
	flag.Parse()
	if *source == "" {
		fmt.Fprintln(os.Stderr, "-source required")
		os.Exit(2)
	}
	if *internal && *out == "migration/features/reference.json" {
		*out = "migration/internal/reference.json"
	}
	features, err := scanMode(*source, *internal)
	if err != nil {
		panic(err)
	}
	data, err := json.MarshalIndent(struct {
		Schema   int       `json:"schema"`
		Policy   string    `json:"policy"`
		Features []Feature `json:"features"`
	}{1, "Path-based discovery of internals and tests, not complete behavioral coverage. No implementation bodies copied. A reference test name is not OmniGo test evidence.", features}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	for _, f := range features {
		n := 0
		for _, file := range f.Files {
			n += len(file.Tests)
		}
		fmt.Printf("%s: %d files, %d test/benchmark/fuzz declarations\n", f.ID, len(f.Files), n)
	}
}
