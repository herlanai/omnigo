package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
	"testing"
)

type fakeTray struct{ updates, closes int }

func (f *fakeTray) update(omniwindow.TrayOptions) error { f.updates++; return nil }
func (f *fakeTray) state() (omniwindow.TrayState, error) {
	return omniwindow.TrayState{Visible: true}, nil
}
func (f *fakeTray) popup() error { return nil }
func (f *fakeTray) dismiss()     {}
func (f *fakeTray) close()       { f.closes++ }
func TestTrayOwnershipAndUpdate(t *testing.T) {
	q := dispatch.New(8)
	driver := &fakeTray{}
	h := &Controller{queue: q, wake: func() {}, newTray: func(omniwindow.TrayOptions) (trayDriver, error) { return driver, nil }}
	out := make(chan *Tray, 1)
	go func() {
		tray, err := h.NewTray(t.Context(), omniwindow.TrayOptions{ID: "test", Title: "T"})
		if err != nil {
			t.Error(err)
		}
		out <- tray
	}()
	<-q.Wake()
	h.drain()
	tray := <-out
	call := func(fn func() error) error {
		result := make(chan error, 1)
		go func() { result <- fn() }()
		<-q.Wake()
		h.drain()
		return <-result
	}
	if err := call(func() error { return tray.Update(t.Context(), omniwindow.TrayOptions{ID: "test", Title: "U"}) }); err != nil {
		t.Fatal(err)
	}
	if err := call(func() error {
		_, err := h.NewTray(t.Context(), omniwindow.TrayOptions{ID: "test", Title: "duplicate"})
		return err
	}); err == nil {
		t.Fatal("duplicate ID accepted")
	}
	if driver.updates != 1 {
		t.Fatal(driver.updates)
	}
	if err := call(func() error { return tray.Close(t.Context()) }); err != nil {
		t.Fatal(err)
	}
	if err := call(func() error { _, err := tray.State(t.Context()); return err }); !errors.Is(err, ErrTrayClosed) {
		t.Fatal(err)
	}
	h.close()
	if err := tray.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if driver.closes != 1 {
		t.Fatal(driver.closes)
	}
}
func TestCanceledTrayCreationDoesNotLeak(t *testing.T) {
	q := dispatch.New(1)
	driver := &fakeTray{}
	entered, release := make(chan struct{}), make(chan struct{})
	h := &Controller{queue: q, wake: func() {}, newTray: func(omniwindow.TrayOptions) (trayDriver, error) { close(entered); <-release; return driver, nil }}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := h.NewTray(ctx, omniwindow.TrayOptions{ID: "test", Title: "T"}); result <- err }()
	<-q.Wake()
	drained := make(chan struct{})
	go func() { h.drain(); close(drained) }()
	<-entered
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	<-drained
	h.drain()
	h.close()
	if driver.closes != 1 {
		t.Fatal("late tray leaked", driver.closes)
	}
}

func TestRejectedTrayCreatesDoNotQueueCleanup(t *testing.T) {
	q := dispatch.New(1)
	h := &Controller{queue: q, wake: func() {}}
	defer h.close()
	if _, err := q.Submit(t.Context(), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		if _, err := h.NewTray(t.Context(), omniwindow.TrayOptions{ID: "test", Title: "T"}); !errors.Is(err, dispatch.ErrFull) {
			t.Fatal(err)
		}
	}
	if len(h.controls) != 0 {
		t.Fatal("rejected requests leaked cleanup work")
	}
}
