package scaffold

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/omniweb/client"
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

// A separate module exercises Go's internal import boundary, unlike tests
// running under github.com/herlanai/omnigo itself.
func TestGeneratedProjectUsesPublicAPI(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "external")
	if err := Create(destination, "example.com/external", root); err != nil {
		t.Fatal(err)
	}
	source := `package external_test
import (
 "context"
 "testing"
 "github.com/herlanai/omnigo"
 "github.com/herlanai/omnigo/omniweb"
 ui "github.com/herlanai/omnigo/omniui/baseui"
 "github.com/herlanai/omnigo/omniui/plugins/automation"
)
func TestPublicTypes(t *testing.T) {
 _ = omnigo.Options{Lifecycle:omnigo.LifecycleOptions{Permissions:[]string{"read"}}}
 var snapshot ui.TextSnapshot = ui.NewTextBuffer("hello").Snapshot()
 var summary ui.TextSummary = snapshot.Summary()
 if summary.Bytes != 5 {t.Fatal(summary)}
 queue := automation.NewDispatchQueue(1); defer queue.Close()
 router := omniweb.NewRouter(1,1024)
 if err:=omniweb.Bind(router,"echo","",func(_ context.Context,n int)(int,error){return n,nil});err!=nil{t.Fatal(err)}
 result,err:=router.Call(t.Context(),"echo",[]byte("42"),nil)
 if err!=nil || string(result)!="42" {t.Fatal(string(result),err)}
}
`
	if err := os.WriteFile(filepath.Join(destination, "public_api_test.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./...")
	command.Dir = destination
	command.Env = append(os.Environ(), "GOWORK=off", "CGO_ENABLED=0", "GOPROXY=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("external project failed: %v\n%s", err, output)
	}
}

func TestCreateAcceptsEmptyAndNestedDestinations(t *testing.T) {
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, existing := range []bool{false, true} {
		destination := filepath.Join(t.TempDir(), "nested", "app")
		if existing {
			if err := os.MkdirAll(destination, 0755); err != nil {
				t.Fatal(err)
			}
		}
		if err := Create(destination, "example.com/app", framework); err != nil {
			t.Fatal(existing, err)
		}
		for _, name := range []string{"go.mod", "app/main.go", "ui/src/lib/omniweb.ts"} {
			if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := os.Stat(filepath.Join(destination, ".omnigo-init.lock")); !os.IsNotExist(err) {
			t.Fatal("initialization lock retained", err)
		}
	}
}

func TestCreatePreservesNonemptyDestinationIncludingHiddenFiles(t *testing.T) {
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"owned.txt", ".gitkeep", ".omnigo-init.lock"} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("owned"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := Create(dir, "example.com/app", framework); err == nil {
			t.Fatal("nonempty directory accepted")
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 {
			t.Fatal(entries, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != "owned" {
			t.Fatal("existing content changed", err)
		}
	}
}

func TestConcurrentInitializersProduceOneCompleteProject(t *testing.T) {
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "app")
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() { <-start; results <- Create(destination, "example.com/app", framework) }()
	}
	close(start)
	successes := 0
	for range 2 {
		if err := <-results; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("expected one successful initializer", successes)
	}
	sdk, err := os.ReadFile(filepath.Join(destination, "ui/src/lib/omniweb.ts"))
	if err != nil || string(sdk) != client.TypeScript {
		t.Fatal("incomplete SDK", err)
	}
	mod, err := os.ReadFile(filepath.Join(destination, "go.mod"))
	if err != nil || !strings.HasPrefix(string(mod), "module example.com/app\n") {
		t.Fatal("incomplete module", err)
	}
}
