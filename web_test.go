package omnigo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/herlanai/omnigo/internal/runtime/rpc"
	"github.com/herlanai/omnigo/omniweb"
)

func TestDeclarativeAppBindings(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{}, Window: &WindowOptions{Title: "test"}, Registrations: []Registration{
		Query("value", func() int { return 42 }, "read"),
		Method("echo", func(_ context.Context, n int) (int, error) { return n, nil }),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if app.window == nil || app.window.URL != "/" {
		t.Fatal("missing default URL")
	}
	if _, err = app.router.Call(t.Context(), "value", []byte(`{}`), nil); !errors.Is(err, rpc.ErrForbidden) {
		t.Fatal(err)
	}
	out, err := app.router.Call(t.Context(), "value", []byte(`{}`), []string{"read"})
	if err != nil || string(out) != "42" {
		t.Fatal(string(out), err)
	}
	if _, err = app.router.Call(t.Context(), "value", []byte(`{"unknown":true}`), []string{"read"}); !errors.Is(err, rpc.ErrInvalidRequest) {
		t.Fatal(err)
	}
	if _, err := New(Options{Assets: fstest.MapFS{}, Registrations: []Registration{Group(Query("same", func() int { return 1 }), Query("same", func() int { return 2 }))}}); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestGenerationModeDoesNotStartApplication(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {}}, Services: []Service{{ID: "test.never", Start: func(context.Context) error { t.Error("started service"); return nil }}}, Registrations: []Registration{Query("app.version", func() string { t.Error("invoked handler"); return "1" })}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ui", "services.ts")
	t.Setenv("OMNIGO_GENERATE", path)
	if err = app.RunMain(); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = app.RunMain(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical generated content rewritten")
	}
	if err = Bind(app, "invalid", "", func(context.Context, chan int) (int, error) { return 0, nil }); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(path)
	if err = app.WriteTypeScript(path); err == nil {
		t.Fatal("unsupported type accepted")
	}
	current, _ := os.ReadFile(path)
	if string(original) != string(current) {
		t.Fatal("generation failure overwrote previous contracts")
	}
}

func TestDesktopBridgeRequiresExplicitPermissions(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {Data: []byte("test")}}, Name: "bridge-test", Registrations: []Registration{DesktopBridge()}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop.platform", "desktop.screen", "desktop.appearance", "desktop.titlebar", "desktop.dialog.open", "desktop.dialog.save", "desktop.dialog.message"} {
		if _, err := app.router.Call(context.Background(), name, []byte(`{}`), nil); !errors.Is(err, omniweb.ErrForbidden) {
			t.Fatal(name, err)
		}
	}
	if _, err := app.router.Call(context.Background(), "desktop.platform", []byte(`{}`), []string{"desktop.read"}); err != nil {
		t.Fatal(err)
	}
	generated, err := app.GenerateTypeScript()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"desktop.screen", "desktop.dialog.open", "desktop.titlebar"} {
		if !strings.Contains(string(generated), name) {
			t.Fatal("missing contract", name)
		}
	}
}
func TestDesktopBridgeNotImplicit(t *testing.T) {
	app, err := New(Options{Assets: fstest.MapFS{"index.html": {Data: []byte("test")}}, Name: "no-bridge"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.router.Call(context.Background(), "desktop.platform", []byte(`{}`), []string{"desktop.read"}); !errors.Is(err, omniweb.ErrNotFound) {
		t.Fatal(err)
	}
}
