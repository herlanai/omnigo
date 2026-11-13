package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/omniwindow"
)

// Appearance reads native appearance and accessibility preferences. Worker-only.
func (h *Controller) Appearance(ctx context.Context) (omniwindow.AppearanceState, error) {
	return h.appearanceState(ctx, "")
}

// SetAppearance changes only this application's native appearance. System
// clears the override. Worker-only; cancellation cannot undo an applied change.
func (h *Controller) SetAppearance(ctx context.Context, source omniwindow.AppearanceSource) (omniwindow.AppearanceState, error) {
	if source != omniwindow.AppearanceSystem && source != omniwindow.AppearanceLight && source != omniwindow.AppearanceDark {
		return omniwindow.AppearanceState{}, errors.New("host: appearance source must be system, light or dark")
	}
	return h.appearanceState(ctx, source)
}

func (h *Controller) appearanceState(ctx context.Context, source omniwindow.AppearanceSource) (omniwindow.AppearanceState, error) {
	if ctx == nil {
		return omniwindow.AppearanceState{}, errors.New("host: nil context")
	}
	var result omniwindow.AppearanceState
	done, err := h.post(ctx, func() error {
		if h.appearance == nil {
			return ErrUnsupported
		}
		var e error
		result, e = h.appearance(source)
		return e
	})
	if err != nil {
		return omniwindow.AppearanceState{}, err
	}
	select {
	case err := <-done:
		if err != nil {
			return omniwindow.AppearanceState{}, err
		}
		return result, nil
	case <-ctx.Done():
		return omniwindow.AppearanceState{}, ctx.Err()
	}
}
