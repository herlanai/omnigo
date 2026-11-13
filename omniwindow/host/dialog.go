package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
	"slices"
)

var ErrDialogBusy = errors.New("host: another dialog is already open")

type dialogRequest struct {
	file    omniwindow.FileDialogOptions
	message *omniwindow.MessageDialogOptions
	save    bool
}
type dialogResult struct {
	file    omniwindow.FileDialogResult
	message omniwindow.MessageDialogResult
	err     error
}
type dialogOperation struct {
	result   chan dialogResult
	cancel   func()
	finished bool
}

func (h *Controller) OpenDialog(ctx context.Context, o omniwindow.FileDialogOptions) (omniwindow.FileDialogResult, error) {
	o, err := omniwindow.PrepareFileDialog(o, false)
	if err != nil {
		return omniwindow.FileDialogResult{}, err
	}
	r, err := h.showDialog(ctx, dialogRequest{file: o})
	return r.file, err
}
func (h *Controller) SaveDialog(ctx context.Context, o omniwindow.FileDialogOptions) (omniwindow.FileDialogResult, error) {
	o, err := omniwindow.PrepareFileDialog(o, true)
	if err != nil {
		return omniwindow.FileDialogResult{}, err
	}
	r, err := h.showDialog(ctx, dialogRequest{file: o, save: true})
	return r.file, err
}
func (h *Controller) MessageDialog(ctx context.Context, o omniwindow.MessageDialogOptions) (omniwindow.MessageDialogResult, error) {
	o, err := omniwindow.PrepareMessageDialog(o)
	if err != nil {
		return omniwindow.MessageDialogResult{}, err
	}
	r, err := h.showDialog(ctx, dialogRequest{message: &o})
	return r.message, err
}
func (h *Controller) finishDialog(op *dialogOperation, r dialogResult) {
	if op.finished {
		return
	}
	op.finished = true
	if h.dialog == op {
		h.dialog = nil
	}
	r.file.Paths = slices.Clone(r.file.Paths)
	op.result <- r
}
func (h *Controller) stopDialog(op *dialogOperation, err error) {
	if op.finished {
		return
	}
	h.finishDialog(op, dialogResult{err: err})
	if op.cancel != nil {
		op.cancel()
	}
}
func (h *Controller) closeDialog() {
	if h.dialog != nil {
		h.stopDialog(h.dialog, dispatch.ErrClosed)
	}
}
func (h *Controller) showDialog(ctx context.Context, request dialogRequest) (dialogResult, error) {
	if ctx == nil {
		return dialogResult{}, errors.New("host: nil context")
	}
	op := &dialogOperation{result: make(chan dialogResult, 1)}
	started, err := h.post(ctx, func() error {
		if op.finished {
			return ctx.Err()
		}
		if h.startDialog == nil {
			return ErrUnsupported
		}
		if h.dialog != nil {
			return ErrDialogBusy
		}
		h.dialog = op
		cancel, err := h.startDialog(request, func(r dialogResult) { h.finishDialog(op, r) })
		op.cancel = cancel
		if err != nil {
			h.finishDialog(op, dialogResult{err: err})
		}
		return nil
	})
	if err != nil {
		return dialogResult{}, err
	}
	for {
		select {
		case err := <-started:
			started = nil
			if err != nil {
				return dialogResult{}, err
			}
		case r := <-op.result:
			return r, r.err
		case <-ctx.Done():
			h.control(func() { h.stopDialog(op, ctx.Err()) })
			return dialogResult{}, ctx.Err()
		}
	}
}
