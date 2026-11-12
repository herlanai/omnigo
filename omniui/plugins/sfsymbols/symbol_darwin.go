//go:build darwin

package sfsymbols

import (
	"bytes"
	"errors"
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"image"
	"image/png"
	"sync"
	"unsafe"
)

var once sync.Once
var loadErr error

func render(name string, o Options) (image.Image, error) {
	once.Do(func() {
		_, loadErr = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL|purego.RTLD_LAZY)
	})
	if loadErr != nil {
		return nil, loadErr
	}
	sel := objc.RegisterName
	if objc.ID(objc.GetClass("NSThread")).Send(sel("isMainThread")) == 0 {
		return nil, errors.New("sfsymbols: call on the main UI thread")
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(sel("new"))
	defer pool.Send(sel("drain"))
	str := objc.ID(objc.GetClass("NSString")).Send(sel("stringWithUTF8String:"), name)
	img := objc.ID(objc.GetClass("NSImage")).Send(sel("imageWithSystemSymbolName:accessibilityDescription:"), str, objc.ID(0))
	if img == 0 {
		return nil, errors.New("sfsymbols: symbol unavailable on this OS")
	}
	config := objc.ID(objc.GetClass("NSImageSymbolConfiguration")).Send(sel("configurationWithPointSize:weight:"), o.Size, o.Weight)
	img = img.Send(sel("imageWithSymbolConfiguration:"), config)
	if img == 0 {
		return nil, errors.New("sfsymbols: configuration failed")
	}
	tiff := img.Send(sel("TIFFRepresentation"))
	if tiff == 0 {
		return nil, errors.New("sfsymbols: rasterization failed")
	}
	rep := objc.ID(objc.GetClass("NSBitmapImageRep")).Send(sel("alloc")).Send(sel("initWithData:"), tiff)
	if rep == 0 {
		return nil, errors.New("sfsymbols: bitmap failed")
	}
	defer rep.Send(sel("release"))
	props := objc.ID(objc.GetClass("NSDictionary")).Send(sel("dictionary"))
	data := rep.Send(sel("representationUsingType:properties:"), uint64(4), props)
	if data == 0 {
		return nil, errors.New("sfsymbols: PNG encoding failed")
	}
	length := data.Send(sel("length"))
	ptr := objc.Send[unsafe.Pointer](data, sel("bytes"))
	if length == 0 || length > 32<<20 || ptr == nil {
		return nil, errors.New("sfsymbols: invalid image data")
	}
	encoded := append([]byte(nil), unsafe.Slice((*byte)(ptr), int(length))...)
	return png.Decode(bytes.NewReader(encoded))
}
