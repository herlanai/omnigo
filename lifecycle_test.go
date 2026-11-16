package omnigo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/herlanai/omnigo/omniwindow/host"
	"github.com/herlanai/omnigo/omniwindow/instance"
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

func instanceApp(t *testing.T, root string, services []Service) *Application {
	t.Helper()
	app, err := New(Options{SingleInstance: &SingleInstanceOptions{ID: "test", Directory: root}, InitialWindow: "test", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{}, nil
	}, Services: services})
	if err != nil {
		t.Fatal(err)
	}
	return app
}
func TestSecondLaunchSkipsServicesAndNative(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	owner, err := instance.Open(t.Context(), root, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	app := instanceApp(t, root, []Service{{ID: "test.never", Start: func(context.Context) error { t.Error("secondary started services"); return nil }}})
	if err := app.run(t.Context(), func(context.Context, host.Options) error { t.Error("secondary opened window"); return nil }); err != nil {
		t.Fatal(err)
	}
	launch := <-owner.Messages()
	wd, _ := os.Getwd()
	if !reflect.DeepEqual(launch.Args, os.Args[1:]) || launch.WorkingDir != wd {
		t.Fatalf("%+v", launch)
	}
}
func TestInstanceReleasedOnStartupFailureAndNativeExit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{true: "failure", false: "exit"}[fail], func(t *testing.T) {
			root := t.TempDir()
			os.Chmod(root, 0700)
			failure := errors.New("startup failure")
			services := []Service{}
			if fail {
				services = append(services, Service{ID: "test.fail", Start: func(context.Context) error { return failure }})
			}
			app := instanceApp(t, root, services)
			err := app.run(t.Context(), func(context.Context, host.Options) error { return nil })
			if fail && !errors.Is(err, failure) {
				t.Fatal(err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			next, err := instance.Open(t.Context(), root, "test")
			if err != nil {
				t.Fatal(err)
			}
			next.Close()
		})
	}
}
func TestSingleInstanceConfigSnapshot(t *testing.T) {
	root := t.TempDir()
	options := &SingleInstanceOptions{ID: "one", Directory: root}
	app, err := New(Options{SingleInstance: options, InitialWindow: "test", CreateWindow: func(string, bool, host.WindowServices) (host.ContentWindowOptions, error) {
		return host.ContentWindowOptions{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	options.ID = "changed"
	if app.options.SingleInstance.ID != "one" {
		t.Fatal("options not snapshotted")
	}
	if _, err := New(Options{SingleInstance: &SingleInstanceOptions{}}); err == nil {
		t.Fatal("empty ID")
	}
}

func TestInstanceOwnershipHeldDuringServiceShutdown(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := false
	app := instanceApp(t, root, []Service{{ID: "test.shutdown", Start: func(context.Context) error { return nil }, Stop: func(context.Context) error {
		stopped = true
		l, err := instance.Open(context.Background(), root, "test")
		if l != nil {
			l.Close()
			t.Error("ownership released before service shutdown")
		}
		if !errors.Is(err, instance.ErrRunning) {
			t.Errorf("lock: %v", err)
		}
		return nil
	}}})
	_ = app.run(ctx, func(context.Context, host.Options) error { cancel(); return nil })
	if !stopped {
		t.Fatal("shutdown not called")
	}
	l, err := instance.Open(t.Context(), root, "test")
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}

func TestActivationBeforeHostReady(t *testing.T) {
	app := instanceApp(t, t.TempDir(), nil)
	if _, err := app.Activate(t.Context()); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}
