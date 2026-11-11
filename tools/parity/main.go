// Command parity records source fingerprints and exported declarations without
// importing or copying implementation bodies from the reference repository.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type API struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	Signature string `json:"signature"`
	Status    string `json:"status"`
}
type Inventory struct {
	Schema  int      `json:"schema"`
	Source  string   `json:"source"`
	Files   []File   `json:"files"`
	API     []API    `json:"api"`
	Plugins []string `json:"plugins"`
}

func main() {
	source := flag.String("source", "", "reference repository directory")
	output := flag.String("out", "migration/reference.json", "output inventory")
	flag.Parse()
	if *source == "" {
		fmt.Fprintln(os.Stderr, "-source is required")
		os.Exit(2)
	}
	if err := run(*source, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(source, output string) error {
	if info, err := os.Stat(filepath.Join(source, "go.mod")); err != nil || info.IsDir() {
		return fmt.Errorf("reference must contain go.mod")
	}
	inv := Inventory{Schema: 1, Source: "local golet reference; fingerprints only"}
	plugins := map[string]bool{}
	err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != source && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "dist" || d.Name() == "build" || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ext := filepath.Ext(rel)
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".json", ".md", ".h", ".m", ".mm", ".c", ".cpp", ".metal", ".hlsl", ".glsl":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		inv.Files = append(inv.Files, File{rel, hex.EncodeToString(hash[:])})
		parts := strings.Split(rel, "/")
		if len(parts) > 2 && parts[0] == "backend" && parts[1] == "plugins" {
			plugins[parts[2]] = true
		}
		if ext != ".go" || strings.HasSuffix(rel, "_test.go") || !strings.HasPrefix(rel, "backend/") || strings.Contains(rel, "/internal/") || strings.Contains(rel, "/examples/") || strings.Contains(rel, "/cmd/") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, data, 0)
		if err != nil {
			return err
		}
		print := func(node any) string { var b bytes.Buffer; _ = format.Node(&b, fset, node); return b.String() }
		add := func(name, sig string) {
			inv.API = append(inv.API, API{ID: filepath.ToSlash(filepath.Dir(rel)) + ":" + name, File: rel, Signature: sig, Status: "pending"})
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if !ast.IsExported(decl.Name.Name) {
					continue
				}
				name := decl.Name.Name
				if decl.Recv != nil {
					name = print(decl.Recv.List[0].Type) + "." + name
				}
				decl.Body = nil
				add(name, print(decl))
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(spec.Name.Name) {
							add(spec.Name.Name, "type "+print(spec))
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if ast.IsExported(name.Name) {
								sig := decl.Tok.String() + " " + name.Name
								if spec.Type != nil {
									sig += " " + print(spec.Type)
								}
								add(name.Name, sig)
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for p := range plugins {
		inv.Plugins = append(inv.Plugins, p)
	}
	sort.Strings(inv.Plugins)
	sort.Slice(inv.API, func(i, j int) bool {
		if inv.API[i].ID == inv.API[j].ID {
			return inv.API[i].File < inv.API[j].File
		}
		return inv.API[i].ID < inv.API[j].ID
	})
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("Recorded %d files, %d exported declarations, %d plugins\n", len(inv.Files), len(inv.API), len(inv.Plugins))
	return nil
}
