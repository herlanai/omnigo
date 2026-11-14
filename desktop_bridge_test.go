package omnigo

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/omniweb"
	"strings"
	"testing"
	"testing/fstest"
)

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
