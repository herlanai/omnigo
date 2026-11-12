//go:build darwin && arm64

package native

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"structs"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	ui "omnigo.dev/omnigo/omniui/baseui"
	"omnigo.dev/omnigo/runtime/dispatch"
	"omnigo.dev/omnigo/studio"
	"omnigo.dev/omnigo/studio/internal/demos"
)

const Available = true
const width, height = 1040, 840

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
	web          *embeddedWeb
	native, view objc.ID
	surface      *ui.Surface
	demo         *demos.Demo
	start        time.Time
	timer        *time.Timer
	frameToken   uint64
}

var frames frameRenderer

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
	// Direct executable launches do not reliably refresh LaunchServices/Dock icons.
	// Set the icon from embedded bytes before any window is activated.
	iconBytes := studio.ApplicationIcon()
	iconData := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&iconBytes[0]), uint64(len(iconBytes)))
	appIcon := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), iconData)
	if appIcon == 0 {
		return fmt.Errorf("Studio: application icon could not be decoded")
	}
	app.Send(sel("setApplicationIconImage:"), appIcon)
	appIcon.Send(sel("release"))
	h := &Host{queue: dispatch.New(128), ctx: ctx}
	controllerClass, err := objc.RegisterClass("OmniStudioDispatcher", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: sel("drain:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { guard(func() { h.queue.Drain(64) }) }}})
	if err != nil {
		return err
	}
	controller := objc.ID(controllerClass).Send(sel("new"))
	defer controller.Send(sel("release"))
	defer h.close()
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
				w.surface.Advance(time.Since(w.start))
				p := objc.Send[point](event, sel("locationInWindow"))
				if err := w.surface.Pointer(ui.PointerEvent{X: p.X, Y: height - p.Y, Kind: kind, Button: int(event.Send(sel("buttonNumber")))}); err != nil {
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
				w.surface.Advance(time.Since(w.start))
				code := uint16(event.Send(sel("keyCode")))
				flags := uint64(event.Send(sel("modifierFlags")))
				key := map[uint16]string{36: "Enter", 48: "Tab", 49: "Space", 51: "Backspace", 53: "Escape", 115: "Home", 116: "PageUp", 117: "Delete", 119: "End", 121: "PageDown", 123: "Left", 124: "Right", 125: "Down", 126: "Up", 109: "F10"}[code]
				cmd := flags&(1<<20) != 0
				if key == "" && cmd {
					key = map[uint16]string{0: "A", 6: "Z", 8: "C", 7: "X", 9: "V"}[code]
				}
				if !up && cmd && (key == "C" || key == "X" || key == "V") {
					pasteboard := objc.ID(objc.GetClass("NSPasteboard")).Send(sel("generalPasteboard"))
					textType := stringID("public.utf8-plain-text")
					if key == "V" {
						value := pasteboard.Send(sel("stringForType:"), textType)
						if value != 0 {
							if err := w.surface.Paste(objc.Send[string](value, sel("UTF8String"))); err != nil {
								log.Print(err)
							}
						}
					} else if text, ok := w.surface.CopySelection(); ok {
						pasteboard.Send(sel("clearContents"))
						if pasteboard.Send(sel("setString:forType:"), stringID(text), textType) != 0 && key == "X" {
							if _, _, err := w.surface.CutSelection(); err != nil {
								log.Print(err)
							}
						}
					}
					repaint(w)
					return
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
	wheel := func(id objc.ID, _ objc.SEL, event objc.ID) {
		guard(func() {
			w := windows[id]
			if w == nil {
				return
			}
			w.surface.Advance(time.Since(w.start))
			p := objc.Send[point](event, sel("locationInWindow"))
			factor := 1.
			if event.Send(sel("hasPreciseScrollingDeltas")) == 0 {
				factor = 16
			}
			err := w.surface.Wheel(ui.WheelEvent{X: p.X, Y: height - p.Y, DeltaX: -objc.Send[float64](event, sel("scrollingDeltaX")) * factor, DeltaY: -objc.Send[float64](event, sel("scrollingDeltaY")) * factor})
			if err != nil {
				log.Print(err)
			}
			repaint(w)
		})
	}
	viewClass, err := objc.RegisterClass("OmniStudioCanvas", objc.GetClass("NSImageView"), nil, nil, []objc.MethodDef{
		{Cmd: sel("acceptsFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("scrollWheel:"), Fn: wheel},
		{Cmd: sel("mouseMoved:"), Fn: pointer("move")},
		{Cmd: sel("mouseExited:"), Fn: pointer("leave")},
		{Cmd: sel("rightMouseDown:"), Fn: pointer("down")},
		{Cmd: sel("rightMouseUp:"), Fn: pointer("up")},
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
						if w.timer != nil {
							w.timer.Stop()
							w.timer = nil
						}
						w.frameToken++
						w.demo.Close()
						_ = w.surface.Close()
						if w.web != nil {
							w.web.Close()
						}
						delete(windows, id)
						if len(windows) == 0 {
							frames.reset()
						}
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
						_ = w.surface.Advance(time.Since(w.start))
						w.surface.SetActive(false)
						_ = w.surface.Pointer(ui.PointerEvent{Kind: "cancel"})
					}
				}
			})
		}},
		{Cmd: sel("windowDidBecomeKey:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			guard(func() {
				for _, w := range windows {
					if w.native == n.Send(sel("object")) {
						_ = w.surface.Advance(time.Since(w.start))
						w.surface.SetActive(true)
						repaint(w)
					}
				}
			})
		}},
		{Cmd: sel("windowDidChangeBackingProperties:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			guard(func() {
				for _, w := range windows {
					if w.native == n.Send(sel("object")) {
						repaint(w)
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
		if !demos.Supported(id) && id != "web-view" {
			return fmt.Errorf("no executable Go demo: %s", id)
		}
		name := fmt.Sprintf("%s/%t", id, dark)
		if w := byName[name]; w != nil {
			w.native.Send(sel("deminiaturize:"), objc.ID(0))
			w.native.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
			app.Send(sel("activateIgnoringOtherApps:"), true)
			return nil
		}
		surface, err := ui.NewSurface(width, height, ui.NewTheme(dark))
		if err != nil {
			return err
		}
		w := &window{surface: surface, demo: demos.New(id), start: time.Now()}
		w.demo.RunTask = func(work func() func()) {
			go func() {
				apply := work()
				_, err := h.post(h.ctx, func() error {
					if windows[w.view] == w {
						apply()
						repaint(w)
					}
					return nil
				})
				if err != nil {
					log.Printf("Studio background result: %v", err)
				}
			}()
		}

		w.demo.WriteClipboard = func(value string) bool {
			board := objc.ID(objc.GetClass("NSPasteboard")).Send(sel("generalPasteboard"))
			board.Send(sel("clearContents"))
			return board.Send(sel("setString:forType:"), stringID(value), stringID("public.utf8-plain-text")) != 0
		}
		w.native = objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: width, H: height}}, uint64(1|2|4), uint64(2), false)
		w.native.Send(sel("setReleasedWhenClosed:"), false)
		w.native.Send(sel("setAcceptsMouseMovedEvents:"), true)
		w.native.Send(sel("setTitle:"), stringID("Omni Studio — "+id))
		w.native.Send(sel("setDelegate:"), delegate)
		w.view = objc.ID(viewClass).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: width, H: height}})
		w.view.Send(sel("setImageScaling:"), uint64(2))
		tracking := objc.ID(objc.GetClass("NSTrackingArea")).Send(sel("alloc")).Send(sel("initWithRect:options:owner:userInfo:"), rect{}, uint64(1|2|0x20|0x200), w.view, objc.ID(0))
		w.view.Send(sel("addTrackingArea:"), tracking)
		tracking.Send(sel("release"))
		w.native.Send(sel("setContentView:"), w.view)
		w.view.Send(sel("release"))
		if id == "web-view" {
			web, err := newEmbeddedWeb(w.view)
			if err != nil {
				w.native.Send(sel("close"))
				surface.Close()
				return err
			}
			w.web = web
			web.HTML(embeddedDemoHTML)
			w.demo.NativeWeb = ui.NewWebViewState(web)
		}
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
		if _, scheduled := surface.NextWakeup(); scheduled {
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
		done, err := h.post(context.Background(), func() error {
			app.Send(sel("stop:"), objc.ID(0))
			event := objc.ID(objc.GetClass("NSEvent")).Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"), uint64(15), point{}, uint64(0), float64(0), int64(0), objc.ID(0), int16(0), int64(0), int64(0))
			app.Send(sel("postEvent:atStart:"), event, true)
			return nil
		})
		if err == nil {
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
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.frameToken++
	token := w.frameToken
	if err := w.surface.Mount(w.demo.View()); err != nil {
		return err
	}
	w.surface.Advance(time.Since(w.start))
	w.surface.SyncNativeViews()
	scale := objc.Send[float64](w.native, sel("backingScaleFactor"))
	data, err := frames.render(w.surface, width, height, scale)
	if err != nil {
		return err
	}

	// NSData copies the bytes before returning: AppKit never retains Go memory.
	nsdata := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&data[0]), uint64(len(data)))
	runtime.KeepAlive(data)
	image := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), nsdata)
	if image == 0 {
		return fmt.Errorf("AppKit rejected rendered image")
	}
	image.Send(sel("setSize:"), size{W: width, H: height})
	w.view.Send(sel("setImage:"), image)
	image.Send(sel("release"))
	if delay, scheduled := w.surface.NextWakeup(); scheduled && w.native.Send(sel("isVisible")) != 0 && w.native.Send(sel("isMiniaturized")) == 0 {
		w.timer = time.AfterFunc(delay, func() {
			_, err := h.post(h.ctx, func() error {
				if token != w.frameToken {
					return nil
				}
				w.timer = nil
				if windows[w.view] != w || w.native.Send(sel("isVisible")) == 0 || w.native.Send(sel("isMiniaturized")) != 0 {
					return nil
				}
				return paint(w, h)
			})
			if err != nil && h.ctx.Err() == nil {
				log.Printf("Studio animation scheduling: %v", err)
			}
		})
	}
	return nil
}
