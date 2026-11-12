package host

import (
	"context"
	"errors"

	"github.com/herlanai/omnigo/omniwindow"
)

type TitleBarRequest struct {
	Style                omniwindow.TitleBarStyle `json:"style"`
	TrafficLightPosition *omniwindow.Point        `json:"trafficLightPosition,omitempty"`
}
type TitleBarState struct {
	Style    omniwindow.TitleBarStyle `json:"style"`
	TopInset float64            `json:"topInset"`
}

func (h *Controller) TitleBarState(ctx context.Context) (TitleBarState, error) {
	return h.titleBar(ctx, TitleBarRequest{})
}
func (h *Controller) SetTitleBar(ctx context.Context, request TitleBarRequest) (TitleBarState, error) {
	if request.Style == "" {
		return TitleBarState{}, errors.New("host: title bar style required")
	}
	return h.titleBar(ctx, request)
}
func (h *Controller) titleBar(ctx context.Context, request TitleBarRequest) (TitleBarState, error) {
	if ctx == nil {
		return TitleBarState{}, errors.New("host: nil context")
	}
	options := omniwindow.WindowOptions{TitleBarStyle: request.Style, TrafficLightPosition: request.TrafficLightPosition}
	if err := options.Validate(); err != nil {
		return TitleBarState{}, err
	}
	if request.TrafficLightPosition != nil {
		p := *request.TrafficLightPosition
		request.TrafficLightPosition = &p
	}
	var result TitleBarState
	done, err := h.post(ctx, func() error {
		if h.titlebar == nil {
			return ErrUnsupported
		}
		var e error
		result, e = h.titlebar(request)
		return e
	})
	if err != nil {
		return result, err
	}
	select {
	case err = <-done:
		if err != nil {
			return TitleBarState{}, err
		}
		return result, nil
	case <-ctx.Done():
		// Do not read result while an already-started UI callback may write it.
		return TitleBarState{}, ctx.Err()
	}
}
