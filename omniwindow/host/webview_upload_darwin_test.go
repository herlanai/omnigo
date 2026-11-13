//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Opt-in because this test creates real AppKit sheets. No sleeps or synthetic
// mouse events: exercise the installed WKUIDelegate and deterministic cleanup.
func checkNativeWebUploadLifecycle(t uploadTestReporter) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		t.Fatal(err)
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	window := objc.ID(objc.GetClass("NSWindow")).Send(sel("alloc")).Send(sel("initWithContentRect:styleMask:backing:defer:"), rect{S: size{W: 500, H: 400}}, uint64(1|2|4), uint64(2), false)
	window.Send(sel("setReleasedWhenClosed:"), false)
	defer window.Send(sel("release"))
	defer window.Send(sel("close"))
	view := objc.ID(objc.GetClass("NSView")).Send(sel("alloc")).Send(sel("initWithFrame:"), rect{S: size{W: 500, H: 400}})
	defer view.Send(sel("release"))
	window.Send(sel("setContentView:"), view)
	web, err := newEmbeddedWeb(view)
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	class, err := objc.RegisterClass("OmniUploadTestParameters", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("allowsMultipleSelection"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("allowsDirectories"), Fn: func(objc.ID, objc.SEL) bool { return true }},
	})
	if err != nil {
		t.Fatal(err)
	}
	parameters := objc.ID(class).Send(sel("new"))
	defer parameters.Send(sel("release"))
	calls := 0
	block := objc.NewBlock(func(_ objc.Block, urls objc.ID) {
		calls++
		if urls != 0 {
			t.Error("cancel returned file URLs")
		}
	})
	defer block.Release()
	invoke := func() {
		// Preserve the native address without a uintptr-to-pointer conversion.
		pointer := *(*unsafe.Pointer)(unsafe.Pointer(&block))
		delegate := web.web.Send(sel("UIDelegate"))
		if delegate == 0 {
			t.Fatal("missing UI delegate")
		}
		delegate.Send(sel("webView:runOpenPanelWithParameters:initiatedByFrame:completionHandler:"), web.web, parameters, objc.ID(0), pointer)
	}
	invoke()
	panel := window.Send(sel("attachedSheet"))
	if panel == 0 {
		t.Fatal("no attached open panel")
	}
	if panel.Send(sel("allowsMultipleSelection")) == 0 || panel.Send(sel("canChooseDirectories")) == 0 {
		t.Fatal("input options not forwarded")
	}
	invoke()
	if calls != 1 {
		t.Fatal("busy request not canceled", calls)
	}
	cancel := webUploadCancels[web.web]
	if cancel == nil {
		t.Fatal("missing cancellation")
	}
	cancel()
	cancel()
	if calls != 2 {
		t.Fatal("completion must run exactly once", calls)
	}
	invoke()
	web.Close()
	if calls != 3 || len(webUploadCancels) != 0 || len(nativeDialogOwners) != 0 {
		t.Fatal("close leaked pending request", calls)
	}
}

// TestMain retains the process main thread locked by the host init function.
// testing.T callbacks run on other goroutines and cannot create NSWindow objects.
func TestMain(m *testing.M) {
	if os.Getenv("OMNIGO_NATIVE_UPLOAD_TEST") != "1" {
		os.Exit(m.Run())
	}
	code := func() (code int) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				code = 1
			}
		}()
		checkNativeWebUploadLifecycle(uploadTestReporter{})
		fmt.Println("PASS: native upload delegate, options, busy cancellation, repeated cancel, and close cleanup")
		return 0
	}()
	os.Exit(code)
}

type uploadTestReporter struct{}

func (uploadTestReporter) Fatal(values ...any) { panic(fmt.Sprint(values...)) }
func (uploadTestReporter) Error(values ...any) { panic(fmt.Sprint(values...)) }
