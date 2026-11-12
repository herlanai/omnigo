//go:build darwin && (amd64 || arm64)

package host

import "github.com/ebitengine/purego/objc"

// Keep the titled style so the window remains keyable and retains standard
// close/minimize behavior. Full-size content removes the titlebar's reserved area.
func (w *window) setTitlebarHidden(hidden bool) {
	const fullSizeContent uint64 = 1 << 15
	mask := uint64(w.native.Send(sel("styleMask")))
	frame := objc.Send[rect](w.native, sel("frame"))
	if hidden {
		mask |= fullSizeContent
	} else {
		mask &^= fullSizeContent
	}
	w.native.Send(sel("setStyleMask:"), mask)
	visibility := int64(0)
	if hidden {
		visibility = 1
	}
	w.native.Send(sel("setTitleVisibility:"), visibility)
	w.native.Send(sel("setTitlebarAppearsTransparent:"), hidden)
	for _, kind := range []uint64{0, 1, 2} {
		if button := w.native.Send(sel("standardWindowButton:"), kind); button != 0 {
			button.Send(sel("setHidden:"), hidden)
		}
	}
	// Maintain the logical canvas and top-left position through both transitions.
	w.native.Send(sel("setContentSize:"), size{W: float64(w.width), H: float64(w.height)})
	next := objc.Send[rect](w.native, sel("frame"))
	w.native.Send(sel("setFrameOrigin:"), point{X: frame.P.X, Y: frame.P.Y + frame.S.H - next.S.H})
	w.native.Send(sel("makeFirstResponder:"), w.view)
	w.titlebarHidden = hidden
}
