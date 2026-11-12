package omnigo

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/herlanai/omnigo/runtime/rpc"
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
