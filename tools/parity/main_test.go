package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryDeterministicAndBodyFree(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                          "module reference\n",
		"backend/api.go":                  "package ref\ntype Config struct { Name string }\nfunc Start(c Config) error { panic(\"IMPLEMENTATION_SENTINEL\") }\nfunc hidden() {}\n",
		"backend/plugins/demo/service.go": "package demo\nfunc Open() {}\n",
		"backend/internal/host/host.go":   "package host\nfunc Private() {}\n",
		"node_modules/ignored.ts":         "ignore me",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(t.TempDir(), "inventory.json")
	if err := run(root, out); err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(a), "IMPLEMENTATION_SENTINEL") {
		t.Fatal("copied implementation body")
	}
	var inv Inventory
	if err := json.Unmarshal(a, &inv); err != nil {
		t.Fatal(err)
	}
	if len(inv.Files) != 3 || len(inv.API) != 3 || len(inv.Plugins) != 1 {
		t.Fatalf("unexpected counts: %+v", inv)
	}
	if err := run(root, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("inventory not deterministic")
	}
}
