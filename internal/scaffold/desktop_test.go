package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herlanai/omnigo/internal/appbuild"
)

// External modules exercise import visibility and the exact generated build config.
func TestDesktopProjectsPublicBuild(t *testing.T) {
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"go", "webview"} {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), kind)
			if err := CreateProject(dir, "example.com/desktop", framework, kind, `Notes "中文" & Tools`, "com.example.notes"); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "go", "mod", "tidy")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "CGO_ENABLED=0")
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("tidy: %v\n%s", err, b)
			}
			var log bytes.Buffer
			run := func(ctx context.Context, dir string, argv, env []string, out io.Writer) error {
				return appbuild.Execute(ctx, dir, argv, append(env, "GOWORK=off", "GOFLAGS=-mod=readonly", "GOPROXY=off"), out)
			}
			path, err := appbuild.Build(t.Context(), filepath.Join(dir, "omnigo.json"), &log, run)
			if err != nil {
				t.Fatalf("generated build: %v\n%s", err, log.String())
			}
			if info, err := os.Stat(path); err != nil || info.Size() == 0 {
				t.Fatal("missing executable", err)
			}
			icon, err := os.ReadFile(filepath.Join(dir, "app/assets/icon.png"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(icon))
			if err != nil || cfg.Width != 1024 || cfg.Height != 1024 {
				t.Fatal(cfg, err)
			}
			var c appbuild.Config
			data, _ := os.ReadFile(filepath.Join(dir, "omnigo.json"))
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatal(err)
			}
			for _, path := range c.Bundle.Licenses {
				if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "package.json")); !os.IsNotExist(err) {
				t.Fatal("desktop starter unexpectedly requires Node", err)
			}
			if kind == "webview" {
				page, _ := os.ReadFile(filepath.Join(dir, "app/web/index.html"))
				if strings.Contains(string(page), `Notes "中文" & Tools`) {
					t.Fatal("HTML title not escaped")
				}
			}
		})
	}
}

func TestProjectOptionsRejectBeforeWriting(t *testing.T) {
	root, _ := filepath.Abs("../..")
	for _, tc := range [][3]string{{"unknown", "App", "com.example.app"}, {"go", "../escape", "com.example.app"}, {"webview", "App", "bad id"}, {"go", "A\nB", "com.example.app"}} {
		dest := filepath.Join(t.TempDir(), "absent")
		if err := CreateProject(dest, "example.com/app", root, tc[0], tc[1], tc[2]); err == nil {
			t.Fatal(tc)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatal("invalid options created project", tc, err)
		}
	}
}

func TestMobileProjectUsesPublicAPI(t *testing.T) {
	framework, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "mobile")
	if err = CreateProject(dir, "example.com/mobile", framework, "mobile", "Mobile Notes", "com.example.notes"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "go", "test", "-mod=mod", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off", "GOPROXY=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("public mobile API: %v\n%s", err, output)
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "app/AndroidManifest.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "org.golang.app.GoNativeActivity") || strings.Contains(string(manifest), "uses-sdk") {
		t.Fatal("invalid gomobile manifest")
	}
	var c appbuild.Config
	raw, _ := os.ReadFile(filepath.Join(dir, "omnigo.json"))
	if err = json.Unmarshal(raw, &c); err != nil || c.Mobile == nil {
		t.Fatal(err)
	}
	if err = c.Mobile.Validate(); err != nil {
		t.Fatal(err)
	}
}
