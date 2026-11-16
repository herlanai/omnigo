//go:build darwin && (amd64 || arm64)

package host

import "github.com/ebitengine/purego/objc"

// Keep the titled style so the window remains keyable and retains standard
// close/minimize behavior. Full-size content removes the titlebar's reserved area.
func (w *window) setTitlebarHidden(hidden bool) {
	// Native style changes can synchronously deliver resize notifications.
	w.titlebarHidden = hidden
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

// Own the native-button placement per window; never discover a global key window.
func (w *window) layoutWindowButtons() {
	layout := w.content.WindowButtons
	if layout == nil {
		return
	}
	bounds := objc.Send[rect](w.view, sel("bounds"))
	for i := range w.buttonParents {
		button := w.native.Send(sel("standardWindowButton:"), uint64(i))
		if button == 0 {
			continue
		}
		if w.titlebarHidden {
			if w.buttonParents[i] == 0 {
				w.buttonParents[i] = button.Send(sel("superview"))
				w.buttonParents[i].Send(sel("retain"))
				w.buttonFrames[i] = objc.Send[rect](button, sel("frame"))
			}
			if button.Send(sel("superview")) != w.view {
				w.view.Send(sel("addSubview:"), button)
			}
			button.Send(sel("setHidden:"), false)
			frame := objc.Send[rect](button, sel("frame"))
			y := bounds.S.H - layout.Top - frame.S.H
			if w.view.Send(sel("isFlipped")) != 0 {
				y = layout.Top
			}
			button.Send(sel("setFrameOrigin:"), point{X: layout.Left + float64(i)*layout.Spacing, Y: y})
			button.Send(sel("setHidden:"), false)
		} else if w.buttonParents[i] != 0 {
			w.buttonParents[i].Send(sel("addSubview:"), button)
			button.Send(sel("setFrame:"), w.buttonFrames[i])
			button.Send(sel("setHidden:"), false)
			w.buttonParents[i].Send(sel("release"))
			w.buttonParents[i] = 0
		}
	}
	// AppKit can relayout all standard buttons when a sibling is reparented
	// or unhidden. Apply positions only after all three have been installed.
	if w.titlebarHidden {
		w.view.Send(sel("layoutSubtreeIfNeeded"))
		for i := range w.buttonParents {
			button := w.native.Send(sel("standardWindowButton:"), uint64(i))
			if button == 0 {
				continue
			}
			frame := objc.Send[rect](button, sel("frame"))
			y := bounds.S.H - layout.Top - frame.S.H
			if w.view.Send(sel("isFlipped")) != 0 {
				y = layout.Top
			}
			button.Send(sel("setFrameOrigin:"), point{X: layout.Left + float64(i)*layout.Spacing, Y: y})
		}
	}
}
func (w *window) releaseButtonParents() {
	for i, parent := range w.buttonParents {
		if parent != 0 {
			parent.Send(sel("release"))
			w.buttonParents[i] = 0
		}
	}
}
