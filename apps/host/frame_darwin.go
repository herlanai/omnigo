//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"image"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// newFrameImage copies premultiplied RGBA into AppKit-owned storage. NSImage
// retains the representation; no Go pointer escapes into the native view.
func newFrameImage(frame *image.RGBA, width, height int) (objc.ID, error) {
	rep := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(sel("alloc")).Send(
		sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bitmapFormat:bytesPerRow:bitsPerPixel:"),
		uintptr(0), int64(frame.Rect.Dx()), int64(frame.Rect.Dy()), int64(8), int64(4), true, false,
		stringID("NSDeviceRGBColorSpace"), uint64(0), int64(frame.Stride), int64(32))
	if rep == 0 {
		return 0, fmt.Errorf("host: bitmap allocation failed")
	}
	defer rep.Send(sel("release"))
	data := objc.Send[unsafe.Pointer](rep, sel("bitmapData"))
	if data == nil {
		return 0, fmt.Errorf("host: missing bitmap storage")
	}
	copy(unsafe.Slice((*byte)(data), len(frame.Pix)), frame.Pix)
	img := objc.ID(objc.GetClass("NSImage")).Send(sel("alloc")).Send(sel("initWithSize:"), size{W: float64(width), H: float64(height)})
	if img == 0 {
		return 0, fmt.Errorf("host: image allocation failed")
	}
	img.Send(sel("addRepresentation:"), rep)
	return img, nil
}
