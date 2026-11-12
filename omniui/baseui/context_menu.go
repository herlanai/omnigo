package baseui

// ContextMenuState retains menu selection and the pointer's anchor-relative
// location. Keyboard invocation instead attaches below the focused anchor.
type ContextMenuState struct {
	Menu      MenuState
	x, y      float64
	atPointer bool
}

// ContextMenu attaches secondary-click and Shift+F10 handling to anchor while
// preserving its normal activation and key handler. It shares menu semantics
// with DropdownMenu, including nested submenus and focus restoration.
func ContextMenu(id string, anchor *Element, state *ContextMenuState, items []MenuItem) *Element {
	if anchor == nil || state == nil {
		panic("baseui: context menu requires anchor and state")
	}
	clone := *anchor
	clone.onContext = func(x, y float64, keyboard bool) {
		state.Menu.Open = true
		state.Menu.Path = nil
		state.Menu.search = typeSearch{}
		state.x, state.y, state.atPointer = x, y, !keyboard
	}
	clone.controls = id + "/popup"
	clone.expanded = state.Menu.Open
	root := menuView(id, &clone, &state.Menu, items)
	popup := root.children[1]
	popup.pointAnchor = state.atPointer
	popup.pointX = state.x
	popup.pointY = state.y
	return root
}
func (s *Surface) contextAt(x, y float64) error {
	s.pressed = ""
	target := ""
	for i := len(s.entries) - 1; i >= 0; i-- {
		item := s.entries[i]
		if item.clip.Contains(x, y) && s.inScope(item) {
			target = item.element.id
			break
		}
	}
	for target != "" {
		item := s.entries[s.index[target]]
		if item.element.onContext != nil {
			if item.disabled || item.element.readOnly {
				return nil
			}
			s.focus = target
			return guarded(func() { item.element.onContext(x-item.bounds.X, y-item.bounds.Y, false) })
		}
		target = item.parent
	}
	return nil
}
func (s *Surface) contextFocused() error {
	for target := s.focus; target != ""; {
		i, ok := s.index[target]
		if !ok {
			return nil
		}
		item := s.entries[i]
		if item.element.onContext != nil {
			if item.disabled || item.element.readOnly || !s.inScope(item) {
				return nil
			}
			return guarded(func() { item.element.onContext(0, 0, true) })
		}
		target = item.parent
	}
	return nil
}
