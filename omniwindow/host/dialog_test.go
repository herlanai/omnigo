package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
	"testing"
)

func TestDialogResultAndBusy(t *testing.T) {
	q := dispatch.New(4)
	h := &Controller{queue: q, wake: func() {}}
	defer h.close()
	var finish func(dialogResult)
	h.startDialog = func(_ dialogRequest, done func(dialogResult)) (func(), error) { finish = done; return func() {}, nil }
	result := make(chan omniwindow.FileDialogResult, 1)
	go func() {
		r, err := h.OpenDialog(t.Context(), omniwindow.FileDialogOptions{})
		if err != nil {
			t.Error(err)
		}
		result <- r
	}()
	<-q.Wake()
	h.drain()
	busy := make(chan error, 1)
	go func() { _, err := h.MessageDialog(t.Context(), omniwindow.MessageDialogOptions{}); busy <- err }()
	<-q.Wake()
	h.drain()
	if err := <-busy; !errors.Is(err, ErrDialogBusy) {
		t.Fatal(err)
	}
	paths := []string{"/tmp/example"}
	finish(dialogResult{file: omniwindow.FileDialogResult{Paths: paths}})
	paths[0] = "changed"
	finish(dialogResult{err: errors.New("duplicate callback")})
	if r := <-result; len(r.Paths) != 1 || r.Paths[0] != "/tmp/example" {
		t.Fatal(r)
	}
}

func TestDialogCancelWithFullQueue(t *testing.T) {
	q := dispatch.New(1)
	h := &Controller{queue: q, wake: func() {}}
	defer h.close()
	canceled := 0
	h.startDialog = func(_ dialogRequest, done func(dialogResult)) (func(), error) {
		return func() { canceled++; done(dialogResult{}) }, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := h.OpenDialog(ctx, omniwindow.FileDialogOptions{}); result <- err }()
	<-q.Wake()
	h.drain()
	_, err := q.Submit(t.Context(), func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	h.drain()
	if canceled != 1 || h.dialog != nil {
		t.Fatal("cancellation leaked dialog", canceled)
	}
}

func TestDialogCloseAndStartFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "shutdown", true: "start failure"}[fail], func(t *testing.T) {
			q := dispatch.New(1)
			h := &Controller{queue: q, wake: func() {}}
			canceled := 0
			failure := errors.New("cannot create panel")
			h.startDialog = func(_ dialogRequest, done func(dialogResult)) (func(), error) {
				if fail {
					return nil, failure
				}
				return func() { canceled++; done(dialogResult{}) }, nil
			}
			result := make(chan error, 1)
			go func() { _, err := h.SaveDialog(t.Context(), omniwindow.FileDialogOptions{}); result <- err }()
			<-q.Wake()
			h.drain()
			h.close()
			h.close()
			err := <-result
			if fail {
				if !errors.Is(err, failure) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, dispatch.ErrClosed) || canceled != 1 {
				t.Fatal(err, canceled)
			}
		})
	}
}

func TestDialogCanceledBeforeDrain(t *testing.T) {
	q := dispatch.New(1)
	h := &Controller{queue: q, wake: func() {}}
	defer h.close()
	h.startDialog = func(dialogRequest, func(dialogResult)) (func(), error) {
		t.Error("opened canceled dialog")
		return nil, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { _, err := h.OpenDialog(ctx, omniwindow.FileDialogOptions{}); result <- err }()
	<-q.Wake()
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	h.drain()
	if h.dialog != nil {
		t.Fatal("canceled dialog remains active")
	}
}
