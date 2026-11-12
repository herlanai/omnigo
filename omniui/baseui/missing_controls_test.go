package baseui

import "testing"

func TestDecimalExactEditingAndProtection(t *testing.T) {
	s := surface(t)
	state := &DecimalInputState{Value: "9007199254740993.10"}
	calls := 0
	build := func() *Element {
		return DecimalInput("d", "Amount", state, DecimalInputOptions{Step: "0.01", OnChange: func(string) { calls++ }})
	}
	mount(t, s, build())
	s.Focus("d/input")
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != "9007199254740993.11" || calls != 1 {
		t.Fatal(state.Value, calls)
	}
	replace := func(text string) {
		state.Text.Select(0, len([]rune(state.Text.String())))
		if err := s.TextInput(text); err != nil {
			t.Fatal(err)
		}
	}
	replace("1,234.50")
	if state.Value != "1234.50" {
		t.Fatal(state.Value)
	}
	for _, bad := range []string{"12,34.50", "1.234", "NaN", "1e2", "-"} {
		replace(bad)
		if state.Value != "1234.50" || state.Error == "" {
			t.Fatal("invalid accepted", bad, state.Value)
		}
	}
	s.Key(KeyEvent{Key: "Escape"})
	if state.Text.String() != "1234.50" || state.Error != "" {
		t.Fatal("cancel failed")
	}
	s.SetComposition("0")
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != "1234.50" {
		t.Fatal("IME changed amount")
	}
	state.Text.CancelComposition()
	for _, readonly := range []bool{true, false} {
		mount(t, s, build().ReadOnly(readonly).Disabled(!readonly))
		s.Focus("d/input")
		s.Key(KeyEvent{Key: "Down"})
		if state.Value != "1234.50" {
			t.Fatal("protected input changed")
		}
	}
	state.Value = "2.00"
	mount(t, s, build())
	if state.Text.String() != "2.00" {
		t.Fatal("external update")
	}
}
func TestDecimalBoundariesAndGrammar(t *testing.T) {
	for _, tc := range []struct {
		text, want string
		scale      int
	}{{"-.5", "-0.50", 2}, {"1.2300", "1.23", 2}, {"-0", "0", 0}, {"123,456.0", "123456.00", 2}} {
		n, ok := decimalUnits(tc.text, tc.scale)
		if !ok || decimalString(n, tc.scale) != tc.want {
			t.Fatal(tc)
		}
	}
	s := surface(t)
	state := &DecimalInputState{Value: "0.95"}
	build := func() *Element {
		return DecimalInput("d", "Amount", state, DecimalInputOptions{Min: "0", Max: "1", Step: "0.10"})
	}
	mount(t, s, build())
	s.Focus("d/input")
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != "1.00" {
		t.Fatal(state.Value)
	}
	mount(t, s, build())
	for i := 0; i < 20; i++ {
		s.Key(KeyEvent{Key: "Down"})
	}
	if state.Value != "0.00" {
		t.Fatal(state.Value)
	}
}
func TestOutlineKeyboardCellsAndStableSelection(t *testing.T) {
	for _, table := range []bool{false, true} {
		s := surface(t)
		state := &OutlineState[int]{}
		children := map[int][]int{1: {2, 3, 4}}
		opened := 0
		o := OutlineOptions[int]{ID: func(k int) string { return string(rune('a' + k)) }, Label: func(k int) string { return string(rune('A' + k)) }, Children: func(k int) []int { return children[k] }, Disabled: func(k int) bool { return k == 3 }, OnActivate: func(k int) { opened = k }}
		build := func() *Element {
			if !table {
				return TreeOutline("t", state, []int{1, 5}, o)
			}
			return OutlineTable("t", state, []int{1, 5}, o, []OutlineColumn[int]{{Title: "Name"}, {Title: "Detail", Cell: func(id string, k int) *Element { return Text(id, "Detail") }}})
		}
		mount(t, s, build())
		s.Focus("t/item/b/select")
		s.Key(KeyEvent{Key: "Right"})
		mount(t, s, build())
		if !state.Open[1] {
			t.Fatal("not expanded")
		}
		s.Key(KeyEvent{Key: "Right"})
		mount(t, s, build())
		s.Key(KeyEvent{Key: "Down"})
		mount(t, s, build())
		if state.Selected != 4 || s.Focused() != "t/item/e/select" {
			t.Fatal("navigation", state.Selected, s.Focused())
		}
		s.Key(KeyEvent{Key: "Enter"})
		if opened != 4 {
			t.Fatal("activation")
		}
		children[1] = []int{4, 3, 2}
		mount(t, s, build())
		if state.Selected != 4 {
			t.Fatal("selection lost on reorder")
		}
		s.Key(KeyEvent{Key: "Left"})
		mount(t, s, build())
		s.Key(KeyEvent{Key: "Left"})
		mount(t, s, build())
		if state.Open[1] {
			t.Fatal("not collapsed")
		}
		if _, ok := s.Bounds("t/item/e/select"); ok {
			t.Fatal("collapsed child mounted")
		}
		mount(t, s, build().ReadOnly(true))
		s.Focus("t/item/b/select")
		s.Key(KeyEvent{Key: "Right"})
		if state.Open[1] {
			t.Fatal("readonly expansion")
		}
	}
}
func TestOutlineRejectsVisibleCycle(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("cycle accepted")
		}
	}()
	TreeOutline("t", &OutlineState[int]{Open: map[int]bool{1: true}}, []int{1}, OutlineOptions[int]{ID: func(int) string { return "1" }, Label: func(int) string { return "1" }, Children: func(int) []int { return []int{1} }})
}
