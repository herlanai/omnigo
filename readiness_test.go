package omnigo

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/herlanai/omnigo/omniwindow/host"
)

func TestReadinessAndApplicationPaths(t *testing.T) {
	calls := 0
	app, err := New(Options{Name: "TestReadiness", InitialWindow: "main", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{}, nil
	}, OnReady: func(a *Application) {
		calls++
		if a.Name() != "TestReadiness" {
			t.Error(a.Name())
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "private")
	if err = app.SetPath(PathUserData, path); err != nil {
		t.Fatal(err)
	}
	if got, e := app.Path(PathUserData); e != nil || got != path {
		t.Fatal(got, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = app.WhenReady(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	exit := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- app.run(context.Background(), func(ctx context.Context, o host.Options) error {
			o.OnReady(&host.Controller{})
			o.OnReady(&host.Controller{})
			close(entered)
			<-exit
			return nil
		})
	}()
	<-entered
	if err = app.WhenReady(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("ready callback count", calls)
	}
	close(exit)
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
	if err = app.WhenReady(t.Context()); !errors.Is(err, ErrStopped) {
		t.Fatal(err)
	}
}
func TestReadinessUnblocksOnStartupFailure(t *testing.T) {
	failure := errors.New("failed start")
	app, err := New(Options{InitialWindow: "main", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{}, nil
	}, Services: []Service{{ID: "test.fails", Start: func(context.Context) error { return failure }}}})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- app.WhenReady(t.Context()) }()
	if err = app.run(t.Context(), func(context.Context, host.Options) error { t.Error("host started"); return nil }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err = <-result; !errors.Is(err, ErrStopped) {
		t.Fatal(err)
	}
}
