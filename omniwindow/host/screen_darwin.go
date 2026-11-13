//go:build darwin && (amd64 || arm64)

package host

import (
	"fmt"
	"math"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/herlanai/omnigo/omniwindow"
)

var screenAPI struct {
	once     sync.Once
	err      error
	builtin  func(uint32) uint32
	rotation func(uint32) float64
}

// Called only on the AppKit main thread after Cocoa has been loaded.
func nativeScreen() (omniwindow.ScreenState, error) {
	screenAPI.once.Do(func() {
		var library uintptr
		library, screenAPI.err = purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if screenAPI.err != nil {
			return
		}
		// Keep the framework loaded while registered function pointers exist.
		for _, entry := range []struct {
			name string
			fn   any
		}{
			{"CGDisplayIsBuiltin", &screenAPI.builtin},
			{"CGDisplayRotation", &screenAPI.rotation},
		} {
			address, err := purego.Dlsym(library, entry.name)
			if err != nil {
				screenAPI.err = err
				return
			}
			purego.RegisterFunc(entry.fn, address)
		}
	})
	if screenAPI.err != nil {
		return omniwindow.ScreenState{}, screenAPI.err
	}
	screens := objc.ID(objc.GetClass("NSScreen")).Send(sel("screens"))
	count := int(screens.Send(sel("count")))
	if count == 0 {
		return omniwindow.ScreenState{}, fmt.Errorf("host: no active displays")
	}
	primary := screens.Send(sel("objectAtIndex:"), uint64(0))
	frame := objc.Send[rect](primary, sel("frame"))
	top := frame.P.Y + frame.S.H
	state := omniwindow.ScreenState{Displays: make([]omniwindow.Display, 0, count)}
	for i := 0; i < count; i++ {
		screen := screens.Send(sel("objectAtIndex:"), uint64(i))
		frame := objc.Send[rect](screen, sel("frame"))
		visible := objc.Send[rect](screen, sel("visibleFrame"))
		description := screen.Send(sel("deviceDescription"))
		number := description.Send(sel("objectForKey:"), stringID("NSScreenNumber"))
		id := uint32(number.Send(sel("unsignedIntValue")))
		label := screen.Send(sel("localizedName"))
		state.Displays = append(state.Displays, omniwindow.Display{
			ID: int64(id), Label: objc.Send[string](label, sel("UTF8String")),
			Bounds:      screenRect(frame.P.X, frame.P.Y, frame.S.W, frame.S.H, top),
			WorkArea:    screenRect(visible.P.X, visible.P.Y, visible.S.W, visible.S.H, top),
			ScaleFactor: objc.Send[float64](screen, sel("backingScaleFactor")),
			Rotation:    int(math.Round(screenAPI.rotation(id))), Internal: screenAPI.builtin(id) != 0,
			Primary: i == 0,
		})
	}
	cursor := objc.Send[point](objc.ID(objc.GetClass("NSEvent")), sel("mouseLocation"))
	state.Cursor = omniwindow.Point{X: cursor.X, Y: top - cursor.Y}
	return state, nil
}
