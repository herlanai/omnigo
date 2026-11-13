package host

import (
	"context"
	"errors"
	"testing"

	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
)

func TestScreenCoordinates(t *testing.T) {
	for _, tt := range []struct {
		name            string
		x, y, w, h, top float64
		want            omniwindow.ScreenRect
	}{
		{"primary", 0, 0, 1440, 900, 900, omniwindow.ScreenRect{Width: 1440, Height: 900}},
		{"left", -1920, -180, 1920, 1080, 900, omniwindow.ScreenRect{X: -1920, Width: 1920, Height: 1080}},
		{"above", 0, 900, 1440, 900, 900, omniwindow.ScreenRect{Y: -900, Width: 1440, Height: 900}},
		{"dock and menu", 0, 70, 1440, 805, 900, omniwindow.ScreenRect{Y: 25, Width: 1440, Height: 805}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := screenRect(tt.x, tt.y, tt.w, tt.h, tt.top); got != tt.want {
				t.Fatalf("%+v != %+v", got, tt.want)
			}
		})
	}
}

func TestScreenDispatchSnapshot(t *testing.T) {
	q := dispatch.New(2)
	source := []omniwindow.Display{{ID: 42, Primary: true, ScaleFactor: 2}}
	h := &Controller{queue: q, wake: func() {}, screen: func() (omniwindow.ScreenState, error) { return omniwindow.ScreenState{Displays: source}, nil }}
	defer h.close()
	result := make(chan omniwindow.ScreenState, 1)
	go func() {
		s, err := h.Screen(t.Context())
		if err != nil {
			t.Error(err)
		}
		result <- s
	}()
	<-q.Wake()
	q.Drain(1)
	s := <-result
	if len(s.Displays) != 1 || s.Displays[0].ID != 42 {
		t.Fatal(s)
	}
	s.Displays[0].ID = 7
	if source[0].ID != 42 {
		t.Fatal("snapshot aliases host state")
	}
	if _, err := h.Screen(nil); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestScreenCancellationDuringQuery(t *testing.T) {
	q := dispatch.New(2)
	entered, release, drained := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h := &Controller{queue: q, wake: func() {}, screen: func() (omniwindow.ScreenState, error) {
		close(entered)
		<-release
		return omniwindow.ScreenState{Displays: []omniwindow.Display{{ID: 1}}}, nil
	}}
	defer h.close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		s, err := h.Screen(ctx)
		if len(s.Displays) != 0 {
			t.Error("canceled query returned data")
		}
		result <- err
	}()
	<-q.Wake()
	go func() { q.Drain(1); close(drained) }()
	<-entered
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	<-drained
}

func TestScreenUnavailableAndShutdown(t *testing.T) {
	q := dispatch.New(2)
	h := &Controller{queue: q, wake: func() {}}
	result := make(chan error, 1)
	go func() { _, err := h.Screen(t.Context()); result <- err }()
	<-q.Wake()
	q.Drain(1)
	if err := <-result; !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	h.close()
	if _, err := h.Screen(t.Context()); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatal(err)
	}
}
