package omnigo

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/herlanai/omnigo/omniwindow/host"
	"github.com/herlanai/omnigo/omniwindow/instance"
)

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
