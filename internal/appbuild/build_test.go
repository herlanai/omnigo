package appbuild

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPreservesPreviousOnFailure(t *testing.T) {
	for _, mode := range []string{"fail", "missing", "cancel", "success"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			config := filepath.Join(dir, "omnigo.json")
			target := filepath.Join(dir, "app")
			os.WriteFile(config, []byte(`{"package":".","output":"app","goos":"darwin","goarch":"arm64","before":[["frontend","build"]]}`), 0600)
			os.WriteFile(target, []byte("old"), 0700)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			run := func(_ context.Context, wd string, args, env []string, _ io.Writer) error {
				calls++
				if wd != dir {
					t.Fatal(wd)
				}
				if calls == 1 {
					if args[0] != "frontend" {
						t.Fatal(args)
					}
					return nil
				}
				if !strings.Contains(strings.Join(env, "\n"), "CGO_ENABLED=0") {
					t.Fatal(env)
				}
				if mode == "fail" {
					return errors.New("compiler failed")
				}
				if mode == "missing" {
					return nil
				}
				if mode == "cancel" {
					cancel()
				}
				return os.WriteFile(args[4], []byte("new"), 0700)
			}
			_, err := Build(ctx, config, io.Discard, run)
			if (err == nil) != (mode == "success") {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(target)
			want := "old"
			if mode == "success" {
				want = "new"
			}
			if string(got) != want {
				t.Fatal(string(got))
			}
			leftovers, _ := filepath.Glob(filepath.Join(dir, ".omni-build-*"))
			if len(leftovers) != 0 {
				t.Fatal(leftovers)
			}
		})
	}
}
func TestBuildRejectsInvalidConfigurationBeforeExecution(t *testing.T) {
	for _, raw := range []string{`null`, `{"package":".","output":"app","goos":"linux","goarch":"arm64","typo":true}`, `{} {}`, `{"package":".","output":".","goos":"linux","goarch":"arm64"}`} {
		p := filepath.Join(t.TempDir(), "omnigo.json")
		os.WriteFile(p, []byte(raw), 0600)
		_, err := Build(context.Background(), p, io.Discard, func(context.Context, string, []string, []string, io.Writer) error {
			t.Fatal("executed invalid config")
			return nil
		})
		if err == nil {
			t.Fatal(raw)
		}
	}
}
