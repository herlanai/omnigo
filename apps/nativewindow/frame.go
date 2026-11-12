package nativewindow

import (
	"errors"
	"time"
)

// frameState is shared by root and owned windows. The owning event loop supplies
// the clock; scheduling one window never advances another window's deadline.
type frameState struct {
	width, height int
	dirty         bool
	due           time.Time
}

func (w *Window) drainFrame() {
	drained := w.queue.Drain(64)
	if drained > 0 {
		w.frame.dirty = true
	}
	if drained == 64 {
		w.driver.wake()
	}
}

// handleEvent leaves destruction to the owner of the window tree.
func (w *Window) handleEvent(e Event) (closeWindow bool, err error) {
	w.frame.dirty = true
	if e.Kind == "close" {
		return w.options.ShouldClose == nil || w.options.ShouldClose(), nil
	}
	if e.Kind == "resize" {
		w.frame.width, w.frame.height = e.Width, e.Height
	}
	if w.options.Event != nil {
		err = w.options.Event(e)
	}
	return false, err
}

func (w *Window) renderFrame(now time.Time) error {
	f := &w.frame
	if !f.due.IsZero() && !now.Before(f.due) {
		f.due = time.Time{}
		if w.options.Tick != nil {
			if err := w.options.Tick(); err != nil {
				return err
			}
		}
		f.dirty = true
	}
	if f.dirty && w.options.Paint != nil && validSize(f.width, f.height) {
		frame, err := w.options.Paint(f.width, f.height)
		if err != nil {
			return err
		}
		if frame == nil || frame.Bounds().Dx() != f.width || frame.Bounds().Dy() != f.height {
			return errors.New("nativewindow: framebuffer size mismatch")
		}
		if err = w.driver.present(frame); err != nil {
			return err
		}
	}
	f.dirty = false
	return nil
}

func (w *Window) frameTimeout(now time.Time) int {
	if w.options.NextFrame == nil {
		w.frame.due = time.Time{}
		return -1
	}
	delay, ok := w.options.NextFrame()
	if !ok {
		w.frame.due = time.Time{}
		return -1
	}
	due := now.Add(max(time.Millisecond, delay))
	if w.frame.due.IsZero() || due.Before(w.frame.due) {
		w.frame.due = due
	}
	return int(max(time.Millisecond, w.frame.due.Sub(now)).Milliseconds())
}
