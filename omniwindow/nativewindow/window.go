// Package nativewindow provides optional CGO-free Win32 and X11 software windows.
// All callbacks run on the thread calling Run. This package is separate from
// Studio's WebView shell and does not add a WebView dependency to applications.
package nativewindow

import (
	"context"
	"errors"
	"image"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
)

type Event struct {
	Ctrl, Alt, Super           bool
	Kind                       string // paint, resize, down, up, move, wheel, key, text, composition, blur, focus, close
	Width, Height              int
	X, Y, DeltaX, DeltaY       float64
	Button                     int
	Key, Text                  string
	Shift, Command, Repeat, Up bool
}

// Options callbacks and their application state belong exclusively to Run's UI goroutine.
// Paint must return an opaque RGBA framebuffer matching the client size in pixels.
// Tick is called only when NextFrame requests a timer, never by a polling loop.
type Options struct {
	Window    omniwindow.WindowOptions
	Event     func(Event) error
	Paint     func(width, height int) (*image.RGBA, error)
	NextFrame func() (time.Duration, bool)
	Tick      func() error
	Ready     func(*Window) error
	// Closing runs on the UI thread before the native window is destroyed, including Ready failure.
	Closing     func()
	ShouldClose func() bool
}
type backend interface {
	next(int) ([]Event, error)
	present(*image.RGBA) error
	wake()
	close()
}
type Window struct {
	parent, root   *Window
	children       []*Window
	frame          frameState
	now            func() time.Time
	options        Options
	used           atomic.Bool
	mu             sync.Mutex
	driver         backend
	queue          *dispatch.Queue
	closed         bool
	closeRequested bool // UI goroutine only
}

func New(o Options) (*Window, error) {
	if err := o.Window.Validate(); err != nil {
		return nil, err
	}
	if o.Window.URL != "" {
		return nil, errors.New("nativewindow: URL requires a WebView driver")
	}
	if o.Window.TitleBarStyle != "" && o.Window.TitleBarStyle != omniwindow.TitleBarDefault {
		return nil, errors.New("nativewindow: custom titlebar styles are not implemented by this driver")
	}
	if o.Window.TrafficLightPosition != nil {
		return nil, errors.New("nativewindow: traffic lights require the macOS driver")
	}
	if o.Window.Width == 0 {
		o.Window.Width = 960
	}
	if o.Window.Height == 0 {
		o.Window.Height = 640
	}
	if err := o.Window.Validate(); err != nil {
		return nil, err
	}
	if strings.ContainsRune(o.Window.Title, 0) {
		return nil, errors.New("nativewindow: title contains NUL")
	}
	if !validSize(o.Window.Width, o.Window.Height) {
		return nil, errors.New("nativewindow: invalid dimensions")
	}
	return &Window{options: o, queue: dispatch.New(256), now: time.Now, frame: frameState{width: o.Window.Width, height: o.Window.Height, dirty: true}}, nil
}

// Post schedules state changes and repaint from a worker. Do not wait for its
// result inside an event callback. Pending calls are rejected during teardown.
func (w *Window) Post(ctx context.Context, fn func() error) (<-chan error, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, dispatch.ErrClosed
	}
	result, err := w.queue.Submit(ctx, fn)
	if err == nil && w.driver != nil {
		w.driver.wake()
	}
	return result, err
}
func (w *Window) Run(ctx context.Context) error {
	return w.run(ctx, openBackend)
}

func (w *Window) run(ctx context.Context, open func(omniwindow.WindowOptions) (backend, error)) error {
	if ctx == nil {
		return errors.New("nativewindow: nil context")
	}
	if !w.used.CompareAndSwap(false, true) {
		return errors.New("nativewindow: Run is single-use")
	}
	defer func() { w.mu.Lock(); defer w.mu.Unlock(); w.closed = true; w.queue.Close() }()
	if err := ctx.Err(); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	b, err := open(w.options.Window)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.driver = b
	w.mu.Unlock()
	defer func() { w.mu.Lock(); defer w.mu.Unlock(); w.driver = nil; b.close() }()
	defer func() {
		for len(w.children) > 0 {
			w.destroyChild(w.children[0])
		}
	}()
	stop := context.AfterFunc(ctx, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.driver != nil {
			w.driver.wake()
		}
	})
	defer stop()
	if w.options.Closing != nil {
		defer w.options.Closing()
	}
	if w.options.Ready != nil {
		if err := w.options.Ready(w); err != nil {
			return err
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		w.drainFrame()
		if w.closeRequested {
			return nil
		}
		if err := w.pumpChildren(); err != nil {
			return err
		}
		if err := w.renderFrame(w.now()); err != nil {
			return err
		}
		timeout := w.childTimeout(w.frameTimeout(w.now()))
		events, err := b.next(timeout)
		if err != nil {
			return err
		}
		for _, e := range events {
			closeWindow, err := w.handleEvent(e)
			if err != nil {
				return err
			}
			if closeWindow {
				return nil
			}
		}
	}
}
func validSize(w, h int) bool {
	return w > 0 && h > 0 && w <= 16384 && h <= 16384 && int64(w)*int64(h) <= 64_000_000
}
func bgra(img *image.RGBA) []byte {
	out := make([]byte, img.Rect.Dx()*img.Rect.Dy()*4)
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			s := img.PixOffset(img.Rect.Min.X+x, img.Rect.Min.Y+y)
			d := (y*img.Rect.Dx() + x) * 4
			out[d] = img.Pix[s+2]
			out[d+1] = img.Pix[s+1]
			out[d+2] = img.Pix[s]
			out[d+3] = 255
		}
	}
	return out
}
