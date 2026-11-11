// Command controlaudit records every UI declaration and test identity in the
// reference, including advanced editors and plugin UI, without implementation bodies.
package main

import (
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
	"strconv"
	"strings"
)

type Declaration struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	Signature string `json:"signature"`
}
type Catalog struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Status string `json:"status"`
	Target string `json:"target"`
}
type Test struct {
	File string `json:"file"`
	Name string `json:"name"`
}
type Inventory struct {
	Schema       int           `json:"schema"`
	Catalog      []Catalog     `json:"catalog"`
	Declarations []Declaration `json:"declarations"`
	Tests        []Test        `json:"tests"`
}

func main() {
	source := flag.String("source", "", "read-only reference checkout")
	reference := flag.String("reference", "migration/reference.json", "existing declaration snapshot")
	out := flag.String("out", "migration/controls/reference.json", "output")
	flag.Parse()
	if err := run(*source, *reference, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(source, reference, out string) error {
	if source == "" {
		return fmt.Errorf("-source required")
	}
	data, err := os.ReadFile(reference)
	if err != nil {
		return err
	}
	var snapshot struct {
		API []Declaration `json:"api"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}
	inv := Inventory{Schema: 1}
	for _, d := range snapshot.API {
		if strings.HasPrefix(d.File, "backend/ui/") || (strings.HasPrefix(d.File, "backend/plugins/") && strings.Contains(d.Signature, "ui.")) {
			inv.Declarations = append(inv.Declarations, d)
		}
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(source, "backend/ui/forma/catalog.go"), nil, 0)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "Components" && fn.Name.Name != "AIComponents") {
			continue
		}
		group := "forma"
		if fn.Name.Name == "AIComponents" {
			group = "ai"
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			literal, ok := n.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			name, err := strconv.Unquote(literal.Value)
			if err == nil {
				inv.Catalog = append(inv.Catalog, Catalog{ID: name, Group: group, Status: "pending", Target: "omniui/baseui"})
			}
			return true
		})
	}
	err = filepath.WalkDir(filepath.Join(source, "backend"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(path, "_test.go") || (!strings.HasPrefix(rel, "backend/ui/") && !strings.HasPrefix(rel, "backend/plugins/")) {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && (strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Fuzz") || strings.HasPrefix(fn.Name.Name, "Benchmark")) {
				inv.Tests = append(inv.Tests, Test{rel, fn.Name.Name})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(inv.Catalog, func(i, j int) bool { return inv.Catalog[i].ID < inv.Catalog[j].ID })
	data, err = json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(out, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("%d catalog entries, %d UI declarations, %d test/benchmark contracts\n", len(inv.Catalog), len(inv.Declarations), len(inv.Tests))
	return nil
}
