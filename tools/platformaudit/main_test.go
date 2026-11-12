package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInventoryPreservesSignaturesAndEmbeddedInterfaces(t *testing.T) {
	file := filepath.Join(t.TempDir(), "platform.go")
	if err := os.WriteFile(file, []byte("package platform\ntype Window interface { Close() error; SetTitle(string); Parent }\ntype Parent interface { ID() uint64 }"), 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := inventory(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatal(entries)
	}
	found := false
	for _, e := range entries {
		if e.Interface == "Window" && e.Method == "<embedded>" && e.Signature == "Parent" {
			found = true
		}
	}
	if !found {
		t.Fatal("embedded interface dropped")
	}
}
