//go:build darwin && arm64

package native

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"log"
	"runtime"
	"structs"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	ui "omnigo.dev/omnigo/omniui/baseui"
	"omnigo.dev/omnigo/runtime/dispatch"
	"omnigo.dev/omnigo/studio/internal/demos"
)

const Available = true
const width, height = 720, 680

// AppKit must be entered on the process main thread.
func init() { runtime.LockOSThread() }

type point struct {
	_    structs.HostLayout
	X, Y float64
}
type size struct {
	_    structs.HostLayout
	W, H float64
}
type rect struct {
	_ structs.HostLayout
	P point
	S size
}
type window struct {
	native, view objc.ID
	surface      *ui.Surface
	demo         *demos.Demo
	start        time.Time
	pending      bool
}

var windows = map[objc.ID]*window{}
var byName = map[string]*window{}

func sel(v string) objc.SEL { return objc.RegisterName(v) }
func stringID(v string) objc.ID {
	return objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), v)
}
func guard(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("Studio native callback: %v", p)
		}
	}()
	fn()
}
func Run(ctx context.Context, url string, quit func(), ready func(*Host)) error {
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return err
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), 0)
	h := &Host{queue: dispatch.New(128), ctx: ctx}
	defer h.queue.Close()
	controllerClass, err := objc.RegisterClass("OmniStudioDispatcher", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: sel("drain:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { guard(func() { h.queue.Drain(64) }) }}})
	if err != nil {
		return err
	}
	controller := objc.ID(controllerClass).Send(sel("new"))
	defer controller.Send(sel("release"))
	h.wake = func() {
		controller.Send(sel("performSelectorOnMainThread:withObject:waitUntilDone:"), sel("drain:"), objc.ID(0), false)
	}
	repaint := func(w *window) {
		if err := paint(w, h); err != nil {
			log.Printf("Studio paint: %v", err)
		}
	}
	pointer := func(kind string) func(objc.ID, objc.SEL, objc.ID) {
		return func(id objc.ID, _ objc.SEL, event objc.ID) {
			guard(func() {
				w := windows[id]
				if w == nil {
					return
				}
				p := objc.Send[point](event, sel("locationInWindow"))
				if err := w.surface.Pointer(ui.PointerEvent{X: p.X, Y: height - p.Y, Kind: kind}); err != nil {
					log.Print(err)
				}
				repaint(w)
			})
		}
	}
	keyboard := func(up bool) func(objc.ID, objc.SEL, objc.ID) {
		return func(id objc.ID, _ objc.SEL, event objc.ID) {
			guard(func() {
				w := windows[id]
				if w == nil {
					return
				}
				code := uint16(event.Send(sel("keyCode")))
				flags := uint64(event.Send(sel("modifierFlags")))
				key := map[uint16]string{36: "Enter", 48: "Tab", 49: "Space", 51: "Backspace", 53: "Escape", 115: "Home", 116: "PageUp", 117: "Delete", 119: "End", 121: "PageDown", 123: "Left", 124: "Right", 125: "Down", 126: "Up"}[code]
				cmd := flags&(1<<20) != 0
				if key == "" && cmd {
					key = map[uint16]string{0: "a", 6: "z"}[code]
				}
				if key != "" {
					if err := w.surface.Key(ui.KeyEvent{Key: key, Up: up, Shift: flags&(1<<17) != 0, Command: cmd, Repeat: event.Send(sel("isARepeat")) != 0}); err != nil {
						log.Print(err)
					}
				}
				if !up && !cmd && (key == "" || key == "Space") {
					chars := event.Send(sel("characters"))
					text := objc.Send[string](chars, sel("UTF8String"))
					if err := w.surface.TextInput(text); err != nil {
						log.Print(err)
					}
				}
				repaint(w)
			})
		}
	}
	viewClass, err := objc.RegisterClass("OmniStudioCanvas", objc.GetClass("NSImageView"), nil, nil, []objc.MethodDef{
		{Cmd: sel("acceptsFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("mouseDown:"), Fn: pointer("down")}, {Cmd: sel("mouseDragged:"), Fn: pointer("move")}, {Cmd: sel("mouseUp:"), Fn: pointer("up")},
		{Cmd: sel("keyDown:"), Fn: keyboard(false)}, {Cmd: sel("keyUp:"), Fn: keyboard(true)},
	})
	if err != nil {
		return err
	}
	delegateClass, err := objc.RegisterClass("OmniStudioWindowDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("windowWillClose:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			guard(func() {
				native := n.Send(sel("object"))
				for id, w := range windows {
					if w.native == native {
						_ = w.surface.Close()
						delete(windows, id)
						for name, entry := range byName {
							if entry == w {
								delete(byName, name)
							}
						}
						w.native.Send(sel("autorelease"))
						break
					}
				}
			})
		}},
		{Cmd: sel("windowDidResignKey:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			guard(func() {
				for _, w := range windows {
					if w.native == n.Send(sel("object")) {
						_ = w.surface.Pointer(ui.PointerEvent{Kind: "cancel"})
					}
				}
			})
		}},
		{Cmd: sel("windowDidDeminiaturize:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			guard(func() {
				for _, w := range windows {
					if w.native == n.Send(sel("object")) {
						repaint(w)
					}
				}
			})
		}},
	})
	if err != nil {
		return err
	}
	delegate := objc.ID(delegateClass).Send(sel("new"))
	defer delegate.Send(sel("release"))
	h.open = func(id string, dark bool) error {
		if !demos.Supported(id) {
			return fmt.Errorf("no executable Go demo: %s", id)
		}
		name := fmt.Sprintf("%s/%t", id, dark)
		if w := byName[name]; w != nil {
			w.native.Send(sel("deminiaturize:"), objc.ID(0))
			w.native.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
			app.Send(sel("activateIgnoringOtherApps:"), true)
			return nil
		}
		if len(windows) >= 12 {
			return fmt.Errorf("close an existing demo before opening more (limit 12)")
		}
		surface, err := ui.NewSurface(width, height, ui.NewTheme(dark))
		if err != nil {
			return err
		}
		w := &window{surface: surface, demo: demos.New(id), start: time.Now()}
		w.native = objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: width, H: height}}, uint64(1|2|4), uint64(2), false)
		w.native.Send(sel("setReleasedWhenClosed:"), false)
		w.native.Send(sel("setTitle:"), stringID("Omni Studio — "+id))
		w.native.Send(sel("setDelegate:"), delegate)
		w.view = objc.ID(viewClass).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: width, H: height}})
		w.view.Send(sel("setImageScaling:"), uint64(2))
		w.native.Send(sel("setContentView:"), w.view)
		w.view.Send(sel("release"))
		windows[w.view] = w
		byName[name] = w
		if err := paint(w, h); err != nil {
			w.native.Send(sel("close"))
			return err
		}
		w.native.Send(sel("center"))
		w.native.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
		w.native.Send(sel("makeFirstResponder:"), w.view)
		app.Send(sel("activateIgnoringOtherApps:"), true)
		// Start animated demos after they are visible.
		if surface.NeedsAnimation() {
			repaint(w)
		}
		return nil
	}
	ready(h)
	closeShell, err := openShell(app, url, quit)
	if err != nil {
		return err
	}
	defer closeShell()
	go func() {
		<-ctx.Done()
		done, err := h.queue.Submit(context.Background(), func() error {
			app.Send(sel("stop:"), objc.ID(0))
			event := objc.ID(objc.GetClass("NSEvent")).Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"), uint64(15), point{}, uint64(0), float64(0), int64(0), objc.ID(0), int16(0), int64(0), int64(0))
			app.Send(sel("postEvent:atStart:"), event, true)
			return nil
		})
		if err == nil {
			h.wake()
			<-done
		}
	}()
	app.Send(sel("run"))
	for _, w := range windows {
		w.native.Send(sel("close"))
	}
	return nil
}
func paint(w *window, h *Host) error {
	if err := w.surface.Mount(w.demo.View()); err != nil {
		return err
	}
	w.surface.Advance(time.Since(w.start))
	img, err := w.surface.Render()
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return err
	}
	data := buffer.Bytes()
	// NSData copies the bytes before returning: AppKit never retains Go memory.
	nsdata := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&data[0]), uint64(len(data)))
	runtime.KeepAlive(data)
	image := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), nsdata)
	if image == 0 {
		return fmt.Errorf("AppKit rejected rendered image")
	}
	w.view.Send(sel("setImage:"), image)
	image.Send(sel("release"))
	if !w.pending && w.surface.NeedsAnimation() && w.native.Send(sel("isVisible")) != 0 && w.native.Send(sel("isMiniaturized")) == 0 {
		w.pending = true
		time.AfterFunc(time.Second/30, func() {
			_, err := h.queue.Submit(h.ctx, func() error {
				w.pending = false
				if windows[w.view] != w || w.native.Send(sel("isVisible")) == 0 || w.native.Send(sel("isMiniaturized")) != 0 {
					return nil
				}
				return paint(w, h)
			})
			if err == nil {
				h.wake()
			}
		})
	}
	return nil
}
