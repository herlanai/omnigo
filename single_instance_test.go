package omnigo

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
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

type testLaunchDispatcher struct{ q *dispatch.Queue }

func (d testLaunchDispatcher) Post(ctx context.Context, f func() error) (<-chan error, error) {
	return d.q.Submit(ctx, f)
}
func TestLaunchDeliveryWaitsForUIAndPropagatesFailure(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	owner, err := instance.Open(t.Context(), root, "delivery")
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ready := make(chan launchDispatcher, 1)
	q := dispatch.New(2)
	defer q.Close()
	result := make(chan error, 1)
	failure := errors.New("callback failed")
	called := false
	go func() {
		result <- deliverLaunches(ctx, owner, ready, func(l Launch) error {
			called = true
			if len(l.Args) != 1 || l.Args[0] != "later" {
				t.Error(l)
			}
			return failure
		})
	}()
	if err := instance.Notify(ctx, root, "delivery", Launch{Args: []string{"later"}}); err != nil {
		t.Fatal(err)
	}
	ready <- testLaunchDispatcher{q}
	<-q.Wake()
	if called {
		t.Fatal("callback ran before UI drain")
	}
	q.Drain(1)
	if err := <-result; !errors.Is(err, failure) || !called {
		t.Fatalf("%v %v", called, err)
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

func TestHandoffRetriesOnlyBeforeSend(t *testing.T) {
	for _, becomesOwner := range []bool{false, true} {
		attempts, notifications, waits := 0, 0, 0
		marker := &instance.Lease{}
		lease, forwarded, err := acquireOrForward(t.Context(), func() (*instance.Lease, error) {
			attempts++
			if becomesOwner && attempts == 2 {
				return marker, nil
			}
			return nil, instance.ErrRunning
		}, func() error {
			notifications++
			if notifications == 1 {
				return instance.ErrNotReady
			}
			return nil
		}, func(context.Context) error { waits++; return nil })
		if err != nil || waits != 1 || attempts != 2 || forwarded == becomesOwner {
			t.Fatalf("%v %v %d %d", err, forwarded, waits, attempts)
		}
		if becomesOwner && lease != marker {
			t.Fatal("lost ownership")
		}
	}
	ambiguous := errors.New("ack lost after send")
	for _, failure := range []error{ambiguous, instance.ErrBusy, os.ErrPermission} {
		_, _, err := acquireOrForward(t.Context(), func() (*instance.Lease, error) { return nil, instance.ErrRunning }, func() error { return failure }, func(context.Context) error { t.Error("unsafe retry"); return nil })
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
	}
}
func TestHandoffCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, _, err := acquireOrForward(ctx, func() (*instance.Lease, error) { return nil, instance.ErrRunning }, func() error { return instance.ErrNotReady }, func(ctx context.Context) error { cancel(); return ctx.Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestActivationBeforeHostReady(t *testing.T) {
	app := instanceApp(t, t.TempDir(), nil)
	if _, err := app.Activate(t.Context()); !errors.Is(err, ErrNotReady) {
		t.Fatal(err)
	}
}
