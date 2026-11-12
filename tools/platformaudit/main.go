// Command platformaudit inventories reference interfaces without importing implementations.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
)

type entry struct {
	Interface string `json:"interface"`
	Method    string `json:"method"`
	Signature string `json:"signature"`
	Line      int    `json:"line"`
}

func inventory(path string) ([]entry, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, path, nil, 0)
	if err != nil {
		return nil, err
	}
	result := []entry{}
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			t, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			it, ok := t.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, field := range it.Methods.List {
				var out bytes.Buffer
				if err := printer.Fprint(&out, fs, field.Type); err != nil {
					return nil, err
				}
				names := field.Names
				if len(names) == 0 {
					result = append(result, entry{t.Name.Name, "<embedded>", out.String(), fs.Position(field.Pos()).Line})
				}
				for _, name := range names {
					result = append(result, entry{t.Name.Name, name.Name, out.String(), fs.Position(field.Pos()).Line})
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Interface != b.Interface {
			return a.Interface < b.Interface
		}
		return a.Method < b.Method
	})
	return result, nil
}
func main() {
	source := flag.String("source", "", "read-only golet repository root")
	out := flag.String("out", "migration/platform/reference.json", "interface inventory")
	flag.Parse()
	if *source == "" {
		panic("-source is required")
	}
	path := filepath.Join(*source, "backend/internal/platform/platform.go")
	items, err := inventory(path)
	if err != nil {
		panic(err)
	}
	payload := struct {
		Source  string  `json:"source"`
		Policy  string  `json:"policy"`
		Methods []entry `json:"methods"`
	}{"backend/internal/platform/platform.go", "Read-only API inventory. No implementation copied; presence does not establish parity.", items}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Inventoried %d reference interface entries\n", len(items))
}
