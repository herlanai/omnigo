//go:build darwin && (amd64 || arm64)

package host

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"image"
	"runtime"
	"time"
)

// CheckFrameSubmission must run on the process main thread. It measures AppKit submission, not compositor presentation.
func CheckFrameSubmission() ([]time.Duration, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		return nil, err
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	app := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	app.Send(sel("setActivationPolicy:"), int64(1))
	w := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: 640, H: 480}}, uint64(7), uint64(2), false)
	w.Send(sel("setReleasedWhenClosed:"), false)
	defer w.Send(sel("release"))
	defer w.Send(sel("close"))
	view := objc.ID(objc.GetClass("NSImageView")).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: 640, H: 480}})
	defer view.Send(sel("release"))
	w.Send(sel("setContentView:"), view)
	w.Send(sel("orderFront:"), objc.ID(0))
	frame := image.NewRGBA(image.Rect(0, 0, 1280, 960))
	samples := make([]time.Duration, 100)
	for i := 0; i < len(samples); i++ {
		start := time.Now()
		frame.Pix[0] = byte(i)
		img, err := newFrameImage(frame, 640, 480)
		if err != nil {
			return nil, err
		}
		view.Send(sel("setImage:"), img)
		view.Send(sel("displayIfNeeded"))
		img.Send(sel("release"))
		samples[i] = time.Since(start)
	}
	return samples, nil
}
