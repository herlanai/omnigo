package nativewindow

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/runtime/dispatch"
)

type actionFake struct {
	fakeBackend
	applied []action
	failure error
}

func (b *actionFake) apply(a action) error { b.applied = append(b.applied, a); return b.failure }
func TestActionsRunOnlyDuringUIDrain(t *testing.T) {
	b := &actionFake{}
	ctx := context.Background()
	w, _ := New(Options{})
	done, err := w.SetTitle(ctx, "新的标题")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.applied) != 0 {
		t.Fatal("native action ran outside UI loop")
	}
	if err = w.run(ctx, func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil || len(b.applied) != 1 || b.applied[0].text != "新的标题" {
		t.Fatal(err, b.applied)
	}
	if _, err = w.Show(ctx); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatal(err)
	}
}
func TestActionsRejectInvalidAndCancelledRequests(t *testing.T) {
	w, _ := New(Options{})
	for _, v := range []float64{-1, 2, math.NaN(), math.Inf(1)} {
		if _, err := w.SetOpacity(context.Background(), v); err == nil {
			t.Fatal(v)
		}
	}
	if _, err := w.SetContentBounds(context.Background(), Bounds{Width: 0, Height: 1}); err == nil {
		t.Fatal("invalid bounds")
	}
	if _, err := w.SetTitle(context.Background(), "a\x00b"); err == nil {
		t.Fatal("NUL title")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done, err := w.Maximize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	b := &actionFake{}
	if err = w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if err = <-done; !errors.Is(err, context.Canceled) || len(b.applied) != 0 {
		t.Fatal(err, b.applied)
	}
}
func TestActionErrorsPropagateWithoutClosingWindow(t *testing.T) {
	want := errors.New("OS refused activation")
	b := &actionFake{failure: want}
	w, _ := New(Options{})
	done, _ := w.Focus(context.Background())
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(<-done, want) {
		t.Fatal("native failure lost")
	}
}
func TestNativeCloseCanBeVetoed(t *testing.T) {
	attempts := 0
	b := &actionFake{fakeBackend: fakeBackend{events: [][]Event{{{Kind: "close"}}, {{Kind: "close"}}}}}
	w, _ := New(Options{ShouldClose: func() bool { attempts++; return attempts > 1 }})
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || b.closed != 1 {
		t.Fatal(attempts, b.closed)
	}
}
func TestProgrammaticCloseBypassesVetoAndRejectsPending(t *testing.T) {
	w, _ := New(Options{ShouldClose: func() bool { t.Fatal("programmatic Close must bypass veto"); return false }})
	done, _ := w.Close(context.Background())
	pending, _ := w.Hide(context.Background())
	b := &actionFake{}
	if err := w.run(context.Background(), func(omniwindow.WindowOptions) (backend, error) { return b, nil }); err != nil {
		t.Fatal(err)
	}
	if <-done != nil || !errors.Is(<-pending, dispatch.ErrClosed) || len(b.applied) != 0 || b.closed != 1 {
		t.Fatal("shutdown contract broken")
	}
}
