//go:build linux && (amd64 || arm64)

package nativewindow

import (
	"encoding/binary"
	"fmt"
	"image"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/herlanai/omnigo/omniwindow"
	"golang.org/x/sys/unix"
)

// XEvent is 24 native longs on both supported 64-bit ABIs. We decode only
// documented members, not compiler-dependent Go unions.
type xevent [192]byte

func (e *xevent) u32(at int) uint32 { return binary.LittleEndian.Uint32(e[at:]) }
func (e *xevent) i32(at int) int    { return int(int32(e.u32(at))) }
func (e *xevent) u64(at int) uint64 { return binary.LittleEndian.Uint64(e[at:]) }

type ximage struct {
	Width, Height, Offset, Format                                                       int32
	Data                                                                                unsafe.Pointer
	ByteOrder, BitmapUnit, BitmapBitOrder, BitmapPad, Depth, BytesPerLine, BitsPerPixel int32
	Red, Green, Blue                                                                    uintptr
	DataObject                                                                          uintptr
	Functions                                                                           [6]uintptr
}
type xapi struct {
	transient       func(uintptr, uintptr, uintptr) int32
	send            func(uintptr, uintptr, int32, int64, *xevent) int32
	property        func(uintptr, uintptr, uintptr, uintptr, int32, int32, unsafe.Pointer, int32) int32
	unmap           func(uintptr, uintptr) int32
	iconify         func(uintptr, uintptr, int32) int32
	open            func(uintptr) uintptr
	close           func(uintptr) int32
	screen          func(uintptr) int32
	root            func(uintptr, int32) uintptr
	visual          func(uintptr, int32) uintptr
	depth           func(uintptr, int32) int32
	create          func(uintptr, uintptr, int32, int32, uint32, uint32, uint32, uintptr, uintptr) uintptr
	destroy         func(uintptr, uintptr) int32
	name            func(uintptr, uintptr, string) int32
	selectInput     func(uintptr, uintptr, int64) int32
	mapWindow       func(uintptr, uintptr) int32
	intern          func(uintptr, string, int32) uintptr
	protocols       func(uintptr, uintptr, *uintptr, int32) int32
	hints           func(uintptr, uintptr, unsafe.Pointer)
	pending         func(uintptr) int32
	next            func(uintptr, *xevent) int32
	fd              func(uintptr) int32
	flush           func(uintptr) int32
	gc              func(uintptr, uintptr, uintptr, uintptr) uintptr
	freeGC          func(uintptr, uintptr) int32
	createImage     func(uintptr, uintptr, uint32, int32, int32, unsafe.Pointer, uint32, uint32, int32, int32) *ximage
	destroyImage    func(*ximage) int32
	putImage        func(uintptr, uintptr, uintptr, *ximage, int32, int32, int32, int32, uint32, uint32) int32
	malloc          func(uintptr) unsafe.Pointer
	free            func(unsafe.Pointer)
	locale          func(int32, string) uintptr
	localeModifiers func(string) uintptr
	openIM          func(uintptr, uintptr, uintptr, uintptr) uintptr
	closeIM         func(uintptr) int32
	createIC        func(uintptr, string, uintptr, string, uintptr, string, uintptr, uintptr) uintptr
	destroyIC       func(uintptr)
	focusIC         func(uintptr)
	unfocusIC       func(uintptr)
	filter          func(*xevent, uintptr) int32
	lookup          func(uintptr, *xevent, *byte, int32, *uintptr, *int32) int32
	lookupBasic     func(*xevent, *byte, int32, *uintptr, uintptr) int32
	detectRepeat    func(uintptr, int32, *int32) int32
}
type xhost struct {
	ownsConnection                                                     bool
	parent                                                             *xhost
	family                                                             map[uintptr]*xhost
	events                                                             []Event
	mapped, hiddenByParent, explicitlyHidden                           bool
	lastInputTime                                                      uint64
	pressed                                                            [256]bool
	api                                                                xapi
	lib, display, window, gc, visual, deleteAtom, protocolAtom, im, ic uintptr
	depth                                                              int32
	pipe                                                               [2]int
}

var xActive atomic.Bool

func openBackend(o omniwindow.WindowOptions) (result backend, err error) {
	if !xActive.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("nativewindow: X11 currently supports one event loop per process")
	}
	h, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		xActive.Store(false)
		return nil, fmt.Errorf("nativewindow: X11 runtime unavailable: %w", err)
	}
	w := &xhost{lib: h, pipe: [2]int{-1, -1}, ownsConnection: true, family: map[uintptr]*xhost{}}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("nativewindow: X11 binding: %v", p)
		}
		if err != nil {
			w.close()
		}
	}()
	bind := func(fn any, name string) { purego.RegisterLibFunc(fn, h, name) }
	a := &w.api
	bind(&a.open, "XOpenDisplay")
	bind(&a.close, "XCloseDisplay")
	bind(&a.screen, "XDefaultScreen")
	bind(&a.root, "XRootWindow")
	bind(&a.visual, "XDefaultVisual")
	bind(&a.depth, "XDefaultDepth")
	bind(&a.create, "XCreateSimpleWindow")
	bind(&a.destroy, "XDestroyWindow")
	bind(&a.name, "XStoreName")
	bind(&a.selectInput, "XSelectInput")
	bind(&a.mapWindow, "XMapWindow")
	bind(&a.intern, "XInternAtom")
	bind(&a.protocols, "XSetWMProtocols")
	bind(&a.hints, "XSetWMNormalHints")
	bind(&a.pending, "XPending")
	bind(&a.next, "XNextEvent")
	bind(&a.fd, "XConnectionNumber")
	bind(&a.flush, "XFlush")
	bind(&a.gc, "XCreateGC")
	bind(&a.freeGC, "XFreeGC")
	bind(&a.createImage, "XCreateImage")
	bind(&a.destroyImage, "XDestroyImage")
	bind(&a.putImage, "XPutImage")
	bind(&a.malloc, "malloc")
	bind(&a.free, "free")
	bind(&a.locale, "setlocale")
	bind(&a.localeModifiers, "XSetLocaleModifiers")
	bind(&a.openIM, "XOpenIM")
	bind(&a.closeIM, "XCloseIM")
	bind(&a.createIC, "XCreateIC")
	bind(&a.destroyIC, "XDestroyIC")
	bind(&a.focusIC, "XSetICFocus")
	bind(&a.unfocusIC, "XUnsetICFocus")
	bind(&a.filter, "XFilterEvent")
	bind(&a.lookup, "Xutf8LookupString")
	bind(&a.lookupBasic, "XLookupString")
	bind(&a.send, "XSendEvent")
	bind(&a.property, "XChangeProperty")
	bind(&a.unmap, "XUnmapWindow")
	bind(&a.iconify, "XIconifyWindow")
	bind(&a.detectRepeat, "XkbSetDetectableAutoRepeat")
	bind(&a.transient, "XSetTransientForHint")
	w.display = a.open(0)
	if w.display == 0 {
		return nil, fmt.Errorf("nativewindow: cannot open DISPLAY; X11 or XWayland is required")
	}
	return w.setup(o)
}
func (w *xhost) setup(o omniwindow.WindowOptions) (backend, error) {
	a := &w.api
	var err error
	screen := a.screen(w.display)
	var detectable int32
	a.detectRepeat(w.display, 1, &detectable)
	w.visual = a.visual(w.display, screen)
	w.depth = a.depth(w.display, screen)
	if w.depth != 24 && w.depth != 32 {
		return nil, fmt.Errorf("nativewindow: unsupported X11 depth %d", w.depth)
	}
	w.window = a.create(w.display, a.root(w.display, screen), 0, 0, uint32(o.Width), uint32(o.Height), 0, 0, 0xffffff)
	if w.window == 0 {
		return nil, fmt.Errorf("nativewindow: XCreateSimpleWindow failed")
	}
	w.family[w.window] = w
	if w.parent != nil {
		a.transient(w.display, w.window, w.parent.window)
	}
	if err = w.apply(action{kind: "title", text: o.Title}); err != nil {
		return nil, err
	}
	// Key, button, motion, exposure, structure and focus events.
	a.selectInput(w.display, w.window, 1|2|4|8|32|64|1<<15|1<<17|1<<21)
	w.deleteAtom = a.intern(w.display, "WM_DELETE_WINDOW", 0)
	w.protocolAtom = a.intern(w.display, "WM_PROTOCOLS", 0)
	a.protocols(w.display, w.window, &w.deleteAtom, 1)
	// XSizeHints: native long flags then 17 ints, padded to 80 bytes.
	var hints [80]byte
	binary.LittleEndian.PutUint64(hints[:], 1<<4)
	binary.LittleEndian.PutUint32(hints[24:], uint32(o.MinWidth))
	binary.LittleEndian.PutUint32(hints[28:], uint32(o.MinHeight))
	a.hints(w.display, w.window, unsafe.Pointer(&hints[0]))
	w.gc = a.gc(w.display, w.window, 0, 0)
	if w.gc == 0 {
		return nil, fmt.Errorf("nativewindow: XCreateGC failed")
	}
	// The environment selects the locale/input method. XIM server-owned preedit
	// is used; inline preedit callbacks and candidate positioning remain pending.
	if w.ownsConnection {
		a.locale(0, "")
		a.localeModifiers("")
	}
	w.im = a.openIM(w.display, 0, 0, 0)
	if w.im != 0 {
		w.ic = a.createIC(w.im, "inputStyle", 0x408, "clientWindow", w.window, "focusWindow", w.window, 0)
	}
	if w.ownsConnection {
		if err = unix.Pipe2(w.pipe[:], unix.O_NONBLOCK|unix.O_CLOEXEC); err != nil {
			return nil, err
		}
	}
	w.mapped = true
	if w.parent != nil && !w.parent.mapped {
		w.mapped = false
		w.hiddenByParent = true
	}
	if w.mapped {
		a.mapWindow(w.display, w.window)
	}
	a.flush(w.display)
	return w, nil
}
func (w *xhost) wake()         { _, _ = unix.Write(w.pipe[1], []byte{1}) }
func (w *xhost) pending() bool { return len(w.events) > 0 }
func (w *xhost) close() {
	if w.ownsConnection {
		defer xActive.Store(false)
	}
	delete(w.family, w.window)
	a := &w.api
	if w.ic != 0 {
		a.destroyIC(w.ic)
	}
	if w.im != 0 {
		a.closeIM(w.im)
	}
	if w.gc != 0 {
		a.freeGC(w.display, w.gc)
	}
	if w.window != 0 {
		a.destroy(w.display, w.window)
	}
	if w.display != 0 && w.ownsConnection {
		a.close(w.display)
	}
	if w.ownsConnection {
		for _, fd := range w.pipe {
			if fd >= 0 {
				_ = unix.Close(fd)
			}
		}
	}
	if w.lib != 0 && w.ownsConnection {
		_ = purego.Dlclose(w.lib)
	}
}
func (w *xhost) next(timeout int) ([]Event, error) {
	a := &w.api
	if a.pending(w.display) == 0 && len(w.events) == 0 {
		fds := []unix.PollFd{{Fd: int32(a.fd(w.display)), Events: unix.POLLIN}, {Fd: int32(w.pipe[0]), Events: unix.POLLIN}}
		if _, err := unix.Poll(fds, timeout); err != nil && err != unix.EINTR {
			return nil, err
		}
		if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return nil, fmt.Errorf("nativewindow: X11 connection closed")
		}
		var buf [256]byte
		for {
			if _, err := unix.Read(w.pipe[0], buf[:]); err != nil {
				break
			}
		}
	}
	for i := 0; i < 256 && a.pending(w.display) > 0; i++ {
		var e xevent
		a.next(w.display, &e)
		target := w.family[uintptr(e.u64(32))]
		if target == nil {
			continue
		}
		w := target
		if typ := e.u32(0); typ >= 2 && typ <= 6 {
			w.lastInputTime = e.u64(56)
		}
		if a.filter(&e, 0) != 0 {
			continue
		}
		typ := e.u32(0)
		switch typ {
		case 18:
			w.visibility(false)
		case 19:
			w.visibility(true)
		case 8:
			w.events = append(w.events, Event{Kind: "move", X: -1, Y: -1})
		case 12:
			w.events = append(w.events, Event{Kind: "paint"})
		case 22:
			w.events = append(w.events, Event{Kind: "resize", Width: e.i32(56), Height: e.i32(60)})
		case 33:
			if uintptr(e.u64(40)) == w.protocolAtom && e.u32(48) == 32 && uintptr(e.u64(56)) == w.deleteAtom {
				w.events = append(w.events, Event{Kind: "close"})
			}
		case 9:
			if w.ic != 0 {
				a.focusIC(w.ic)
			}
			w.events = append(w.events, Event{Kind: "focus"})
		case 10:
			clear(w.pressed[:])
			if w.ic != 0 {
				a.unfocusIC(w.ic)
			}
			w.events = append(w.events, Event{Kind: "blur"})
		case 6:
			w.events = append(w.events, Event{Kind: "move", X: float64(e.i32(64)), Y: float64(e.i32(68))})
		case 4, 5:
			x, y := float64(e.i32(64)), float64(e.i32(68))
			button := e.u32(84)
			if button >= 4 && button <= 7 {
				if typ == 4 {
					ev := Event{Kind: "wheel", X: x, Y: y}
					switch button {
					case 4:
						ev.DeltaY = -40
					case 5:
						ev.DeltaY = 40
					case 6:
						ev.DeltaX = -40
					case 7:
						ev.DeltaX = 40
					}
					w.events = append(w.events, ev)
				}
				continue
			}
			if button != 1 && button != 3 {
				continue
			}
			b := 0
			if button == 3 {
				b = 1
			}
			kind := "down"
			if typ == 5 {
				kind = "up"
			}
			w.events = append(w.events, Event{Kind: kind, X: x, Y: y, Button: b})
		case 2, 3:
			var symbol uintptr
			var status int32
			buf := make([]byte, 256)
			var n int32
			if w.ic != 0 && typ == 2 {
				n = a.lookup(w.ic, &e, &buf[0], int32(len(buf)), &symbol, &status)
				if status == -1 && n > 0 && n <= 1<<20 {
					buf = make([]byte, n)
					n = a.lookup(w.ic, &e, &buf[0], n, &symbol, &status)
				}
			} else {
				n = a.lookupBasic(&e, &buf[0], int32(len(buf)), &symbol, 0)
			}
			state := e.u32(80)
			code := e.u32(84)
			repeat := false
			if code < 256 {
				repeat = w.pressed[code]
				w.pressed[code] = typ == 2
			}
			key := xkey(symbol)
			if key != "" {
				w.events = append(w.events, Event{Kind: "key", Key: key, Up: typ == 3, Repeat: repeat, Shift: state&1 != 0, Command: state&4 != 0, Ctrl: state&4 != 0, Alt: state&8 != 0, Super: state&64 != 0})
			}
			if typ == 2 && state&(4|8) == 0 && n > 0 && int(n) <= len(buf) && (w.ic == 0 || status == 2 || status == 4) {
				text := string(buf[:n])
				if buf[0] >= 32 && buf[0] != 127 {
					w.events = append(w.events, Event{Kind: "text", Text: text})
				}
			}
		}
	}
	events := w.events
	w.events = nil
	return events, nil
}
func xkey(k uintptr) string {
	if k >= 'a' && k <= 'z' {
		return string(rune(k - 32))
	}
	if k >= 'A' && k <= 'Z' {
		return string(rune(k))
	}
	return map[uintptr]string{0xff08: "Backspace", 0xff09: "Tab", 0xfe20: "Tab", 0xff0d: "Enter", 0xff1b: "Escape", 32: "Space", 0xff50: "Home", 0xff51: "Left", 0xff52: "Up", 0xff53: "Right", 0xff54: "Down", 0xff55: "PageUp", 0xff56: "PageDown", 0xff57: "End", 0xffff: "Delete", 0xffc7: "F10"}[k]
}
func (w *xhost) present(frame *image.RGBA) error {
	a := &w.api
	width, height := frame.Rect.Dx(), frame.Rect.Dy()
	pixels := bgra(frame)
	data := a.malloc(uintptr(len(pixels)))
	if data == nil {
		return fmt.Errorf("nativewindow: framebuffer allocation failed")
	}
	copy(unsafe.Slice((*byte)(data), len(pixels)), pixels)
	img := a.createImage(w.display, w.visual, uint32(w.depth), 2, 0, data, uint32(width), uint32(height), 32, int32(width*4))
	if img == nil {
		a.free(data)
		return fmt.Errorf("nativewindow: XCreateImage failed")
	}
	defer a.destroyImage(img)
	if img.BitsPerPixel != 32 || img.ByteOrder != 0 || img.Red != 0xff0000 || img.Green != 0xff00 || img.Blue != 0xff {
		return fmt.Errorf("nativewindow: unsupported X11 visual pixel format")
	}
	a.putImage(w.display, w.window, w.gc, img, 0, 0, 0, 0, uint32(width), uint32(height))
	a.flush(w.display)
	runtime.KeepAlive(frame)
	return nil
}

func (w *xhost) child(o omniwindow.WindowOptions) (result backend, err error) {
	child := &xhost{api: w.api, lib: w.lib, display: w.display, pipe: w.pipe, parent: w, family: w.family}
	defer func() {
		if err != nil {
			child.close()
		}
	}()
	return child.setup(o)
}
func (w *xhost) visibility(visible bool) {
	w.mapped = visible
	for _, child := range w.family {
		if child.parent != w {
			continue
		}
		if !visible {
			if child.mapped {
				child.hiddenByParent = true
				child.mapped = false
				w.api.unmap(w.display, child.window)
			}
		} else if child.hiddenByParent && !child.explicitlyHidden {
			child.hiddenByParent = false
			child.mapped = true
			w.api.mapWindow(w.display, child.window)
		}
	}
	w.api.flush(w.display)
}
