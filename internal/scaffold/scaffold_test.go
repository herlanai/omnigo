package scaffold

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"omnigo.dev/omnigo/omniweb/client"
)

func TestDefaultProjectAndNoOverwrite(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "app")
	if err := Create(destination, "example.com/app", root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "app/main.go", "ui/package.json", "ui/components.json", "ui/vite.config.ts", "ui/src/routes/index.tsx"} {
		if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), filepath.Join(destination, "app/main.go"), nil, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "ui/src/lib/omniweb.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != client.TypeScript {
		t.Fatal("SDK is not canonical")
	}
	if err := Create(destination, "example.com/app", root); err == nil {
		t.Fatal("existing project overwritten")
	}
	if err := Create(filepath.Join(t.TempDir(), "bad"), "bad\nmodule evil", root); err == nil {
		t.Fatal("module injection accepted")
	}
}

func TestTemplateLockIsPortable(t *testing.T) {
	data, err := templates.ReadFile("template/ui/package-lock.json")
	if err != nil {
		t.Fatal(err)
	}
	var lock struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) < 2 {
		t.Fatal("missing dependency lock")
	}
	for path := range lock.Packages {
		if path != "" && !strings.HasPrefix(path, "node_modules/") {
			t.Fatalf("nonportable dependency path %q", path)
		}
	}
	if strings.Contains(string(data), "file:") {
		t.Fatal("local file dependency in default template")
	}
}
