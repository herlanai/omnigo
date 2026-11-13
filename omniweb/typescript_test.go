package omniweb

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type wireNode struct {
	Name     string            `json:"name"`
	Next     *wireNode         `json:"next,omitempty"`
	Data     []byte            `json:"data"`
	Children []wireNode        `json:"children"`
	Metadata map[string]string `json:"metadata"`
	Created  time.Time         `json:"created"`
	Count    int64             `json:"count,string"`
	Payload  json.RawMessage   `json:"payload"`
	Ignore   string            `json:"-"`
}

func TestGenerateTypeScriptWireContracts(t *testing.T) {
	router := NewRouter(1, 1024)
	if err := Bind(router, "node.get", "", func(context.Context, wireNode) (wireNode, error) {
		t.Error("generation executed handler")
		return wireNode{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Bind(router, "app.version", "", func(context.Context, struct{}) (string, error) { return "1", nil }); err != nil {
		t.Fatal(err)
	}
	source, err := GenerateTypeScript(router)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`export type Services`, `"app.version"`, `"next"?: (Wire2 | null)`, `"data": (string | null)`, `"count": string`, `"created": string`, `"payload": unknown`} {
		if !strings.Contains(string(source), part) {
			t.Errorf("missing %s in %s", part, source)
		}
	}
	second, err := GenerateTypeScript(router)
	if err != nil || string(source) != string(second) {
		t.Fatal("unstable output", err)
	}
	compiler, err := exec.LookPath("tsc")
	if err != nil {
		t.Skip("TypeScript compiler unavailable; generation assertions passed")
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "services.ts"), source, 0600); err != nil {
		t.Fatal(err)
	}
	sdk, err := os.ReadFile("client/index.ts")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "sdk.ts"), sdk, 0600)
	use := `import { OmniWebClient, HTTPTransport } from "./sdk"; import type { Services } from "./services"; const client = new OmniWebClient<Services>(new HTTPTransport()); const version: Promise<string> = client.call("app.version", {});`
	os.WriteFile(filepath.Join(dir, "use.ts"), []byte(use), 0600)
	cmd := exec.Command(compiler, "--noEmit", "--strict", "--target", "ES2022", "--module", "ESNext", "--lib", "ES2022,DOM", "use.ts")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("TypeScript compile: %s %v", output, err)
	}
}

type customWire string

func (customWire) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }

type recursiveContainer []recursiveContainer

func TestGenerateRejectsAmbiguousWireTypes(t *testing.T) {
	cases := map[string]func(*Router) error{
		"embedded": func(r *Router) error {
			return Bind(r, "bad", "", func(context.Context, struct{ wireNode }) (int, error) { return 0, nil })
		},
		"custom": func(r *Router) error {
			return Bind(r, "bad", "", func(context.Context, customWire) (int, error) { return 0, nil })
		},
		"channel": func(r *Router) error {
			return Bind(r, "bad", "", func(context.Context, chan int) (int, error) { return 0, nil })
		},
		"recursive slice": func(r *Router) error {
			return Bind(r, "bad", "", func(context.Context, recursiveContainer) (int, error) { return 0, nil })
		},
	}
	for name, bind := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewRouter(1, 1024)
			if err := bind(r); err != nil {
				t.Fatal(err)
			}
			if source, err := GenerateTypeScript(r); err == nil || source != nil {
				t.Fatal("unsupported type generated", string(source))
			}
		})
	}
}
