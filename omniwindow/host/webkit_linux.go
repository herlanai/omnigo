//go:build linux && (amd64 || arm64) && omnigo_webkitgtk

package host

import (
	"context"
	"fmt"
	"net/url"
	"runtime"

	"github.com/ebitengine/purego"
	"github.com/herlanai/omnigo/internal/runtime/dispatch"
	"github.com/herlanai/omnigo/omniwindow"
)

// Available indicates a compiled backend, not that GTK/display libraries exist.
const Available = true

func init() { runtime.LockOSThread() }

type gtkAPI struct {
	init       func() int32
	window     func() uintptr
	title      func(uintptr, string)
	size       func(uintptr, int32, int32)
	minimum    func(uintptr, int32, int32)
	child      func(uintptr, uintptr)
	present    func(uintptr)
	destroy    func(uintptr)
	webview    func() uintptr
	load       func(uintptr, string)
	stop       func(uintptr)
	iteration  func(uintptr, int32) int32
	wake       func(uintptr)
	connect    func(uintptr, string, uintptr, uintptr, uintptr, uint32) uint64
	disconnect func(uintptr, uint64)
	action     func(uintptr) uintptr
	request    func(uintptr) uintptr
	uri        func(uintptr) string
	ignore     func(uintptr)
}

func loadGTK() (*gtkAPI, error) {
	a := &gtkAPI{}
	libraries := []struct {
		name    string
		symbols map[string]any
	}{
		{"libgtk-4.so.1", map[string]any{"gtk_init_check": &a.init, "gtk_window_new": &a.window, "gtk_window_set_title": &a.title, "gtk_window_set_default_size": &a.size, "gtk_widget_set_size_request": &a.minimum, "gtk_window_set_child": &a.child, "gtk_window_present": &a.present, "gtk_window_destroy": &a.destroy}},
		{"libwebkitgtk-6.0.so.4", map[string]any{"webkit_web_view_new": &a.webview, "webkit_web_view_load_uri": &a.load, "webkit_web_view_stop_loading": &a.stop, "webkit_navigation_policy_decision_get_navigation_action": &a.action, "webkit_navigation_action_get_request": &a.request, "webkit_uri_request_get_uri": &a.uri, "webkit_policy_decision_ignore": &a.ignore}},
		{"libglib-2.0.so.0", map[string]any{"g_main_context_iteration": &a.iteration, "g_main_context_wakeup": &a.wake}},
		{"libgobject-2.0.so.0", map[string]any{"g_signal_connect_data": &a.connect, "g_signal_handler_disconnect": &a.disconnect}},
	}
	// Libraries remain loaded for process lifetime: native types and callbacks can
	// retain code pointers after a window is destroyed.
	for _, lib := range libraries {
		handle, err := purego.Dlopen(lib.name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return nil, fmt.Errorf("host: WebKitGTK requires %s: %w", lib.name, err)
		}
		for name, fn := range lib.symbols {
			symbol, err := purego.Dlsym(handle, name)
			if err != nil {
				return nil, fmt.Errorf("host: missing %s: %w", name, err)
			}
			purego.RegisterFunc(fn, symbol)
		}
	}
	return a, nil
}
func runNative(ctx context.Context, o Options) error {
	if o.Window.URL == "" || o.InitialWindow != "" || o.CreateWindow != nil {
		return fmt.Errorf("host: experimental WebKitGTK supports a Web main window only")
	}
	if o.Window.TitleBarStyle != "" && o.Window.TitleBarStyle != omniwindow.TitleBarDefault {
		return fmt.Errorf("host: WebKitGTK custom titlebars are not implemented")
	}
	if !started.CompareAndSwap(false, true) {
		return fmt.Errorf("host: Run is single-use per process")
	}
	a, err := loadGTK()
	if err != nil {
		return err
	}
	if a.init() == 0 {
		return fmt.Errorf("host: GTK cannot open a display")
	}
	window := a.window()
	if window == 0 {
		return fmt.Errorf("host: GTK window creation failed")
	}
	defer a.destroy(window)
	web := a.webview()
	if web == 0 {
		return fmt.Errorf("host: WebKit view creation failed")
	}
	a.child(window, web)
	a.title(window, o.Window.Title)
	a.size(window, int32(o.Window.Width), int32(o.Window.Height))
	a.minimum(web, int32(o.Window.MinWidth), int32(o.Window.MinHeight))
	h := &Controller{queue: dispatch.New(128), ctx: ctx, wake: func() { a.wake(0) }}
	defer h.close()
	h.activate = func() error { a.present(window); return nil }
	closed := false
	closeID := a.connect(window, "close-request", purego.NewCallback(func(uintptr, uintptr) int32 { closed = true; return 1 }), 0, 0, 0)
	defer a.disconnect(window, closeID)
	origin, _ := url.Parse(o.Window.URL)
	policyID := a.connect(web, "decide-policy", purego.NewCallback(func(_ uintptr, decision uintptr, kind int32, _ uintptr) int32 {
		if kind == 1 {
			a.ignore(decision)
			return 1
		}
		if kind != 0 {
			return 0
		}
		request := a.request(a.action(decision))
		address, err := url.Parse(a.uri(request))
		if err != nil || address.User != nil || address.Scheme != origin.Scheme || address.Host != origin.Host {
			a.ignore(decision)
			return 1
		}
		return 0
	}), 0, 0, 0)
	defer a.disconnect(web, policyID)
	defer a.stop(web)
	finished := make(chan struct{})
	woken := make(chan struct{})
	go func() {
		defer close(woken)
		select {
		case <-ctx.Done():
			a.wake(0)
		case <-finished:
		}
	}()
	defer func() { close(finished); <-woken }()
	a.load(web, o.Window.URL)
	a.present(window)
	if o.OnReady != nil {
		o.OnReady(h)
	}
	for !closed && ctx.Err() == nil {
		h.drain()
		if !closed && ctx.Err() == nil {
			a.iteration(0, 1)
		}
	}
	if o.OnQuit != nil {
		o.OnQuit()
	}
	return nil
}
