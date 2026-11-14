//go:build darwin && (amd64 || arm64)

package host

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
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/clipboard"
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
	textContext     objc.ID
	textClosing     bool
	access          map[string]objc.ID
	accessRoots     []objc.ID
	accessFocus     string
	accessSelection ui.TextRange
	imeFocus        string
	discardingText  bool
	titlebarHidden  bool
	width, height   int
	dark            bool
	web             *embeddedWeb
	native, view    objc.ID
	surface         *ui.Surface
	content         ContentWindowOptions
	start           time.Time
	timer           *time.Timer
	frameToken      uint64
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
			log.Printf("host native callback: %v", p)
		}
	}()
	fn()
}
func runNative(ctx context.Context, options Options) error {
	if !started.CompareAndSwap(false, true) {
		return fmt.Errorf("host: Run is single-use per process")
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return err
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	board, err := clipboard.Open(clipboard.Options{})
	if err != nil {
		return err
	}
	defer board.Close()
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), 0)
	// Direct executable launches do not reliably refresh LaunchServices/Dock icons.
	// Set the icon from embedded bytes before any window is activated.
	iconBytes := options.Icon
	if len(iconBytes) > 0 {
		iconData := objc.ID(objc.GetClass("NSData")).Send(sel("dataWithBytes:length:"), unsafe.Pointer(&iconBytes[0]), uint64(len(iconBytes)))
		appIcon := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithData:"), iconData)
		if appIcon == 0 {
			return fmt.Errorf("host: application icon could not be decoded")
		}
		app.Send(sel("setApplicationIconImage:"), appIcon)
		appIcon.Send(sel("release"))
	}
	h := &Controller{queue: dispatch.New(128), ctx: ctx, nativeExchange: options.NativeExchange}
	h.screen = nativeScreen
	h.newTray = newNativeTray
	h.watchSystem = func(notify func(omniwindow.SystemEventKind)) (func(), error) { return watchNativeSystem(app, notify) }
	h.startDialog = func(r dialogRequest, done func(dialogResult)) (func(), error) { return nativeDialog(app, r, done) }
	h.appearance = func(source omniwindow.AppearanceSource) (omniwindow.AppearanceState, error) {
		return nativeAppearance(app, source)
	}
	h.activate = func() error {
		app.Send(sel("unhide:"), objc.ID(0))
		window := app.Send(sel("keyWindow"))
		if window == 0 {
			window = app.Send(sel("mainWindow"))
		}
		if window == 0 {
			ordered := app.Send(sel("orderedWindows"))
			count := ordered.Send(sel("count"))
			for i := uintptr(0); i < uintptr(count); i++ {
				candidate := ordered.Send(sel("objectAtIndex:"), i)
				if candidate.Send(sel("canBecomeKeyWindow")) != 0 {
					window = candidate
					break
				}
			}
		}
		if window != 0 {
			if window.Send(sel("isMiniaturized")) != 0 {
				window.Send(sel("deminiaturize:"), objc.ID(0))
			}
			window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
		}
		app.Send(sel("activateIgnoringOtherApps:"), true)
		return nil
	}

	controllerClass, err := objc.RegisterClass("OmniGoUIAppDispatcher", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: sel("drain:"), Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) { guard(func() { h.drain() }) }}})
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
			log.Printf("host paint: %v", err)
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
				if w.titlebarHidden && kind == "down" && event.Send(sel("buttonNumber")) == 0 {
					if r, ok := w.surface.Bounds(w.content.DragRegion); ok && p.X >= r.X && p.X < r.X+r.W && float64(w.height)-p.Y >= r.Y && float64(w.height)-p.Y < r.Y+r.H {
						w.native.Send(sel("performWindowDragWithEvent:"), event)
						return
					}
				}
				if err := w.surface.Pointer(ui.PointerEvent{X: p.X, Y: float64(w.height) - p.Y, Kind: kind, Button: int(event.Send(sel("buttonNumber")))}); err != nil {
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
				if !up && cmd && code == 13 {
					w.native.Send(sel("performClose:"), objc.ID(0))
					return
				}
				if key == "" && cmd {
					key = map[uint16]string{0: "A", 6: "Z", 8: "C", 7: "X", 9: "V"}[code]
				}
				if key == "" && (cmd || flags&(1<<18) != 0) {
					chars := event.Send(sel("charactersIgnoringModifiers"))
					if chars != 0 {
						key = objc.Send[string](chars, sel("UTF8String"))
					}
				}
				if !up && cmd && (key == "C" || key == "X" || key == "V") {
					if key == "V" {
						value, err := board.ReadText(ctx)
						if err == nil {
							if err := w.surface.Paste(value); err != nil {
								log.Print(err)
							}
						}
					} else if text, ok := w.surface.CopySelection(); ok {
						if board.WriteText(ctx, text) == nil && key == "X" {
							if _, _, err := w.surface.CutSelection(); err != nil {
								log.Print(err)
							}
						}
					}
					repaint(w)
					return
				}
				if !up && !cmd && flags&(1<<18) == 0 {
					if input, ok := w.surface.NativeText(); ok && input.Editable {
						events := objc.ID(objc.GetClass("NSArray")).Send(sel("arrayWithObject:"), event)
						id.Send(sel("interpretKeyEvents:"), events)
						return
					}
				}
				if key != "" {
					if err := w.surface.Key(ui.KeyEvent{Key: key, Up: up, Shift: flags&(1<<17) != 0, Command: cmd, Ctrl: flags&(1<<18) != 0, Alt: flags&(1<<19) != 0, Super: cmd, Repeat: event.Send(sel("isARepeat")) != 0}); err != nil {
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
			err := w.surface.Wheel(ui.WheelEvent{X: p.X, Y: float64(w.height) - p.Y, DeltaX: -objc.Send[float64](event, sel("scrollingDeltaX")) * factor, DeltaY: -objc.Send[float64](event, sel("scrollingDeltaY")) * factor})
			if err != nil {
				log.Print(err)
			}
			repaint(w)
		})
	}
	if err := registerAccessibility(repaint); err != nil {
		return err
	}
	viewClass, err := objc.RegisterClass("OmniGoUIAppCanvas", objc.GetClass("NSImageView"), []*objc.Protocol{objc.GetProtocol("NSTextInputClient")}, nil, append([]objc.MethodDef{
		{Cmd: sel("acceptsFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("scrollWheel:"), Fn: wheel},
		{Cmd: sel("mouseMoved:"), Fn: pointer("move")},
		{Cmd: sel("mouseExited:"), Fn: pointer("leave")},
		{Cmd: sel("rightMouseDown:"), Fn: pointer("down")},
		{Cmd: sel("rightMouseUp:"), Fn: pointer("up")},
		{Cmd: sel("mouseDown:"), Fn: pointer("down")}, {Cmd: sel("mouseDragged:"), Fn: pointer("move")}, {Cmd: sel("mouseUp:"), Fn: pointer("up")},
		{Cmd: sel("keyDown:"), Fn: keyboard(false)}, {Cmd: sel("keyUp:"), Fn: keyboard(true)},
	}, append(nativeTextMethods(repaint), canvasAccessibilityMethods()...)...))
	if err != nil {
		return err
	}
	delegateClass, err := objc.RegisterClass("OmniGoUIAppWindowDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("windowWillClose:"), Fn: func(_ objc.ID, _ objc.SEL, n objc.ID) {
			guard(func() {
				native := n.Send(sel("object"))
				if cancel := nativeDialogOwners[native]; cancel != nil {
					cancel()
				}
				for id, w := range windows {
					if w.native == native {
						if w.timer != nil {
							w.timer.Stop()
							w.timer = nil
						}
						w.frameToken++
						closeNativeText(w)
						closeAccessibility(w)
						w.content.close()
						_ = w.surface.Close()
						if w.web != nil {
							w.web.Close()
						}
						delete(windows, id)
						if len(windows) == 0 {
							frames.reset()
							if options.Window.URL == "" {
								options.OnQuit()
							}
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
						w.surface.CancelComposition()
						w.discardingText = true
						w.textContext.Send(sel("discardMarkedText"))
						w.discardingText = false
						w.surface.SetActive(false)
						_ = w.surface.Pointer(ui.PointerEvent{Kind: "cancel"})
						repaint(w)
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
		name := fmt.Sprintf("%s/%t", id, dark)
		if w := byName[name]; w != nil {
			w.native.Send(sel("deminiaturize:"), objc.ID(0))
			w.native.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
			app.Send(sel("activateIgnoringOtherApps:"), true)
			return nil
		}

		if options.CreateWindow == nil {
			return fmt.Errorf("host: no window factory configured")
		}
		w := &window{dark: dark, start: time.Now()}
		services := WindowServices{SupportsTitleBar: true, WriteClipboard: func(value string) bool { return board.WriteText(ctx, value) == nil }}
		services.RunTask = func(work func() func()) {
			go func() {
				apply := work()
				_, err := h.post(h.ctx, func() error {
					if windows[w.view] == w {
						if apply != nil {
							apply()
						}
						repaint(w)
					}
					return nil
				})
				if err != nil && h.ctx.Err() == nil {
					log.Printf("host background result: %v", err)
				}
			}()
		}
		content, err := options.CreateWindow(id, dark, services)
		if err != nil {
			return err
		}
		content, err = content.normalized()
		if err != nil {
			content.close()
			return err
		}
		w.content = content
		viewWidth, viewHeight := content.Width, content.Height
		surface, err := ui.NewSurface(viewWidth, viewHeight, ui.NewTheme(dark))
		if err != nil {
			content.close()
			return err
		}
		w.width, w.height, w.surface = viewWidth, viewHeight, surface
		w.native = objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: float64(viewWidth), H: float64(viewHeight)}}, uint64(1|2|4), uint64(2), false)
		w.native.Send(sel("setReleasedWhenClosed:"), false)
		w.native.Send(sel("setAcceptsMouseMovedEvents:"), true)
		w.native.Send(sel("setTitle:"), stringID(content.Title))
		w.native.Send(sel("setDelegate:"), delegate)
		w.view = objc.ID(viewClass).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: float64(viewWidth), H: float64(viewHeight)}})
		w.view.Send(sel("setImageScaling:"), uint64(2))
		tracking := objc.ID(objc.GetClass("NSTrackingArea")).Send(sel("alloc")).Send(sel("initWithRect:options:owner:userInfo:"), rect{}, uint64(1|2|0x20|0x200), w.view, objc.ID(0))
		w.view.Send(sel("addTrackingArea:"), tracking)
		tracking.Send(sel("release"))
		w.native.Send(sel("setContentView:"), w.view)
		w.view.Send(sel("release"))
		if content.WebView != nil {
			web, err := newEmbeddedWeb(w.view)
			if err != nil {
				w.native.Send(sel("close"))
				surface.Close()
				content.close()
				return err
			}
			w.web = web
			web.HTML(content.WebView.HTML)
			if content.WebView.OnStateReady != nil {
				content.WebView.OnStateReady(ui.NewWebViewState(web))
			}
			if content.WebView.OnReady != nil {
				content.WebView.OnReady(web)
			}
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
		// Start animated windows after they are visible.
		if _, scheduled := surface.NextWakeup(); scheduled {
			repaint(w)
		}
		return nil
	}
	if options.Window.URL != "" {
		closeShell, err := openShell(app, options.Window, options.OnQuit, h)
		if err != nil {
			return err
		}
		defer closeShell()
		defer h.closeDialog()
	} else if err := h.open(options.InitialWindow, false); err != nil {
		return err
	}
	// Readiness includes successful creation of the initial window. Callbacks
	// may enqueue work now; the native loop below will service that work.
	if options.OnReady != nil {
		options.OnReady(h)
	}
	go func() {
		<-ctx.Done()
		h.control(func() {
			h.closeDialog()
			for tray := range h.trays {
				tray.driver.dismiss()
			}
			app.Send(sel("stop:"), objc.ID(0))
			event := objc.ID(objc.GetClass("NSEvent")).Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"), uint64(15), point{}, uint64(0), float64(0), int64(0), objc.ID(0), int16(0), int64(0), int64(0))
			app.Send(sel("postEvent:atStart:"), event, true)
		})
	}()
	app.Send(sel("run"))
	h.closeDialog()
	for _, w := range windows {
		w.native.Send(sel("close"))
	}
	return nil
}
func paint(w *window, h *Controller) error {
	if w.content.TitleBarHidden != nil {
		if hidden := w.content.TitleBarHidden(); w.titlebarHidden != hidden {
			w.setTitlebarHidden(hidden)
		}
	}
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.frameToken++
	token := w.frameToken
	if w.content.BeforeRender != nil {
		if err := w.content.BeforeRender(w.surface); err != nil {
			return err
		}
	}
	if err := w.surface.Mount(w.content.View()); err != nil {
		return err
	}
	w.surface.Advance(time.Since(w.start))
	syncNativeInputFocus(w)
	w.surface.SyncNativeViews()
	syncAccessibility(w)
	scale := objc.Send[float64](w.native, sel("backingScaleFactor"))
	frame, err := frames.pixels(w.surface, w.width, w.height, scale)
	if err != nil {
		return err
	}
	image, err := newFrameImage(frame, w.width, w.height)
	if err != nil {
		return err
	}

	w.view.Send(sel("setImage:"), image)
	image.Send(sel("release"))
	w.view.Send(sel("inputContext")).Send(sel("invalidateCharacterCoordinates"))
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
				log.Printf("host animation scheduling: %v", err)
			}
		})
	}
	return nil
}
