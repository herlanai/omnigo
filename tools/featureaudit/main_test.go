package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInternalTestsAndOwnershipAreInventoried(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "backend/internal/e2e")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	code := "package e2e\nfunc TestChildWindowAboveParent(t *testing.T) { panic(\"never execute reference\") }\n"
	if err := os.WriteFile(filepath.Join(dir, "child_window_test.go"), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result[0].Files) != 1 || len(result[3].Files) != 1 || !reflect.DeepEqual(result[0].Files[0].Tests, []string{"TestChildWindowAboveParent"}) {
		t.Fatal(result)
	}
	if len(result[0].Files[0].SHA256) != 64 {
		t.Fatal("missing fingerprint")
	}
}
func TestFeatureDiscovery(t *testing.T) {
	for p, want := range map[string]string{"backend/internal/diff/myers.go": "diff", "backend/internal/gpu/damage.go": "gpu", "backend/transfer/data.go": "clipboard", "backend/i18n/number.go": "i18n"} {
		if got := groups(p); len(got) != 1 || got[0] != want {
			t.Fatal(p, got)
		}
	}
}

func TestAllInternalDirectoriesAndNativeSources(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"backend/internal/rope/text.go", "backend/internal/windows/native.cpp", "backend/internal/gpu/shader.metal", "backend/not-internal/skip.go"} {
		path := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		data := []byte("native source")
		if filepath.Ext(path) == ".go" {
			data = []byte("package reference\nfunc Keep() {}\n")
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scanMode(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != "gpu" || got[1].ID != "rope" || got[2].ID != "windows" {
		t.Fatal(got)
	}
	for _, group := range got {
		if len(group.Files) != 1 || len(group.Files[0].SHA256) != 64 {
			t.Fatal(group)
		}
	}
}
