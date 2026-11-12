//go:build darwin

package permissionflow

import (
	"github.com/ebitengine/purego"
	"sync"
)

var once sync.Once
var loadErr error
var trusted func() bool
var preflight func() bool
var requestScreen func() bool

func check(p Permission, request bool) (Status, error) {
	if p != Accessibility && p != ScreenRecording {
		return Unsupported, ErrUnsupported
	}
	once.Do(func() {
		var ax, cg uintptr
		ax, loadErr = purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if loadErr != nil {
			return
		}
		cg, loadErr = purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if loadErr != nil {
			return
		}
		purego.RegisterLibFunc(&trusted, ax, "AXIsProcessTrusted")
		purego.RegisterLibFunc(&preflight, cg, "CGPreflightScreenCaptureAccess")
		purego.RegisterLibFunc(&requestScreen, cg, "CGRequestScreenCaptureAccess")
	})
	if loadErr != nil {
		return Unsupported, loadErr
	}
	allowed := false
	if p == Accessibility {
		allowed = trusted()
		if request && !allowed {
			return NotGranted, ErrUnsupported
		}
	} else if request {
		allowed = requestScreen()
	} else {
		allowed = preflight()
	}
	if allowed {
		return Granted, nil
	}
	return NotGranted, nil
}
