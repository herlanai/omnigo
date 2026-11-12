//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

// CheckEmbeddedWeb is an opt-in native smoke probe run on the process main
// thread. It waits for WebKit's navigation callback, never a guessed sleep.
func CheckEmbeddedWeb() error {
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return err
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), int64(1))
	window := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: width, H: height}}, uint64(1|2|4), uint64(2), false)
	window.Send(sel("setReleasedWhenClosed:"), false)
	defer window.Send(sel("release"))
	defer window.Send(sel("close"))
	parent := objc.ID(objc.GetClass("NSImageView")).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: width, H: height}})
	window.Send(sel("setContentView:"), parent)
	defer parent.Send(sel("release"))
	web, err := newEmbeddedWeb(parent)
	if err != nil {
		return err
	}
	defer web.Close()
	surface, err := ui.NewSurface(width, height, ui.NewTheme(false))
	if err != nil {
		return err
	}
	defer surface.Close()
	if err = surface.Mount(ui.Column("root", ui.WebView("web", ui.NewWebViewState(web)).Width(500).Height(300)).Padding(30)); err != nil {
		return err
	}
	surface.SyncNativeViews()
	loaded := false
	var result error
	stop := func() {
		app.Send(sel("stop:"), objc.ID(0))
		event := objc.ID(objc.GetClass("NSEvent")).Send(sel("otherEventWithType:location:modifierFlags:timestamp:windowNumber:context:subtype:data1:data2:"), uint64(15), point{}, uint64(0), float64(0), int64(0), objc.ID(0), int16(0), int64(0), int64(0))
		app.Send(sel("postEvent:atStart:"), event, true)
	}
	completion := objc.NewBlock(func(_ objc.Block, value objc.ID, jsError objc.ID) {
		if jsError != 0 {
			result = fmt.Errorf("native JavaScript failed")
		} else if value == 0 {
			result = fmt.Errorf("empty JavaScript result")
		} else {
			got := objc.Send[string](value, sel("UTF8String"))
			if got != "OmniGo WebView probe:Native input" {
				result = fmt.Errorf("unexpected document result: %q", got)
			} else {
				loaded = true
			}
		}
		stop()
	})
	defer completion.Release()
	cls, err := objc.RegisterClass("OmniEmbeddedWebProbe", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("webView:didFinishNavigation:"), Fn: func(_ objc.ID, _ objc.SEL, v objc.ID, _ objc.ID) {
			if v.Send(sel("superview")) != web.container {
				result = fmt.Errorf("web view was not embedded")
				stop()
				return
			}
			v.Send(sel("evaluateJavaScript:completionHandler:"), stringID("document.title + ':' + document.querySelector('input').value"), completion)
		}},
		{Cmd: sel("webView:didFailProvisionalNavigation:withError:"), Fn: func(objc.ID, objc.SEL, objc.ID, objc.ID, objc.ID) {
			result = fmt.Errorf("native WebKit load failed")
			stop()
		}},
	})
	if err != nil {
		return err
	}
	delegate := objc.ID(cls).Send(sel("new"))
	defer delegate.Send(sel("release"))
	web.web.Send(sel("setNavigationDelegate:"), delegate)
	defer web.web.Send(sel("setNavigationDelegate:"), objc.ID(0))
	web.HTML("<!doctype html><title>OmniGo WebView probe</title><input value='Native input'><p>Embedded WebKit is active.</p>")
	window.Send(sel("makeKeyAndOrderFront:"), objc.ID(0))
	app.Send(sel("run"))
	if result != nil {
		return result
	}
	if !loaded {
		return fmt.Errorf("native navigation callback missing")
	}
	if err = surface.Mount(ui.Text("other", "unmounted")); err != nil {
		return err
	}
	surface.SyncNativeViews()
	if web.container.Send(sel("isHidden")) == 0 {
		return fmt.Errorf("unmounted web view remained visible")
	}
	return nil
}
