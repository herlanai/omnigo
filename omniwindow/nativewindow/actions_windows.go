//go:build windows && (amd64 || arm64)

package nativewindow

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func (w *winHost) apply(a action) error {
	failed := func(name string) error { return fmt.Errorf("nativewindow: %s failed", name) }
	switch a.kind {
	case "title":
		title, err := windows.UTF16PtrFromString(a.text)
		if err != nil {
			return err
		}
		ok := call("SetWindowTextW", w.hwnd, uintptr(unsafe.Pointer(title)))
		runtime.KeepAlive(title)
		if ok == 0 {
			return failed("SetWindowTextW")
		}
	case "bounds":
		if w.fullscreen {
			return fmt.Errorf("nativewindow: leave fullscreen before changing bounds")
		}
		style := call("GetWindowLongPtrW", w.hwnd, ^uintptr(15))
		ex := call("GetWindowLongPtrW", w.hwnd, ^uintptr(19))
		r := winRect{R: int32(a.bounds.Width), B: int32(a.bounds.Height)}
		if call("AdjustWindowRectEx", uintptr(unsafe.Pointer(&r)), style, 0, ex) == 0 {
			return failed("AdjustWindowRectEx")
		}
		if call("SetWindowPos", w.hwnd, 0, uintptr(a.bounds.X+int(r.L)), uintptr(a.bounds.Y+int(r.T)), uintptr(r.R-r.L), uintptr(r.B-r.T), 0x14) == 0 {
			return failed("SetWindowPos")
		}
	case "show":
		w.showOwned(5)
	case "show-inactive":
		w.showOwned(8)
	case "hide":
		w.explicitlyHidden = true
		call("ShowWindow", w.hwnd, 0)
	case "minimize":
		call("ShowWindow", w.hwnd, 6)
	case "maximize":
		if err := w.fullScreen(false); err != nil {
			return err
		}
		call("ShowWindow", w.hwnd, 3)
	case "restore":
		if err := w.fullScreen(false); err != nil {
			return err
		}
		w.showOwned(9)
	case "fullscreen":
		return w.fullScreen(a.flag)
	case "focus":
		if call("SetForegroundWindow", w.hwnd) == 0 {
			return failed("SetForegroundWindow (OS focus policy may deny activation)")
		}
		call("SetFocus", w.hwnd)
	case "top":
		order := ^uintptr(1)
		if a.flag {
			order = ^uintptr(0)
		}
		if call("SetWindowPos", w.hwnd, order, 0, 0, 0, 0, 0x13) == 0 {
			return failed("SetWindowPos")
		}
	case "opacity":
		ex := call("GetWindowLongPtrW", w.hwnd, ^uintptr(19))
		if ex&0x80000 == 0 {
			call("SetWindowLongPtrW", w.hwnd, ^uintptr(19), ex|0x80000)
		}
		if call("SetLayeredWindowAttributes", w.hwnd, 0, uintptr(math.Round(a.opacity*255)), 2) == 0 {
			return failed("SetLayeredWindowAttributes")
		}
	default:
		return failed("unknown action")
	}
	return nil
}

type winPlacement struct {
	Length, Flags, Show uint32
	Min, Max            winPoint
	Normal              winRect
}

func (w *winHost) fullScreen(enable bool) error {
	if w.fullscreen == enable {
		return nil
	}
	if enable {
		placement := winPlacement{Length: uint32(unsafe.Sizeof(winPlacement{}))}
		if call("GetWindowPlacement", w.hwnd, uintptr(unsafe.Pointer(&placement))) == 0 {
			return fmt.Errorf("nativewindow: GetWindowPlacement failed")
		}
		type monitorInfo struct {
			Size          uint32
			Monitor, Work winRect
			Flags         uint32
		}
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		monitor := call("MonitorFromWindow", w.hwnd, 2)
		if monitor == 0 || call("GetMonitorInfoW", monitor, uintptr(unsafe.Pointer(&info))) == 0 {
			return fmt.Errorf("nativewindow: monitor information unavailable")
		}
		style := call("GetWindowLongPtrW", w.hwnd, ^uintptr(15))
		if call("SetWindowLongPtrW", w.hwnd, ^uintptr(15), style & ^uintptr(0x21cf0000)) == 0 {
			return fmt.Errorf("nativewindow: fullscreen style change failed")
		}
		m := info.Monitor
		if call("SetWindowPos", w.hwnd, 0, uintptr(m.L), uintptr(m.T), uintptr(m.R-m.L), uintptr(m.B-m.T), 0x34) == 0 {
			call("SetWindowLongPtrW", w.hwnd, ^uintptr(15), style)
			return fmt.Errorf("nativewindow: fullscreen placement failed")
		}
		w.restorePlacement = placement
		w.restoreStyle = style
		w.fullscreen = true
		return nil
	}
	if call("SetWindowLongPtrW", w.hwnd, ^uintptr(15), w.restoreStyle) == 0 {
		return fmt.Errorf("nativewindow: restore style failed")
	}
	if call("SetWindowPlacement", w.hwnd, uintptr(unsafe.Pointer(&w.restorePlacement))) == 0 {
		return fmt.Errorf("nativewindow: restore placement failed")
	}
	call("SetWindowPos", w.hwnd, 0, 0, 0, 0, 0, 0x37)
	w.fullscreen = false
	return nil
}

func (w *winHost) showOwned(command uintptr) {
	w.explicitlyHidden = false
	if parent, ok := winRegistry.Load(w.owner); ok && !parent.(*winHost).visible {
		w.hiddenByParent = true
		return
	}
	call("ShowWindow", w.hwnd, command)
}
func (w *winHost) visibility(visible bool) {
	if w.visible == visible {
		return
	}
	w.visible = visible
	winRegistry.Range(func(_, value any) bool {
		child := value.(*winHost)
		if child.owner != w.hwnd {
			return true
		}
		if !visible && child.visible {
			child.hiddenByParent = true
			call("ShowWindow", child.hwnd, 0)
		} else if visible && child.hiddenByParent && !child.explicitlyHidden {
			child.hiddenByParent = false
			call("ShowWindow", child.hwnd, 8)
		}
		return true
	})
}
