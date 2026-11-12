//go:build linux && (amd64 || arm64)

package nativewindow

import (
	"encoding/binary"
	"fmt"
	"math"
	"runtime"
	"unsafe"
)

func (w *xhost) atom(name string) uintptr { return w.api.intern(w.display, name, 0) }

// EWMH messages ask the window manager to apply a state. Success only means the
// request was sent; a window manager may reject it or lack the relevant feature.
func (w *xhost) message(name string, data ...uint64) error {
	var e xevent
	binary.LittleEndian.PutUint32(e[:], 33)
	binary.LittleEndian.PutUint64(e[32:], uint64(w.window))
	binary.LittleEndian.PutUint64(e[40:], uint64(w.atom(name)))
	binary.LittleEndian.PutUint32(e[48:], 32)
	for i, v := range data {
		if i < 5 {
			binary.LittleEndian.PutUint64(e[56+i*8:], v)
		}
	}
	root := w.api.root(w.display, w.api.screen(w.display))
	if w.api.send(w.display, root, 0, (1<<20)|(1<<19), &e) == 0 {
		return fmt.Errorf("nativewindow: XSendEvent failed")
	}
	return nil
}
func (w *xhost) apply(a action) error {
	api := &w.api
	var err error
	switch a.kind {
	case "title":
		api.name(w.display, w.window, a.text)
		bytes := append([]byte(a.text), 0)
		api.property(w.display, w.window, w.atom("_NET_WM_NAME"), w.atom("UTF8_STRING"), 8, 0, unsafe.Pointer(&bytes[0]), int32(len(bytes)-1))
		runtime.KeepAlive(bytes)
	case "bounds":
		// EWMH uses client coordinates with StaticGravity and all four fields valid.
		err = w.message("_NET_MOVERESIZE_WINDOW", 10|0xf00|0x1000, uint64(int64(a.bounds.X)), uint64(int64(a.bounds.Y)), uint64(a.bounds.Width), uint64(a.bounds.Height))
	case "show":
		if w.showOwned() {
			err = w.message("_NET_ACTIVE_WINDOW", 1, w.lastInputTime, 0)
		}
	case "show-inactive":
		w.showOwned()
	case "hide":
		w.explicitlyHidden = true
		w.visibility(false)
		api.unmap(w.display, w.window)
	case "focus":
		err = w.message("_NET_ACTIVE_WINDOW", 1, w.lastInputTime, 0)
	case "minimize":
		if api.iconify(w.display, w.window, api.screen(w.display)) == 0 {
			return fmt.Errorf("nativewindow: XIconifyWindow failed")
		}
	case "maximize":
		err = w.message("_NET_WM_STATE", 1, uint64(w.atom("_NET_WM_STATE_MAXIMIZED_VERT")), uint64(w.atom("_NET_WM_STATE_MAXIMIZED_HORZ")), 1)
	case "restore":
		w.showOwned()
		if err = w.message("_NET_WM_STATE", 0, uint64(w.atom("_NET_WM_STATE_FULLSCREEN")), 0, 1); err != nil {
			return err
		}
		err = w.message("_NET_WM_STATE", 0, uint64(w.atom("_NET_WM_STATE_MAXIMIZED_VERT")), uint64(w.atom("_NET_WM_STATE_MAXIMIZED_HORZ")), 1)
	case "top":
		value := uint64(0)
		if a.flag {
			value = 1
		}
		err = w.message("_NET_WM_STATE", value, uint64(w.atom("_NET_WM_STATE_ABOVE")), 0, 1)
	case "fullscreen":
		value := uint64(0)
		if a.flag {
			value = 1
		}
		err = w.message("_NET_WM_STATE", value, uint64(w.atom("_NET_WM_STATE_FULLSCREEN")), 0, 1)
	case "opacity":
		value := uintptr(math.Round(a.opacity * float64(uint32(0xffffffff))))
		api.property(w.display, w.window, w.atom("_NET_WM_WINDOW_OPACITY"), 6, 32, 0, unsafe.Pointer(&value), 1)
	default:
		return fmt.Errorf("nativewindow: unknown action")
	}
	api.flush(w.display)
	return err
}

func (w *xhost) showOwned() bool {
	w.explicitlyHidden = false
	if w.parent != nil && !w.parent.mapped {
		w.hiddenByParent = true
		return false
	}
	w.api.mapWindow(w.display, w.window)
	return true
}
