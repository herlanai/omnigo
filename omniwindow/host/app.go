package host

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniwindow"
)

var started atomic.Bool

type WindowServices struct {
	SupportsTitleBar bool
	RunTask          func(work func() func())
	WriteClipboard   func(string) bool
}

// WebPage controls an embedded page on the UI goroutine. The host owns its lifetime.
type WebPage interface {
	LoadHTML(string)
	Back()
	Forward()
	SetZoom(float64)
}

type WebViewOptions struct {
	HTML         string
	OnReady      func(WebPage)
	OnStateReady func(*ui.WebViewState)
}

// ContentWindowOptions describes one Go window; callbacks run on its UI goroutine.
type ContentWindowOptions struct {
	WebView        *WebViewOptions
	Title          string
	Width, Height  int
	View           func() *ui.Element
	BeforeRender   func(*ui.Surface) error
	Close          func()
	TitleBarHidden func() bool
	DragRegion     string
}

func (c ContentWindowOptions) close() {
	if c.Close != nil {
		c.Close()
	}
}
func (c ContentWindowOptions) normalized() (ContentWindowOptions, error) {
	if c.View == nil {
		return c, errors.New("host: content View is required")
	}
	if c.Width == 0 {
		c.Width = 1040
	}
	if c.Height == 0 {
		c.Height = 840
	}
	if err := (omniwindow.WindowOptions{Title: c.Title, Width: c.Width, Height: c.Height}).Validate(); err != nil {
		return c, err
	}
	if c.Width > 16384 || c.Height > 16384 || int64(c.Width)*int64(c.Height) > 16_000_000 {
		return c, errors.New("host: canvas exceeds pixel budget")
	}
	return c, nil
}

type WindowFactory func(key string, dark bool, services WindowServices) (ContentWindowOptions, error)
type Options struct {
	// NativeExchange handles trusted same-origin RPC off the UI goroutine.
	NativeExchange func(context.Context, []byte) ([]byte, error)
	Window         omniwindow.WindowOptions
	Icon           []byte
	CreateWindow   WindowFactory
	InitialWindow  string            // Required without a Web UI shell URL.
	OnReady        func(*Controller) // UI callback; start workers here, never wait on Controller methods.
	OnQuit         func()
}

func (o Options) normalized() (Options, error) {
	if o.Window.Width == 0 {
		o.Window.Width = 1280
	}
	if o.Window.Height == 0 {
		o.Window.Height = 820
	}
	if o.Window.Title == "" {
		o.Window.Title = "OmniGo"
	}
	if err := o.Window.Validate(); err != nil {
		return o, err
	}
	if o.Window.URL != "" {
		u, err := url.Parse(o.Window.URL)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return o, fmt.Errorf("host: shell requires an absolute HTTP(S) URL")
		}
	} else if o.InitialWindow == "" || o.CreateWindow == nil {
		return o, errors.New("host: shell URL or initial content window required")
	}
	o.Icon = append([]byte(nil), o.Icon...)
	if p := o.Window.TrafficLightPosition; p != nil {
		copy := *p
		o.Window.TrafficLightPosition = &copy
	}
	return o, nil
}

// Run owns the native loop and must be entered from the process main thread.
// The AppKit backend is single-use per process. Other platforms return an error.
func Run(ctx context.Context, options Options) error {
	if ctx == nil {
		return errors.New("host: nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	options, err := options.normalized()
	if err != nil {
		return err
	}
	if !Available {
		return ErrUnsupported
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	quit := options.OnQuit
	var once sync.Once
	options.OnQuit = func() {
		once.Do(func() {
			defer cancel()
			if quit != nil {
				quit()
			}
		})
	}
	return runNative(ctx, options)
}
