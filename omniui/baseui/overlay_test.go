package baseui

import (
	"fmt"
	"testing"
	"time"
)

func TestScrollWheelFocusAndContentShrink(t *testing.T) {
	s := surface(t)
	state := &ScrollState{}
	items := []*Element{}
	for i := range 30 {
		items = append(items, Button(fmt.Sprint(i), "Row", nil))
	}
	view := func() *Element {
		return Column("root", ScrollArea("scroll", state, items...).Height(120).Width(220), Button("after", "After", nil))
	}
	mount(t, s, view())
	if err := s.Wheel(WheelEvent{X: 50, Y: 40, DeltaY: 80}); err != nil {
		t.Fatal(err)
	}
	if state.Y != 80 {
		t.Fatal(state.Y)
	}
	first, _ := s.Bounds("0")
	if first.Y != -80 {
		t.Fatal("content did not move", first)
	}
	if !s.Focus("29") {
		t.Fatal("offscreen child cannot receive focus")
	}
	last := s.entries[s.index["29"]]
	if last.clip.H != last.bounds.H {
		t.Fatal("focused row not revealed", last)
	}
	s.Focus("scroll")
	s.Key(KeyEvent{Key: "Home"})
	if state.Y != 0 {
		t.Fatal("Home did not scroll to start")
	}
	s.Key(KeyEvent{Key: "End"})
	_, limit := state.Limits()
	if state.Y != limit {
		t.Fatal("End did not reach limit")
	}
	items = items[:1]
	mount(t, s, view())
	if state.Y != 0 {
		t.Fatal("shrinking content retained invalid offset")
	}
}
func TestScrollDragCancelDisabledAndNestedBubble(t *testing.T) {
	s := surface(t)
	outer, inner := &ScrollState{}, &ScrollState{}
	mount(t, s, ScrollArea("outer", outer, ScrollArea("inner", inner, Text("long", "a\nb\nc\nd\ne\nf\ng")).Height(80), Text("tail", "a\nb\nc\nd\ne\nf\ng\nh\ni\nj")).Height(150).Width(200))
	s.Wheel(WheelEvent{X: 20, Y: 20, DeltaY: 1000})
	if inner.Y == 0 || outer.Y != 0 {
		t.Fatal("inner viewport did not consume", inner.Y, outer.Y)
	}
	s.Wheel(WheelEvent{X: 20, Y: 20, DeltaY: 20})
	if outer.Y != 20 {
		t.Fatal("edge did not bubble", outer.Y)
	}
	s.ScrollTo("outer", 0, 0)
	track, thumb := s.scrollbar(s.entries[s.index["outer"]], false)
	s.Pointer(PointerEvent{Kind: "down", X: thumb.X + 3, Y: thumb.Y + 2})
	s.Pointer(PointerEvent{Kind: "move", X: track.X + 3, Y: track.Y + track.H})
	if outer.Y == 0 {
		t.Fatal("scrollbar drag did not scroll")
	}
	s.Pointer(PointerEvent{Kind: "cancel"})
	before := outer.Y
	s.Pointer(PointerEvent{Kind: "move", X: track.X + 3, Y: 0})
	if outer.Y != before {
		t.Fatal("canceled drag continued")
	}
	mount(t, s, ScrollArea("outer", outer, Text("long", "a\nb\nc\nd\ne\nf\ng")).Height(50).Disabled(true))
	before = outer.Y
	s.Wheel(WheelEvent{X: 10, Y: 10, DeltaY: -100})
	if outer.Y != before {
		t.Fatal("disabled viewport scrolled")
	}
}
func TestPopoverPortalFlipDismissAndFocusRestore(t *testing.T) {
	s, err := NewSurface(300, 200, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	open := false
	calls := 0
	view := func() *Element {
		return Column("root", Column("clipped", Text("spacer", "").Height(135), Button("trigger", "Open", func() { open = true }), Popover("popup", "trigger", open, func() { open = false }, Button("inside", "Inside", nil)).Height(80)).Height(171), Button("background", "Background", func() { calls++ }))
	}
	mount(t, s, view())
	click(t, s, "trigger")
	mount(t, s, view())
	popup, _ := s.Bounds("popup")
	trigger, _ := s.Bounds("trigger")
	if popup.Y+popup.H > trigger.Y {
		t.Fatal("popup did not flip above", popup, trigger)
	}
	if s.Focused() != "inside" {
		t.Fatal("popup did not focus first action", s.Focused())
	}
	if s.Focus("background") {
		t.Fatal("background accepted focus")
	}
	s.Pointer(PointerEvent{Kind: "down", X: 290, Y: 190})
	s.Pointer(PointerEvent{Kind: "up", X: 290, Y: 190})
	if open || calls != 0 {
		t.Fatal("outside click leaked through", open, calls)
	}
	mount(t, s, view())
	if s.Focused() != "trigger" {
		t.Fatal("focus not restored", s.Focused())
	}
}
func TestNestedMenuKeyboardCheckAndTypeahead(t *testing.T) {
	s := surface(t)
	state := &MenuState{}
	checked := false
	calls := 0
	items := []MenuItem{{ID: "disabled", Label: "Disabled", Disabled: true}, {ID: "file", Label: "File", Children: []MenuItem{{ID: "check", Label: "Checked", Checked: &checked, Action: func() { calls++ }}}}, {ID: "zoom", Label: "Zoom"}}
	view := func() *Element {
		return Column("root", DropdownMenu("menu", "Actions", state, items), Button("outside", "Outside", nil))
	}
	mount(t, s, view())
	click(t, s, "menu/trigger")
	mount(t, s, view())
	if s.Focused() != "menu/item/file" {
		t.Fatal(s.Focused())
	}
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	if s.Focused() != "menu/item/file/item/check" {
		t.Fatal("submenu focus", s.Focused())
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	if s.Focused() != "menu/item/file" || !state.Open {
		t.Fatal("submenu lost parent focus", s.Focused())
	}
	s.Advance(time.Second)
	s.TextInput("z")
	if s.Focused() != "menu/item/zoom" {
		t.Fatal("typeahead failed", s.Focused())
	}
	s.Key(KeyEvent{Key: "Home"})
	mount(t, s, view())
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if !checked || calls != 1 || state.Open || s.Focused() != "menu/trigger" {
		t.Fatal("checkbox action or focus", checked, calls, state.Open, s.Focused())
	}
}
func TestSelectScrollAndComboboxIME(t *testing.T) {
	s := surface(t)
	state := &SelectState{Value: "last"}
	options := []Option{}
	for i := range 20 {
		options = append(options, Option{ID: fmt.Sprint(i), Label: fmt.Sprint(i)})
	}
	options = append(options, Option{ID: "last", Label: "Last"})
	view := func() *Element { return Select("select", "Select", state, options) }
	mount(t, s, view())
	click(t, s, "select/trigger")
	mount(t, s, view())
	if s.Focused() != "select/option/last" || state.scroll.Y == 0 {
		t.Fatal("selected row not revealed", s.Focused(), state.scroll.Y)
	}
	s.Key(KeyEvent{Key: "Home"})
	mount(t, s, view())
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if state.Value != "0" || state.Open {
		t.Fatal("selection failed", state)
	}
	combo := &ComboboxState{}
	count := 0
	comboView := func() *Element {
		return Combobox("combo", "Search", combo, []Option{{ID: "a", Label: "Apple"}, {ID: "b", Label: "Banana"}}, func(string) { count++ })
	}
	mount(t, s, comboView())
	s.Focus("combo/input")
	s.SetComposition("Ba")
	s.Key(KeyEvent{Key: "Enter"})
	if count != 0 || combo.Query.String() != "" {
		t.Fatal("composition prematurely selected")
	}
	if err := s.CommitComposition(); err != nil {
		t.Fatal(err)
	}
	mount(t, s, comboView())
	if !combo.Open || s.Focused() != "combo/input" {
		t.Fatal("IME filter lost focus", combo.Open, s.Focused())
	}
	if _, ok := s.Bounds("combo/option/a"); ok {
		t.Fatal("unmatched result visible")
	}
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, comboView())
	if combo.Value != "b" || count != 1 || combo.Open {
		t.Fatal("combobox selection", combo.Value, count)
	}
}
func TestOverlayMountValidationAndReadOnly(t *testing.T) {
	s := surface(t)
	mount(t, s, Button("keep", "Keep", nil))
	if err := s.Mount(Popover("popup", "missing", true, nil, Text("text", "Body"))); err == nil {
		t.Fatal("missing anchor accepted")
	}
	if _, ok := s.Bounds("keep"); !ok {
		t.Fatal("failed mount destroyed previous tree")
	}
	combo := &ComboboxState{}
	mount(t, s, Column("root", Combobox("combo", "Search", combo, []Option{{ID: "a", Label: "A"}}, nil)).ReadOnly(true))
	s.Focus("combo/input")
	s.TextInput("A")
	s.Key(KeyEvent{Key: "Down"})
	if combo.Open || combo.Query.String() != "" {
		t.Fatal("inherited read-only ignored")
	}
}

func TestCommandSearchKeyboardDisabledAndCompositionEscape(t *testing.T) {
	s := surface(t)
	state := &CommandState{}
	calls := 0
	view := func() *Element {
		return Command("command", state, []CommandItem{{ID: "one", Label: "First", Keywords: []string{"create"}, Action: func() { calls++ }}, {ID: "two", Label: "Second", Disabled: true, Action: func() { t.Fatal("disabled command") }}})
	}
	mount(t, s, view())
	s.Focus("command/query")
	s.TextInput("create")
	mount(t, s, view())
	if _, ok := s.Bounds("command/item/two"); ok {
		t.Fatal("keyword filter failed")
	}
	s.Key(KeyEvent{Key: "Enter"})
	s.Key(KeyEvent{Key: "Enter", Repeat: true})
	if calls != 1 {
		t.Fatal(calls)
	}
	combo := &ComboboxState{Open: true}
	mount(t, s, Combobox("combo", "Search", combo, []Option{{ID: "a", Label: "Apple"}}, nil))
	s.SetComposition("pending")
	s.Key(KeyEvent{Key: "Escape"})
	if !combo.Open || combo.Query.Composition() != "" {
		t.Fatal("Escape dismissed popup before canceling IME")
	}
	s.Key(KeyEvent{Key: "Escape"})
	if combo.Open {
		t.Fatal("second Escape did not close popup")
	}
}
