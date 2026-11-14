//go:build darwin && (amd64 || arm64)

package host

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strconv"
)

type accessEntry struct {
	w        *window
	state    ui.Accessibility
	children []objc.ID
	parent   objc.ID
}

var accessibleObjects = map[objc.ID]*accessEntry{}
var accessClass objc.Class
var postAccessibility func(objc.ID, objc.ID)

func accessArray(ids []objc.ID) objc.ID {
	a := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	for _, id := range ids {
		a.Send(sel("addObject:"), id)
	}
	return a
}
func accessNumber(v int) objc.ID {
	return objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithInt:"), int32(v))
}
func accessRole(role string) string {
	switch role {
	case "button":
		return "AXButton"
	case "checkbox", "switch":
		return "AXCheckBox"
	case "radio":
		return "AXRadioButton"
	case "textbox", "searchbox":
		return "AXTextField"
	case "slider":
		return "AXSlider"
	case "tree", "treegrid", "table", "grid":
		return "AXOutline"
	case "row", "treeitem":
		return "AXRow"
	case "heading":
		return "AXHeading"
	case "text", "note", "label":
		return "AXStaticText"
	case "link":
		return "AXLink"
	case "progressbar", "meter":
		return "AXProgressIndicator"
	case "tab":
		return "AXRadioButton"
	case "menu":
		return "AXMenu"
	case "menuitem":
		return "AXMenuItem"
	}
	return "AXGroup"
}
func registerAccessibility(repaint func(*window)) error {
	action := func(id objc.ID, name, value string) bool {
		ref := accessibleObjects[id]
		if ref == nil {
			return false
		}
		if err := ref.w.surface.AccessibilityAction(ref.state.ID, name, value); err != nil {
			return false
		}
		if name == "focus" {
			ref.w.native.Send(sel("makeFirstResponder:"), ref.w.view)
		}
		repaint(ref.w)
		return true
	}
	methods := []objc.MethodDef{
		{Cmd: sel("accessibilityNumberOfCharacters"), Fn: func(id objc.ID, _ objc.SEL) int64 {
			if r := accessibleObjects[id]; r != nil {
				if text, ok := r.w.surface.AccessibleText(r.state.ID); ok {
					return int64(stringID(text.Text).Send(sel("length")))
				}
			}
			return 0
		}},
		{Cmd: sel("accessibilitySelectedTextRange"), Fn: func(id objc.ID, _ objc.SEL) nsRange {
			if r := accessibleObjects[id]; r != nil {
				if text, ok := r.w.surface.AccessibleText(r.state.ID); ok {
					return nativeRange(text.Selected)
				}
			}
			return nsRange{Location: nsNotFound}
		}},
		{Cmd: sel("setAccessibilitySelectedTextRange:"), Fn: func(id objc.ID, _ objc.SEL, selection nsRange) {
			if r := accessibleObjects[id]; r != nil {
				if r.w.surface.SelectAccessibleText(r.state.ID, surfaceRange(selection)) == nil {
					repaint(r.w)
					postAccessibility(id, stringID("AXSelectedTextChanged"))
				}
			}
		}},
		{Cmd: sel("accessibilitySelectedText"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				if text, ok := r.w.surface.AccessibleText(r.state.ID); ok {
					return stringID(text.Text).Send(sel("substringWithRange:"), nativeRange(text.Selected))
				}
			}
			return 0
		}},
		{Cmd: sel("accessibilityStringForRange:"), Fn: func(id objc.ID, _ objc.SEL, requested nsRange) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				if text, ok := r.w.surface.AccessibleText(r.state.ID); ok {
					str := stringID(text.Text)
					length := uint64(str.Send(sel("length")))
					if requested.Location > length || requested.Length > length-requested.Location {
						return 0
					}
					return str.Send(sel("substringWithRange:"), requested)
				}
			}
			return 0
		}},

		{Cmd: sel("isAccessibilitySelectorAllowed:"), Fn: func(id objc.ID, _ objc.SEL, selector objc.SEL) bool {
			r := accessibleObjects[id]
			if r == nil {
				return false
			}
			switch selector {
			case sel("accessibilityPerformPress"):
				return !r.state.Disabled && !r.state.ReadOnly && r.state.CanPress
			case sel("accessibilityPerformIncrement"), sel("accessibilityPerformDecrement"):
				return !r.state.Disabled && !r.state.ReadOnly && r.state.CanAdjust
			case sel("setAccessibilityValue:"):
				return !r.state.Disabled && !r.state.ReadOnly && r.state.Role == "textbox"
			case sel("setAccessibilitySelectedTextRange:"):
				return !r.state.Disabled && r.state.Role == "textbox"
			case sel("setAccessibilityFocused:"):
				return !r.state.Disabled && r.state.CanFocus
			}
			return true
		}},
		{Cmd: sel("accessibilityMinValue"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil && r.state.CanAdjust {
				return objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithDouble:"), r.state.Minimum)
			}
			return 0
		}},
		{Cmd: sel("accessibilityMaxValue"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil && r.state.CanAdjust {
				return objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithDouble:"), r.state.Maximum)
			}
			return 0
		}},

		{Cmd: sel("isAccessibilityElement"), Fn: func(id objc.ID, _ objc.SEL) bool { return accessibleObjects[id] != nil }},
		{Cmd: sel("accessibilityRole"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				if r.state.Multiline {
					return stringID("AXTextArea")
				}
				return stringID(accessRole(r.state.Role))
			}
			return stringID("AXUnknown")
		}},
		{Cmd: sel("accessibilityIdentifier"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return stringID(r.state.ID)
			}
			return 0
		}},
		{Cmd: sel("accessibilityLabel"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return stringID(r.state.Label)
			}
			return 0
		}},
		{Cmd: sel("accessibilityHelp"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return stringID(r.state.Description)
			}
			return 0
		}},
		{Cmd: sel("accessibilityParent"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return r.parent
			}
			return 0
		}},
		{Cmd: sel("accessibilityWindow"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return r.w.native
			}
			return 0
		}},
		{Cmd: sel("accessibilityTopLevelUIElement"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return r.w.native
			}
			return 0
		}},
		{Cmd: sel("accessibilityChildren"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if r := accessibleObjects[id]; r != nil {
				return accessArray(r.children)
			}
			return accessArray(nil)
		}},
		{Cmd: sel("accessibilityFrame"), Fn: func(id objc.ID, _ objc.SEL) rect {
			if r := accessibleObjects[id]; r != nil {
				return nativeSurfaceRect(r.w, r.state.Bounds)
			}
			return rect{}
		}},
		{Cmd: sel("isAccessibilityEnabled"), Fn: func(id objc.ID, _ objc.SEL) bool { r := accessibleObjects[id]; return r != nil && !r.state.Disabled }},
		{Cmd: sel("isAccessibilityFocused"), Fn: func(id objc.ID, _ objc.SEL) bool {
			r := accessibleObjects[id]
			return r != nil && r.w.surface.Focused() == r.state.ID
		}},
		{Cmd: sel("isAccessibilitySelected"), Fn: func(id objc.ID, _ objc.SEL) bool { r := accessibleObjects[id]; return r != nil && r.state.Selected }},
		{Cmd: sel("isAccessibilityExpanded"), Fn: func(id objc.ID, _ objc.SEL) bool { r := accessibleObjects[id]; return r != nil && r.state.Expanded }},
		{Cmd: sel("accessibilityValue"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			r := accessibleObjects[id]
			if r == nil {
				return 0
			}
			if r.state.Role == "checkbox" || r.state.Role == "switch" || r.state.Role == "radio" {
				v := 0
				if r.state.Checked {
					v = 1
				}
				if r.state.Mixed {
					v = 2
				}
				return accessNumber(v)
			}
			if r.state.CanAdjust {
				v, _ := strconv.ParseFloat(r.state.Value, 64)
				return objc.ID(objc.GetClass("NSNumber")).Send(sel("numberWithDouble:"), v)
			}
			return stringID(r.state.Value)
		}},
		{Cmd: sel("setAccessibilityValue:"), Fn: func(id objc.ID, _ objc.SEL, value objc.ID) { action(id, "value", cocoaText(value)) }},
		{Cmd: sel("setAccessibilityFocused:"), Fn: func(id objc.ID, _ objc.SEL, focused bool) {
			if focused {
				action(id, "focus", "")
			}
		}},
		{Cmd: sel("accessibilityPerformPress"), Fn: func(id objc.ID, _ objc.SEL) bool { return action(id, "press", "") }},
		{Cmd: sel("accessibilityPerformIncrement"), Fn: func(id objc.ID, _ objc.SEL) bool { return action(id, "increment", "") }},
		{Cmd: sel("accessibilityPerformDecrement"), Fn: func(id objc.ID, _ objc.SEL) bool { return action(id, "decrement", "") }},
	}
	var err error
	accessClass, err = objc.RegisterClass("OmniGoAccessibleControl", objc.GetClass("NSAccessibilityElement"), nil, nil, methods)
	if err != nil {
		return err
	}
	address, err := purego.Dlsym(purego.RTLD_DEFAULT, "NSAccessibilityPostNotification")
	if err != nil {
		return err
	}
	purego.RegisterFunc(&postAccessibility, address)
	return nil
}
func canvasAccessibilityMethods() []objc.MethodDef {
	return []objc.MethodDef{
		{Cmd: sel("isAccessibilityElement"), Fn: func(objc.ID, objc.SEL) bool { return false }},
		{Cmd: sel("accessibilityRole"), Fn: func(objc.ID, objc.SEL) objc.ID { return stringID("AXGroup") }},
		{Cmd: sel("accessibilityChildren"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if w := windows[id]; w != nil {
				return accessArray(w.accessRoots)
			}
			return accessArray(nil)
		}},
		{Cmd: sel("accessibilityFocusedUIElement"), Fn: func(id objc.ID, _ objc.SEL) objc.ID {
			if w := windows[id]; w != nil {
				return w.access[w.surface.Focused()]
			}
			return 0
		}},
	}
}
func syncAccessibility(w *window) {
	if accessClass == 0 {
		return
	}
	if w.access == nil {
		w.access = map[string]objc.ID{}
	}
	previousFocus := w.accessFocus
	w.accessFocus = w.surface.Focused()
	snapshot := w.surface.Accessibility()
	present := map[string]bool{}
	changed := false
	valueChanged := []objc.ID{}
	for _, state := range snapshot {
		present[state.ID] = true
		id := w.access[state.ID]
		if id == 0 {
			id = objc.ID(accessClass).Send(sel("new"))
			w.access[state.ID] = id
			changed = true
		} else {
			old := accessibleObjects[id].state
			if old.Value != state.Value || old.Checked != state.Checked || old.Selected != state.Selected || old.Expanded != state.Expanded {
				valueChanged = append(valueChanged, id)
			}
		}
		accessibleObjects[id] = &accessEntry{w: w, state: state, parent: w.view}
	}
	for key, id := range w.access {
		if !present[key] {
			delete(accessibleObjects, id)
			delete(w.access, key)
			id.Send(sel("release"))
			changed = true
		}
	}
	w.accessRoots = nil
	for _, state := range snapshot {
		id := w.access[state.ID]
		ref := accessibleObjects[id]
		parent := w.access[state.Parent]
		if parent != 0 {
			ref.parent = parent
			accessibleObjects[parent].children = append(accessibleObjects[parent].children, id)
		} else {
			w.accessRoots = append(w.accessRoots, id)
		}
	}
	if changed {
		postAccessibility(w.view, stringID("AXLayoutChanged"))
	}
	for _, id := range valueChanged {
		postAccessibility(id, stringID("AXValueChanged"))
	}
	if text, ok := w.surface.NativeText(); ok {
		if text.Selected != w.accessSelection || previousFocus != w.accessFocus {
			w.accessSelection = text.Selected
			if id := w.access[w.accessFocus]; id != 0 {
				postAccessibility(id, stringID("AXSelectedTextChanged"))
			}
		}
	}
	if previousFocus != w.accessFocus {
		if id := w.access[w.accessFocus]; id != 0 {
			postAccessibility(id, stringID("AXFocusedUIElementChanged"))
		}
	}
}
func closeAccessibility(w *window) {
	for _, id := range w.access {
		delete(accessibleObjects, id)
		id.Send(sel("release"))
	}
	w.access = nil
	w.accessRoots = nil
}
