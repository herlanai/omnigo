// A native Go control window on Windows and X11/XWayland Linux.
package main

import (
	"context"
	"fmt"
	"github.com/herlanai/omnigo/omniwindow"
	"github.com/herlanai/omnigo/omniwindow/nativewindow"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/runtime/lifecycle"
	"image"
	"log"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	text := ui.NewTextBuffer("")
	count := 0
	var window *nativewindow.Window
	top, fullscreen, protectClose := false, false, false
	notice := "Native window actions are requested through the UI queue."
	request := func(done <-chan error, err error) {
		if err != nil {
			notice = err.Error()
			return
		}
		notice = "Window action queued"
		go func() {
			err := <-done
			_, _ = window.Post(ctx, func() error {
				if err != nil {
					notice = err.Error()
				} else {
					notice = "Window action sent to the OS"
				}
				return nil
			})
		}()
	}
	s, err := ui.NewSurface(960, 640, ui.NewTheme(false))
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	build := func() *ui.Element {
		return ui.Column("page",
			ui.Text("title", "OmniGo · Native window"), ui.Text("count", fmt.Sprintf("Clicks: %d", count)),
			ui.Input("input", "Type here", text), ui.Button("increment", "Increment", func() { count++ }),
			ui.Row("actions", ui.Button("rename", "Use input as title", func() { request(window.SetTitle(ctx, text.String())) }),
				ui.Button("minimize", "Minimize", func() { request(window.Minimize(ctx)) }),
				ui.Button("maximize", "Maximize", func() { request(window.Maximize(ctx)) }),
				ui.Button("restore", "Restore", func() { fullscreen = false; request(window.Restore(ctx)) })).Gap(8),
			ui.Button("child", "Open owned child window", func() {
				_, err := window.NewChild(nativewindow.Options{
					Window: omniwindow.WindowOptions{Title: "Owned child", Width: 480, Height: 240, MinWidth: 240, MinHeight: 160},
					Paint: func(width, height int) (*image.RGBA, error) {
						surface, err := ui.NewSurface(width, height, ui.NewTheme(false))
						if err != nil {
							return nil, err
						}
						defer surface.Close()
						if err = surface.Mount(ui.Column("child", ui.Text("title", "Owned by the main window"), ui.Text("help", "Hide or minimize the parent to test the relationship.")).Padding(24).Gap(16)); err != nil {
							return nil, err
						}
						return surface.Render()
					},
				})
				if err != nil {
					notice = err.Error()
				} else {
					notice = "Child window opened on the same UI loop"
				}
			}),
			ui.Row("appearance", ui.Button("fullscreen", "Toggle fullscreen", func() { fullscreen = !fullscreen; request(window.SetFullScreen(ctx, fullscreen)) }),
				ui.Button("top", "Toggle always on top", func() { top = !top; request(window.SetAlwaysOnTop(ctx, top)) }),
				ui.Button("dim", "85% opacity", func() { request(window.SetOpacity(ctx, .85)) }),
				ui.Button("opaque", "100% opacity", func() { request(window.SetOpacity(ctx, 1)) })).Gap(8),
			ui.Checkbox("protect", "Block titlebar close (explicit Close still works)", protectClose, func(v bool) { protectClose = v }),
			ui.Button("close", "Close", func() { request(window.Close(ctx)) }), ui.Text("notice", notice),
		).Padding(24).Gap(16)
	}
	start := time.Now()
	advance := func() error { return s.Advance(time.Since(start)) }
	w, err := nativewindow.New(nativewindow.Options{
		Window: omniwindow.WindowOptions{Title: "OmniGo — native controls", Width: 960, Height: 640, MinWidth: 480, MinHeight: 320},
		Ready:  func(w *nativewindow.Window) error { window = w; return nil },
		ShouldClose: func() bool {
			if protectClose {
				notice = "Close blocked: turn off the checkbox or use Close."
			}
			return !protectClose
		},
		Event: func(e nativewindow.Event) error {
			if err := advance(); err != nil {
				return err
			}
			switch e.Kind {
			case "resize":
				if e.Width > 0 && e.Height > 0 {
					return s.Resize(e.Width, e.Height)
				}
			case "down", "move", "up", "cancel":
				return s.Pointer(ui.PointerEvent{Kind: e.Kind, X: e.X, Y: e.Y, Button: e.Button})
			case "wheel":
				return s.Wheel(ui.WheelEvent{X: e.X, Y: e.Y, DeltaX: e.DeltaX, DeltaY: e.DeltaY})
			case "key":
				if e.Key == "Escape" && !e.Up && fullscreen {
					fullscreen = false
					request(window.SetFullScreen(ctx, false))
					return nil
				}
				return s.Key(ui.KeyEvent{Key: e.Key, Up: e.Up, Shift: e.Shift, Command: e.Command, Repeat: e.Repeat, Ctrl: e.Ctrl, Alt: e.Alt, Super: e.Super})
			case "composition":
				s.SetComposition(e.Text)
			case "text":
				s.SetComposition("")
				return s.TextInput(e.Text)
			case "blur":
				s.SetComposition("")
				return s.Pointer(ui.PointerEvent{Kind: "cancel"})
			}
			return nil
		},
		Paint: func(width, height int) (*image.RGBA, error) {
			if err := s.Resize(width, height); err != nil {
				return nil, err
			}
			if err := s.Mount(build()); err != nil {
				return nil, err
			}
			return s.Render()
		},
		NextFrame: s.NextWakeup, Tick: advance,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err = lifecycle.New(lifecycle.Options{}).Run(ctx, w); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
