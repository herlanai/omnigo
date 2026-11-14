//go:build windows && (amd64 || arm64)

package nativewindow

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/herlanai/omnigo/omniwindow"
	"golang.org/x/sys/windows"
)

var user = windows.NewLazySystemDLL("user32.dll")
var gdi = windows.NewLazySystemDLL("gdi32.dll")
var imm = windows.NewLazySystemDLL("imm32.dll")

//go:uintptrescapes
func call(name string, args ...uintptr) uintptr {
	v, _, _ := user.NewProc(name).Call(args...)
	return v
}

//go:uintptrescapes
func immcall(name string, args ...uintptr) uintptr {
	v, _, _ := imm.NewProc(name).Call(args...)
	return v
}

type winPoint struct{ X, Y int32 }
type winRect struct{ L, T, R, B int32 }
type winMsg struct {
	Window  uintptr
	Message uint32
	W, L    uintptr
	Time    uint32
	Point   winPoint
	Private uint32
}
type winClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type paintStruct struct {
	DC              uintptr
	Erase           int32
	Rect            winRect
	Restore, Update int32
	Reserved        [32]byte
}
type winHost struct {
	owner                                     uintptr
	visible, hiddenByParent, explicitlyHidden bool
	fullscreen                                bool
	restorePlacement                          winPlacement
	restoreStyle                              uintptr
	tracking                                  bool
	hwnd                                      uintptr
	events                                    []Event
	input                                     utf16Input
	options                                   omniwindow.WindowOptions
}

var winRegistry sync.Map
var classOnce sync.Once
var classError error
var className = windows.StringToUTF16Ptr("OmniGoSoftwareWindow")
var module windows.Handle

func windowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	value, ok := winRegistry.Load(hwnd)
	if !ok {
		return call("DefWindowProcW", hwnd, uintptr(msg), wp, lp)
	}
	w := value.(*winHost)
	push := func(e Event) { w.events = append(w.events, e) }
	xy := func() (float64, float64) { return float64(int16(lp & 0xffff)), float64(int16(lp >> 16)) }
	switch msg {
	case 0x18:
		w.visibility(wp != 0)
	case 0x10:
		push(Event{Kind: "close"})
		return 0
	case 0xf:
		var p paintStruct
		call("BeginPaint", hwnd, uintptr(unsafe.Pointer(&p)))
		call("EndPaint", hwnd, uintptr(unsafe.Pointer(&p)))
		push(Event{Kind: "paint"})
		return 0
	case 0x14:
		return 1
	case 5:
		if wp == 1 {
			w.visibility(false)
		} else if !w.explicitlyHidden && call("IsWindowVisible", hwnd) != 0 {
			w.visibility(true)
		}
		if wp != 1 {
			push(Event{Kind: "resize", Width: int(lp & 0xffff), Height: int((lp >> 16) & 0xffff)})
		}
		return 0
	case 0x24:
		// MINMAXINFO contains five POINTs; client minima include the non-client frame.
		// LPARAM is a borrowed OS address, not a Go pointer. Copy through the
		// native memory API so checkptr/GC never treat it as a Go allocation.
		var p [5]winPoint
		if err := windows.ReadProcessMemory(windows.CurrentProcess(), lp, (*byte)(unsafe.Pointer(&p)), unsafe.Sizeof(p), nil); err != nil {
			break
		}
		r := winRect{R: int32(w.options.MinWidth), B: int32(w.options.MinHeight)}
		call("AdjustWindowRectEx", uintptr(unsafe.Pointer(&r)), 0xcf0000, 0, 0)
		p[3] = winPoint{r.R - r.L, r.B - r.T}
		if err := windows.WriteProcessMemory(windows.CurrentProcess(), lp, (*byte)(unsafe.Pointer(&p)), unsafe.Sizeof(p), nil); err != nil {
			break
		}
		return 0
	case 7:
		push(Event{Kind: "focus"})
		return 0
	case 8:
		w.input.high = 0
		push(Event{Kind: "blur"})
		return 0
	case 0x215:
		push(Event{Kind: "cancel"})
		return 0
	case 0x200:
		if !w.tracking {
			type track struct {
				Size, Flags uint32
				Window      uintptr
				Hover       uint32
			}
			t := track{Size: uint32(unsafe.Sizeof(track{})), Flags: 2, Window: hwnd}
			w.tracking = call("TrackMouseEvent", uintptr(unsafe.Pointer(&t))) != 0
		}
		x, y := xy()
		push(Event{Kind: "move", X: x, Y: y})
		return 0
	case 0x2a3:
		w.tracking = false
		push(Event{Kind: "move", X: -1, Y: -1})
		return 0
	case 0x201, 0x204, 0x202, 0x205:
		x, y := xy()
		button := 0
		if msg == 0x204 || msg == 0x205 {
			button = 1
		}
		kind := "down"
		if msg == 0x202 || msg == 0x205 {
			kind = "up"
		} else {
			call("SetFocus", hwnd)
			call("SetCapture", hwnd)
		}
		push(Event{Kind: kind, X: x, Y: y, Button: button})
		if kind == "up" {
			// ReleaseCapture synchronously emits WM_CAPTURECHANGED; enqueue
			// the release before cancellation so a completed click activates.
			call("ReleaseCapture")
		}
		return 0
	case 0x20a, 0x20e:
		x, y := xy()
		p := winPoint{int32(x), int32(y)}
		call("ScreenToClient", hwnd, uintptr(unsafe.Pointer(&p)))
		e := Event{Kind: "wheel", X: float64(p.X), Y: float64(p.Y)}
		d := float64(int16(wp>>16)) / 120 * 40
		if msg == 0x20a {
			e.DeltaY = -d
		} else {
			e.DeltaX = d
		}
		push(e)
		return 0
	case 0x100, 0x101, 0x104, 0x105:
		if (msg == 0x104 || msg == 0x105) && wp != 121 {
			break
		}
		key := winKey(uint32(wp))
		if key != "" {
			push(Event{Kind: "key", Key: key, Up: msg == 0x101 || msg == 0x105, Repeat: lp&(1<<30) != 0, Shift: call("GetKeyState", 0x10)&0x8000 != 0, Command: call("GetKeyState", 0x11)&0x8000 != 0, Ctrl: call("GetKeyState", 0x11)&0x8000 != 0, Alt: call("GetKeyState", 0x12)&0x8000 != 0, Super: call("GetKeyState", 0x5b)&0x8000 != 0 || call("GetKeyState", 0x5c)&0x8000 != 0})
		}
		return 0
	case 0x102:
		if text := w.input.feed(uint16(wp)); text != "" {
			push(Event{Kind: "text", Text: text})
		}
		return 0
	case 0x10f:
		ic := immcall("ImmGetContext", hwnd)
		if ic == 0 {
			break
		}
		defer immcall("ImmReleaseContext", hwnd, ic)
		read := func(flag uintptr) string {
			n := int32(immcall("ImmGetCompositionStringW", ic, flag, 0, 0))
			if n <= 0 || n > 1<<20 {
				return ""
			}
			buf := make([]uint16, (int(n)+1)/2)
			got := int32(immcall("ImmGetCompositionStringW", ic, flag, uintptr(unsafe.Pointer(&buf[0])), uintptr(n)))
			if got < 0 || got > n {
				return ""
			}
			return string(utf16.Decode(buf[:got/2]))
		}
		if lp&0x800 != 0 {
			push(Event{Kind: "composition"})
			push(Event{Kind: "text", Text: read(0x800)})
		}
		if lp&8 != 0 {
			push(Event{Kind: "composition", Text: read(8)})
		}
		return 0 // result already consumed; prevent duplicate WM_CHAR from DefWindowProc.
	case 0x10e:
		push(Event{Kind: "composition"})
		return 0
	}
	return call("DefWindowProcW", hwnd, uintptr(msg), wp, lp)
}
func winKey(v uint32) string {
	if v >= 'A' && v <= 'Z' {
		return string(rune(v))
	}
	return map[uint32]string{8: "Backspace", 9: "Tab", 13: "Enter", 27: "Escape", 32: "Space", 33: "PageUp", 34: "PageDown", 35: "End", 36: "Home", 37: "Left", 38: "Up", 39: "Right", 40: "Down", 46: "Delete", 121: "F10"}[v]
}
func openBackend(o omniwindow.WindowOptions) (backend, error) {
	return openOwnedWindow(o, 0)
}
func (w *winHost) child(o omniwindow.WindowOptions) (backend, error) {
	return openOwnedWindow(o, w.hwnd)
}
func openOwnedWindow(o omniwindow.WindowOptions, owner uintptr) (backend, error) {
	classOnce.Do(func() {
		if err := windows.GetModuleHandleEx(2, nil, &module); err != nil {
			classError = err
			return
		}
		c := winClass{Size: uint32(unsafe.Sizeof(winClass{})), Proc: windows.NewCallback(windowProc), Instance: uintptr(module), Name: className, Cursor: call("LoadCursorW", 0, 32512)}
		if call("RegisterClassExW", uintptr(unsafe.Pointer(&c))) == 0 {
			classError = fmt.Errorf("nativewindow: RegisterClassExW failed")
		}
	})
	if classError != nil {
		return nil, classError
	}
	title, err := windows.UTF16PtrFromString(o.Title)
	if err != nil {
		return nil, err
	}
	r := winRect{R: int32(o.Width), B: int32(o.Height)}
	call("AdjustWindowRectEx", uintptr(unsafe.Pointer(&r)), 0xcf0000, 0, 0)
	hwnd := call("CreateWindowExW", 0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0xcf0000, 0x80000000, 0x80000000, uintptr(r.R-r.L), uintptr(r.B-r.T), owner, 0, uintptr(module), 0)
	runtime.KeepAlive(title)
	if hwnd == 0 {
		return nil, fmt.Errorf("nativewindow: CreateWindowExW failed")
	}
	w := &winHost{hwnd: hwnd, options: o, owner: owner}
	winRegistry.Store(hwnd, w)
	w.showOwned(5)
	call("UpdateWindow", hwnd)
	return w, nil
}
func (w *winHost) wake()         { call("PostMessageW", w.hwnd, 0x8001, 0, 0) }
func (w *winHost) pending() bool { return len(w.events) > 0 }
func (w *winHost) close()        { winRegistry.Delete(w.hwnd); call("DestroyWindow", w.hwnd) }
func (w *winHost) next(timeout int) ([]Event, error) {
	if len(w.events) == 0 {
		ms := uintptr(0xffffffff)
		if timeout >= 0 {
			ms = uintptr(timeout)
		}
		if call("MsgWaitForMultipleObjectsEx", 0, 0, ms, 0x4ff, 4) == 0xffffffff {
			return nil, fmt.Errorf("nativewindow: message wait failed")
		}
	}
	var msg winMsg
	for i := 0; i < 256 && call("PeekMessageW", uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1) != 0; i++ {
		if msg.Message == 0x12 {
			w.events = append(w.events, Event{Kind: "close"})
			break
		}
		call("TranslateMessage", uintptr(unsafe.Pointer(&msg)))
		call("DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
	}
	events := w.events
	w.events = nil
	return events, nil
}
func (w *winHost) present(img *image.RGBA) error {
	pixels := bgra(img)
	type info struct {
		Size                   uint32
		W, H                   int32
		Planes, Bits           uint16
		Compression, SizeImage uint32
		X, Y                   int32
		Used, Important        uint32
	}
	header := info{Size: 40, W: int32(img.Rect.Dx()), H: -int32(img.Rect.Dy()), Planes: 1, Bits: 32}
	dc := call("GetDC", w.hwnd)
	if dc == 0 {
		return fmt.Errorf("nativewindow: GetDC failed")
	}
	defer call("ReleaseDC", w.hwnd, dc)
	result, _, _ := gdi.NewProc("StretchDIBits").Call(dc, 0, 0, uintptr(img.Rect.Dx()), uintptr(img.Rect.Dy()), 0, 0, uintptr(img.Rect.Dx()), uintptr(img.Rect.Dy()), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0, 0xcc0020)
	runtime.KeepAlive(pixels)
	if int32(result) <= 0 {
		return fmt.Errorf("nativewindow: StretchDIBits failed")
	}
	return nil
}

// HWND returns the Win32 handle during UI callbacks only. Never retain it after Closing.
func (w *Window) HWND() uintptr {
	if driver, ok := w.driver.(*winHost); ok {
		return driver.hwnd
	}
	return 0
}
