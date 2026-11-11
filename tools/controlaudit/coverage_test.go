package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryCatalogItemRetainsCoverageRecord(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "migration/controls/reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reference Inventory
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(root, "migration/controls/status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var statuses struct {
		Controls []struct{ ID, Status, Implementation, Symbol string }
	}
	if err := json.Unmarshal(data, &statuses); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	catalog := map[string]bool{}
	for _, c := range reference.Catalog {
		catalog[c.ID] = true
	}
	for _, c := range statuses.Controls {
		if seen[c.ID] || !catalog[c.ID] {
			t.Fatalf("unknown/duplicate control %s", c.ID)
		}
		seen[c.ID] = true
		if c.Status == "pending" {
			continue
		}
		if c.Status != "partial" {
			t.Fatalf("unreviewed completion status %s", c.ID)
		}
		if !strings.HasPrefix(c.Implementation, "omniui/baseui/") || !strings.HasSuffix(c.Implementation, ".go") {
			t.Fatalf("control is not implemented in Go baseui: %s", c.ID)
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, c.Implementation), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == c.Symbol {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing implementation symbol %s for %s", c.Symbol, c.ID)
		}
	}
	if len(seen) != len(catalog) {
		t.Fatal("catalog entries were omitted")
	}
}
