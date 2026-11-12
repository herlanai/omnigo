package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHasFlag(t *testing.T) {
	for _, args := range [][]string{{"--output", "a b"}, {"--output=a b"}, {"-output=a b"}} {
		if !hasFlag(args, "output") {
			t.Fatalf("missed %v", args)
		}
	}
	if hasFlag([]string{"--output-other=x"}, "output") {
		t.Fatal("matched another option")
	}
}
func TestCopyBundle(t *testing.T) {
	src := filepath.Join(t.TempDir(), "source")
	dst := filepath.Join(t.TempDir(), "destination")
	if err := os.MkdirAll(filepath.Join(src, "Versions/A"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "Versions/A/host"), []byte("native executable"), 0755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Symlink("A", filepath.Join(src, "Versions/Current")); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "Versions/A/host"))
	if err != nil || string(data) != "native executable" {
		t.Fatalf("copy: %q %v", data, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dst, "Versions/A/host"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0755 {
			t.Fatal("executable permission lost")
		}
		link, err := os.Readlink(filepath.Join(dst, "Versions/Current"))
		if err != nil || link != "A" {
			t.Fatalf("symlink: %q %v", link, err)
		}
	}
}
func TestAuditInventory(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, dir := range []string{"omniui/baseui", "migration/controls", "reference/backend/ui/testdata", "reference/backend/ui/editor"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile("omniui/baseui/tree.go", []byte("package baseui\nfunc TreeOutline[K comparable]() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("omniui/baseui/ignored_test.go", []byte("func Missing() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	source := "func Outline[K comparable](items []K) Element {\nfunc Missing() Element {\nfunc Helper() string {\n"
	if err := os.WriteFile("reference/backend/ui/api_gen.go", []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err := audit([]string{"--source", "reference"}); err != nil {
		t.Fatal(err)
	}
	var output struct {
		Constructors []record
		Subpackages  []string
	}
	if err := readJSON("migration/controls/ui.json", &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Constructors) != 2 || output.Constructors[0]["symbol"] != "TreeOutline" || output.Constructors[0]["status"] != "partial" || output.Constructors[1]["implementation"] != nil {
		t.Fatalf("inventory: %#v", output)
	}
	if len(output.Subpackages) != 1 || output.Subpackages[0] != "backend/ui/editor" {
		t.Fatalf("subpackages: %v", output.Subpackages)
	}
}
