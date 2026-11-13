package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
	"testing"
)

func TestAppearanceValidationAndDispatch(t *testing.T) {
	q := dispatch.New(4)
	source := omniwindow.AppearanceSystem
	calls := 0
	h := &Controller{queue: q, wake: func() {}, appearance: func(s omniwindow.AppearanceSource) (omniwindow.AppearanceState, error) {
		calls++
		if s != "" {
			source = s
		}
		return omniwindow.AppearanceState{Source: source, Dark: source == omniwindow.AppearanceDark}, nil
	}}
	defer h.close()
	for _, s := range []omniwindow.AppearanceSource{"", "auto", "DARK"} {
		if _, err := h.SetAppearance(t.Context(), s); err == nil {
			t.Fatal("invalid source accepted", s)
		}
	}
	if _, err := h.Appearance(nil); err == nil {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := h.SetAppearance(ctx, omniwindow.AppearanceDark); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("invalid/canceled call touched native state")
	}
	for _, s := range []omniwindow.AppearanceSource{omniwindow.AppearanceDark, omniwindow.AppearanceLight, omniwindow.AppearanceSystem, ""} {
		result := make(chan omniwindow.AppearanceState, 1)
		go func() {
			var state omniwindow.AppearanceState
			var err error
			if s == "" {
				state, err = h.Appearance(t.Context())
			} else {
				state, err = h.SetAppearance(t.Context(), s)
			}
			if err != nil {
				t.Error(err)
			}
			result <- state
		}()
		<-q.Wake()
		q.Drain(1)
		state := <-result
		expected := s
		if expected == "" {
			expected = omniwindow.AppearanceSystem
		}
		if state.Source != expected || state.Dark != (expected == omniwindow.AppearanceDark) {
			t.Fatal(state)
		}
	}
	h.close()
	if _, err := h.Appearance(t.Context()); !errors.Is(err, dispatch.ErrClosed) {
		t.Fatal(err)
	}
}
