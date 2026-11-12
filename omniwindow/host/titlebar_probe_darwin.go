//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"math"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
)

// CheckTitlebar validates actual AppKit state without sleeps or synthetic timing.
// Call on the process main thread, like any host entry point.
func CheckTitlebar() error {
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return err
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), int64(1))
	w := &window{width: 960, height: 820}
	w.native = objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: 960, H: 820}}, uint64(7), uint64(2), false)
	w.native.Send(sel("setReleasedWhenClosed:"), false)
	defer w.native.Send(sel("release"))
	defer w.native.Send(sel("close"))
	w.view = objc.ID(objc.GetClass("NSImageView")).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: 960, H: 820}})
	defer w.view.Send(sel("release"))
	w.native.Send(sel("setContentView:"), w.view)
	initial := objc.Send[rect](w.native, sel("frame"))
	for _, hidden := range []bool{true, false, true, false} {
		w.setTitlebarHidden(hidden)
		mask := uint64(w.native.Send(sel("styleMask")))
		if (mask&(1<<15) != 0) != hidden || mask&1 == 0 {
			return fmt.Errorf("unexpected window style: %d", mask)
		}
		if (w.native.Send(sel("titleVisibility")) == 1) != hidden {
			return fmt.Errorf("title visibility mismatch")
		}
		if (w.native.Send(sel("titlebarAppearsTransparent")) != 0) != hidden {
			return fmt.Errorf("titlebar background mismatch")
		}
		for _, kind := range []uint64{0, 1, 2} {
			b := w.native.Send(sel("standardWindowButton:"), kind)
			if b == 0 || (b.Send(sel("isHidden")) != 0) != hidden {
				return fmt.Errorf("window button mismatch: %d", kind)
			}
		}
		bounds := objc.Send[rect](w.view, sel("bounds"))
		if bounds.S.W != 960 || bounds.S.H != 820 {
			return fmt.Errorf("canvas resized: %+v", bounds)
		}
		frame := objc.Send[rect](w.native, sel("frame"))
		if math.Abs(frame.P.X-initial.P.X) > .5 || math.Abs(frame.P.Y+frame.S.H-initial.P.Y-initial.S.H) > .5 {
			return fmt.Errorf("window top-left drift")
		}
	}
	chrome := newWindowChrome(w.native)
	for _, position := range []omniwindow.Point{{X: 12, Y: 11}, {X: 18, Y: 19}} {
		state, err := chrome.apply(TitleBarRequest{Style: omniwindow.TitleBarHiddenInset, TrafficLightPosition: &position})
		if err != nil {
			return err
		}
		for _, dimensions := range []size{{W: 960, H: 820}, {W: 1100, H: 900}} {
			w.native.Send(sel("setContentSize:"), dimensions)
			chrome.layout()
			button := w.native.Send(sel("standardWindowButton:"), uint64(0))
			if button.Send(sel("isHidden")) != 0 {
				return fmt.Errorf("HiddenInset hid traffic lights")
			}
			bounds := objc.Send[rect](button, sel("bounds"))
			converted := objc.Send[rect](button, sel("convertRect:toView:"), bounds, objc.ID(0))
			frame := objc.Send[rect](w.native, sel("frame"))
			if math.Abs(converted.P.X-position.X) > .5 || math.Abs(frame.S.H-converted.P.Y-converted.S.H-position.Y) > .5 {
				return fmt.Errorf("traffic light position mismatch: %+v in %+v, want %+v", converted, frame, position)
			}
			if state.TopInset != bounds.S.H+2*position.Y {
				return fmt.Errorf("incorrect traffic light safe area")
			}
		}
		if _, err := chrome.apply(TitleBarRequest{Style: omniwindow.TitleBarDefault}); err != nil {
			return err
		}
		if w.native.Send(sel("toolbar")) != 0 || w.native.Send(sel("titleVisibility")) != 0 {
			return fmt.Errorf("default chrome not restored")
		}
	}
	return nil
}
