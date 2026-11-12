package baseui

import "testing"

func TestContextMenuSecondaryAndKeyboard(t *testing.T) {
	s := surface(t)
	state := &ContextMenuState{}
	primary, action := 0, 0
	view := func() *Element {
		return Column("root", ContextMenu("context", Button("anchor", "File", func() { primary++ }), state, []MenuItem{{ID: "open", Label: "Open", Action: func() { action++ }}}), Button("other", "Other", nil))
	}
	mount(t, s, view())
	s.Pointer(PointerEvent{Kind: "down", Button: 1, X: 150, Y: 20})
	s.Pointer(PointerEvent{Kind: "up", Button: 1, X: 150, Y: 20})
	mount(t, s, view())
	if primary != 0 || !state.Menu.Open {
		t.Fatal(primary, state.Menu.Open)
	}
	popup, _ := s.Bounds("context/popup")
	if popup.X != 150 || popup.Y != 20 {
		t.Fatal("pointer placement", popup)
	}
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if action != 1 || s.Focused() != "anchor" {
		t.Fatal("action/focus", action, s.Focused())
	}
	s.Key(KeyEvent{Key: "F10", Shift: true})
	mount(t, s, view())
	popup, _ = s.Bounds("context/popup")
	anchor, _ := s.Bounds("anchor")
	if popup.Y != anchor.Y+anchor.H+4 {
		t.Fatal("keyboard placement", popup)
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	click(t, s, "anchor")
	if primary != 1 {
		t.Fatal("primary activation lost")
	}
}
func TestDisabledContextMenuAndSnapshot(t *testing.T) {
	s := surface(t)
	state := &ContextMenuState{}
	anchor := Button("anchor", "File", nil)
	root := ContextMenu("context", anchor, state, []MenuItem{{ID: "open", Label: "Open"}}).Disabled(true)
	if anchor.onContext != nil {
		t.Fatal("builder mutated caller anchor")
	}
	mount(t, s, root)
	s.Pointer(PointerEvent{Kind: "down", Button: 1, X: 20, Y: 20})
	if state.Menu.Open {
		t.Fatal("disabled context menu opened")
	}
}
