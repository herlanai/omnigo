//go:build darwin && (amd64 || arm64)

package host

import (
	"bytes"
	"image"
	"os"
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

func TestNativePixelOwnership(t *testing.T) {
	if os.Getenv("OMNIGO_NATIVE_PIXEL_TEST") != "1" {
		t.Skip("opt-in AppKit pixel test")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		t.Fatal(err)
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	frame := image.NewRGBA(image.Rect(0, 0, 2, 2))
	copy(frame.Pix, []byte{255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 64, 32, 16, 128})
	expected := append([]byte(nil), frame.Pix...)
	img, err := newFrameImage(frame, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Send(sel("release"))
	clear(frame.Pix)
	rep := img.Send(sel("representations")).Send(sel("objectAtIndex:"), uint64(0))
	pixels := unsafe.Slice((*byte)(objc.Send[unsafe.Pointer](rep, sel("bitmapData"))), len(expected))
	if !bytes.Equal(pixels, expected) {
		t.Fatal("native copy, channel order or row order changed")
	}
	if rep.Send(sel("pixelsWide")) != 2 || rep.Send(sel("pixelsHigh")) != 2 {
		t.Fatal("physical size changed")
	}
	if got := objc.Send[size](img, sel("size")); got.W != 1 || got.H != 1 {
		t.Fatal("logical size changed", got)
	}
	if rep.Send(sel("bitmapFormat")) != 0 {
		t.Fatal("premultiplied RGBA format changed")
	}
}
