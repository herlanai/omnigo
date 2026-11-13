package apphost

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow/instance"
)

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
	ready := make(chan LaunchDispatcher, 1)
	q := dispatch.New(2)
	defer q.Close()
	result := make(chan error, 1)
	failure := errors.New("callback failed")
	called := false
	go func() {
		result <- DeliverLaunches(ctx, owner, ready, func(l Launch) error {
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
