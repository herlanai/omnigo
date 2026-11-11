package desktop

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"omnigo.dev/omnigo/omniui"
)

type driverFunc func(context.Context) error

func (f driverFunc) Run(ctx context.Context) error { return f(ctx) }

func TestRollbackAndIndependentShutdown(t *testing.T) {
	var log []string
	var registry omniui.Registry
	failure := errors.New("failure")
	for _, id := range []string{"test.a", "test.b", "test.c"} {
		id := id
		err := registry.Register(omniui.Extension{Descriptor: omniui.Descriptor{ID: id, Version: "1", Kind: omniui.Service}, Start: func(ctx context.Context) error {
			log = append(log, "start "+id)
			if id == "test.c" {
				return failure
			}
			return nil
		}, Stop: func(ctx context.Context) error {
			if ctx.Err() != nil {
				t.Error("cleanup canceled prematurely")
			}
			log = append(log, "stop "+id)
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	app := New(Options{Registry: &registry, Extensions: []string{"test.c", "test.b", "test.a"}})
	err := app.Run(context.Background(), driverFunc(func(context.Context) error { t.Fatal("driver called after start failure"); return nil }))
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	want := []string{"start test.a", "start test.b", "start test.c", "stop test.b", "stop test.a"}
	if !reflect.DeepEqual(log, want) {
		t.Fatal(log)
	}
}

func TestPanicCleanupContinues(t *testing.T) {
	var registry omniui.Registry
	stopped := false
	for _, id := range []string{"test.a", "test.b"} {
		id := id
		if err := registry.Register(omniui.Extension{Descriptor: omniui.Descriptor{ID: id, Version: "1", Kind: omniui.Service}, Stop: func(context.Context) error {
			if id == "test.b" {
				panic("bad extension")
			}
			stopped = true
			return nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	app := New(Options{Registry: &registry, Extensions: []string{"test.a", "test.b"}})
	err := app.Run(context.Background(), driverFunc(func(context.Context) error { panic("driver panic") }))
	if err == nil || !stopped {
		t.Fatalf("err=%v stopped=%v", err, stopped)
	}
	if err := app.Run(context.Background(), driverFunc(func(context.Context) error { return nil })); err == nil {
		t.Fatal("second run accepted")
	}
}
