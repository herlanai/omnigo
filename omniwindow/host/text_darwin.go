//go:build darwin && (amd64 || arm64)

package host

import (
	"github.com/ebitengine/purego/objc"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"log"
	"strings"
	"structs"
)

type nsRange struct {
	_                structs.HostLayout
	Location, Length uint64
}

const nsNotFound = uint64(1<<63 - 1)

func nativeRange(r ui.TextRange) nsRange {
	if r.Location < 0 {
		return nsRange{Location: nsNotFound}
	}
	return nsRange{Location: uint64(r.Location), Length: uint64(r.Length)}
}
func surfaceRange(r nsRange) ui.TextRange {
	if r.Location == nsNotFound {
		return ui.TextRange{Location: -1}
	}
	if r.Location > 1<<31 || r.Length > 1<<31 {
		return ui.TextRange{Location: -2}
	}
	return ui.TextRange{Location: int(r.Location), Length: int(r.Length)}
}
func cocoaText(value objc.ID) string {
	if value == 0 {
		return ""
	}
	if value.Send(sel("isKindOfClass:"), objc.GetClass("NSAttributedString")) != 0 {
		value = value.Send(sel("string"))
	}
	if value.Send(sel("isKindOfClass:"), objc.GetClass("NSString")) == 0 {
		return ""
	}
	return objc.Send[string](value, sel("UTF8String"))
}
func inputState(id objc.ID) (ui.NativeTextState, bool) {
	w := windows[id]
	if w == nil {
		return ui.NativeTextState{}, false
	}
	return w.surface.NativeText()
}
func nativeSurfaceRect(w *window, r ui.Rect) rect {
	local := rect{P: point{X: r.X, Y: float64(w.height) - r.Y - r.H}, S: size{W: r.W, H: r.H}}
	inWindow := objc.Send[rect](w.view, sel("convertRect:toView:"), local, objc.ID(0))
	return objc.Send[rect](w.native, sel("convertRectToScreen:"), inWindow)
}
func nativeTextMethods(repaint func(*window)) []objc.MethodDef {
	update := func(id objc.ID, fn func(*window) error) {
		guard(func() {
			if w := windows[id]; w != nil && !w.discardingText {
				syncNativeInputFocus(w)
				if err := fn(w); err != nil {
					log.Printf("host text input: %v", err)
				}
				repaint(w)
			}
		})
	}
	return []objc.MethodDef{
		{Cmd: sel("inputContext"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			w := windows[id]
			if w == nil || w.textClosing {
				return 0
			}
			state, ok := w.surface.NativeText()
			if !ok || !state.Editable {
				return 0
			}
			if w.textContext == 0 {
				w.textContext = objc.ID(objc.GetClass("NSTextInputContext")).Send(sel("alloc")).Send(sel("initWithClient:"), id)
			}
			return w.textContext
		}},

		{Cmd: sel("hasMarkedText"), Fn: func(id objc.ID, _ objc.SEL) bool { s, ok := inputState(id); return ok && s.Marked.Location >= 0 }},
		{Cmd: sel("markedRange"), Fn: func(id objc.ID, _ objc.SEL) nsRange {
			s, ok := inputState(id)
			if !ok {
				return nativeRange(ui.TextRange{Location: -1})
			}
			return nativeRange(s.Marked)
		}},
		{Cmd: sel("selectedRange"), Fn: func(id objc.ID, _ objc.SEL) nsRange {
			s, ok := inputState(id)
			if !ok {
				return nativeRange(ui.TextRange{Location: -1})
			}
			return nativeRange(s.Selected)
		}},
		{Cmd: sel("validAttributesForMarkedText"), Fn: func(objc.ID, objc.SEL) objc.ID { return objc.ID(objc.GetClass("NSArray")).Send(sel("array")) }},
		{Cmd: sel("setMarkedText:selectedRange:replacementRange:"), Fn: func(id objc.ID, _ objc.SEL, text objc.ID, selected, replacement nsRange) {
			update(id, func(w *window) error {
				r := surfaceRange(selected)
				return w.surface.EditNativeText(cocoaText(text), surfaceRange(replacement), &r)
			})
		}},
		{Cmd: sel("insertText:replacementRange:"), Fn: func(id objc.ID, _ objc.SEL, text objc.ID, replacement nsRange) {
			update(id, func(w *window) error {
				return w.surface.EditNativeText(cocoaText(text), surfaceRange(replacement), nil)
			})
		}},
		{Cmd: sel("unmarkText"), Fn: func(id objc.ID, _ objc.SEL) {
			if w := windows[id]; w == nil || w.discardingText {
				return
			}
			update(id, func(w *window) error { return w.surface.CommitComposition() })
		}},
		{Cmd: sel("attributedSubstringForProposedRange:actualRange:"), Fn: func(id objc.ID, _ objc.SEL, proposed nsRange, actual *nsRange) objc.ID {
			s, ok := inputState(id)
			if !ok || proposed.Location == nsNotFound {
				return 0
			}
			text := stringID(s.Text)
			length := uint64(text.Send(sel("length")))
			if proposed.Location > length {
				return 0
			}
			proposed.Length = min(proposed.Length, length-proposed.Location)
			if actual != nil {
				*actual = proposed
			}
			part := text.Send(sel("substringWithRange:"), proposed)
			return objc.ID(objc.GetClass("NSAttributedString")).Send(sel("alloc")).Send(sel("initWithString:"), part).Send(sel("autorelease"))
		}},
		{Cmd: sel("firstRectForCharacterRange:actualRange:"), Fn: func(id objc.ID, _ objc.SEL, requested nsRange, actual *nsRange) rect {
			w := windows[id]
			if w == nil {
				return rect{}
			}
			state, ok := w.surface.NativeText()
			if !ok {
				return rect{}
			}
			at := surfaceRange(requested).Location
			if at < 0 {
				at = state.Selected.Location
			}
			r, ok := w.surface.NativeTextRect(at)
			if !ok {
				return rect{}
			}
			if actual != nil {
				*actual = nativeRange(ui.TextRange{Location: at})
			}
			return nativeSurfaceRect(w, r)
		}},
		{Cmd: sel("characterIndexForPoint:"), Fn: func(id objc.ID, _ objc.SEL, p point) uint64 {
			w := windows[id]
			if w == nil {
				return nsNotFound
			}
			local := objc.Send[rect](w.native, sel("convertRectFromScreen:"), rect{P: p})
			local = objc.Send[rect](w.view, sel("convertRect:fromView:"), local, objc.ID(0))
			at, ok := w.surface.NativeTextIndex(local.P.X, float64(w.height)-local.P.Y)
			if !ok {
				return nsNotFound
			}
			return uint64(at)
		}},
		{Cmd: sel("doCommandBySelector:"), Fn: func(id objc.ID, _ objc.SEL, command objc.SEL) {
			update(id, func(w *window) error {
				if command == sel("cancelOperation:") {
					w.surface.CancelComposition()
					return nil
				}
				for _, base := range []string{"moveLeft", "moveRight", "moveUp", "moveDown", "moveToBeginningOfLine", "moveToEndOfLine", "moveWordLeft", "moveWordRight", "moveToBeginningOfDocument", "moveToEndOfDocument"} {
					for _, shift := range []bool{false, true} {
						name := base + ":"
						if shift {
							name = base + "AndModifySelection:"
						}
						if command == sel(name) {
							key := map[string]string{"moveLeft": "Left", "moveRight": "Right", "moveUp": "Up", "moveDown": "Down", "moveToBeginningOfLine": "Home", "moveToEndOfLine": "End", "moveWordLeft": "Left", "moveWordRight": "Right", "moveToBeginningOfDocument": "Home", "moveToEndOfDocument": "End"}[base]
							return w.surface.Key(ui.KeyEvent{Key: key, Shift: shift, Alt: strings.Contains(base, "Word"), Command: strings.Contains(base, "Document")})
						}
					}
				}
				for selector, key := range map[string]string{"deleteBackward:": "Backspace", "deleteForward:": "Delete", "insertNewline:": "Enter", "insertTab:": "Tab", "insertBacktab:": "Tab", "moveLeft:": "Left", "moveRight:": "Right", "moveUp:": "Up", "moveDown:": "Down", "moveToBeginningOfLine:": "Home", "moveToEndOfLine:": "End"} {
					if command == sel(selector) {
						return w.surface.Key(ui.KeyEvent{Key: key, Shift: selector == "insertBacktab:"})
					}
				}
				return nil
			})
		}},
	}
}

func syncNativeInputFocus(w *window) {
	if focus := w.surface.Focused(); focus != w.imeFocus {
		w.imeFocus = focus
		w.discardingText = true
		w.textContext.Send(sel("discardMarkedText"))
		w.discardingText = false
	}
}

func closeNativeText(w *window) {
	w.textClosing = true
	w.discardingText = true
	if w.textContext != 0 {
		w.textContext.Send(sel("discardMarkedText"))
		w.textContext.Send(sel("deactivate"))
		w.textContext.Send(sel("release"))
		w.textContext = 0
	}
	w.discardingText = false
}
