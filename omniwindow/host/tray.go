package host

import (
	"context"
	"errors"
	"github.com/herlanai/omnigo/omniwindow"
)

var ErrTrayClosed = errors.New("host: tray is closed")

type trayDriver interface {
	update(omniwindow.TrayOptions) error
	state() (omniwindow.TrayState, error)
	popup() error
	dismiss()
	close()
}

// Tray is an application-owned status item. Methods are worker-facing. Close is
// idempotent, and application shutdown closes every outstanding native item.
type Tray struct {
	host   *Controller
	driver trayDriver
	id     string
	closed bool
	done   chan struct{}
}

func (h *Controller) NewTray(ctx context.Context, o omniwindow.TrayOptions) (*Tray, error) {
	o, err := omniwindow.PrepareTray(o)
	if err != nil {
		return nil, err
	}
	// Creation must be reconciled on cancellation so a late UI result cannot leak.
	var tray *Tray
	err = h.createUI(ctx, func() error {
		if h.newTray == nil {
			return ErrUnsupported
		}
		for existing := range h.trays {
			if existing.id == o.ID {
				return errors.New("host: duplicate tray ID")
			}
		}
		driver, err := h.newTray(o)
		if err != nil {
			return err
		}
		tray = &Tray{host: h, driver: driver, id: o.ID, done: make(chan struct{})}
		if h.trays == nil {
			h.trays = make(map[*Tray]struct{})
		}
		h.trays[tray] = struct{}{}
		return nil
	}, func() {
		if tray != nil {
			tray.closeUI()
		}
	})
	if err != nil {
		return nil, err
	}
	return tray, nil
}
func (t *Tray) closeUI() {
	if t.closed {
		return
	}
	t.closed = true
	delete(t.host.trays, t)
	t.driver.close()
	close(t.done)
}
func (t *Tray) Update(ctx context.Context, o omniwindow.TrayOptions) error {
	o, err := omniwindow.PrepareTray(o)
	if err != nil {
		return err
	}
	if o.ID != t.id {
		return errors.New("host: tray ID cannot change")
	}
	return t.host.callUI(ctx, func() error {
		if t.closed {
			return ErrTrayClosed
		}
		return t.driver.update(o)
	})
}
func (t *Tray) Close(ctx context.Context) error {
	select {
	case <-t.done:
		return nil
	default:
	}
	err := t.host.callUI(ctx, func() error { t.closeUI(); return nil })
	if err != nil {
		select {
		case <-t.done:
			return nil
		default:
		}
	}
	return err
}
func (t *Tray) PopUpMenu(ctx context.Context) error {
	tracking := false
	return t.host.createUI(ctx, func() error {
		if t.closed {
			return ErrTrayClosed
		}
		tracking = true
		defer func() { tracking = false }()
		return t.driver.popup()
	}, func() {
		if tracking && !t.closed {
			t.driver.dismiss()
		}
	})
}
func (t *Tray) State(ctx context.Context) (omniwindow.TrayState, error) {
	var result omniwindow.TrayState
	err := t.host.callUI(ctx, func() error {
		if t.closed {
			return ErrTrayClosed
		}
		var err error
		result, err = t.driver.state()
		return err
	})
	if err != nil {
		return omniwindow.TrayState{}, err
	}
	return result, nil
}
