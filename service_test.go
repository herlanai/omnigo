package omnigo

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/herlanai/omnigo/omniwindow/host"
)

func TestServicesCancelJoinAndReverseCleanup(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	var order []string
	app, err := New(Options{
		GoWindow: func(WindowEnvironment) (ContentWindowOptions, error) { return ContentWindowOptions{}, nil },
		Services: []Service{
			{ID: "test.first", Start: func(context.Context) error { order = append(order, "start"); return nil }, Stop: func(context.Context) error {
				order = append(order, "stop")
				select {
				case <-stopped:
				default:
					t.Error("background not joined before cleanup")
				}
				return nil
			}},
			Background("test.worker", func(ctx context.Context) error { close(started); <-ctx.Done(); close(stopped); return ctx.Err() }),
		},
		Registrations: []Registration{func(app *Application) error {
			return Bind(app, "test.echo", "", func(_ context.Context, n int) (int, error) { return n, nil })
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = app.run(t.Context(), func(ctx context.Context, o host.Options) error {
		<-started
		if o.InitialWindow != "main" || o.CreateWindow == nil {
			t.Fatal("missing native content")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"start", "stop"}) {
		t.Fatal(order)
	}
}

func TestServiceStartupFailureUnwinds(t *testing.T) {
	failure := errors.New("start failed")
	stopped := false
	app, err := New(Options{GoWindow: func(WindowEnvironment) (ContentWindowOptions, error) { return ContentWindowOptions{}, nil }, Services: []Service{
		{ID: "test.a", Stop: func(ctx context.Context) error { stopped = true; return ctx.Err() }},
		{ID: "test.b", Start: func(context.Context) error { return failure }},
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.run(t.Context(), func(context.Context, host.Options) error { t.Fatal("host started after failure"); return nil })
	if !errors.Is(err, failure) || !stopped {
		t.Fatal(err, stopped)
	}
}

func TestBackgroundFailureReported(t *testing.T) {
	failure := errors.New("worker failed")
	app, err := New(Options{GoWindow: func(WindowEnvironment) (ContentWindowOptions, error) { return ContentWindowOptions{}, nil }, Services: []Service{Background("test.worker", func(context.Context) error { return failure })}})
	if err != nil {
		t.Fatal(err)
	}
	if err = app.run(t.Context(), func(context.Context, host.Options) error { return nil }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
}

func TestConfigurationConflictsAndModuleFailure(t *testing.T) {
	create := func(WindowEnvironment) (ContentWindowOptions, error) { return ContentWindowOptions{}, nil }
	if _, err := New(Options{GoWindow: create, InitialWindow: "other"}); err == nil {
		t.Fatal("conflicting windows accepted")
	}
	failure := errors.New("module failed")
	if _, err := New(Options{GoWindow: create, Registrations: []Registration{func(*Application) error { return failure }}}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := New(Options{GoWindow: create, Services: []Service{{ID: "same.id"}, {ID: "same.id"}}}); err == nil {
		t.Fatal("duplicate services accepted")
	}
}
