package baseui

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestNumberDraftIMEAndBounds(t *testing.T) {
	s, _ := NewSurface(500, 160, NewTheme(false))
	defer s.Close()
	state := &NumberInputState{Value: 1}
	calls := 0
	build := func() *Element {
		return NumberInput("n", "Number", state, NumberInputOptions{Min: 0, Max: 2, Step: .25, OnChange: func(float64) { calls++ }})
	}
	s.Mount(build())
	s.Focus("n/input")
	state.Text.Select(0, 1)
	s.TextInput("-")
	if state.Value != 1 || calls != 0 {
		t.Fatal("incomplete input committed")
	}
	s.SetComposition("2")
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != 1 {
		t.Fatal("stepped during composition")
	}
	state.Text.CancelComposition()
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != 1.25 {
		t.Fatal(state.Value)
	}
	s.Mount(build())
	s.Key(KeyEvent{Key: "Up"})
	s.Key(KeyEvent{Key: "Up"})
	s.Key(KeyEvent{Key: "Up"})
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != 2 {
		t.Fatal(state.Value)
	}
	s.Mount(build().ReadOnly(true))
	s.Key(KeyEvent{Key: "Down"})
	s.TextInput("0")
	if state.Value != 2 {
		t.Fatal("read only")
	}
	state.Value = .5
	s.Mount(build())
	if state.Text.String() != "0.5" {
		t.Fatal("external value did not synchronize")
	}
}
func TestDateAndTimePreserveUneditedParts(t *testing.T) {
	zone := time.FixedZone("Test", 8*3600)
	initial := time.Date(2024, 2, 28, 23, 59, 12, 123, zone)
	s, _ := NewSurface(600, 600, NewTheme(false))
	defer s.Close()
	d := &DateInputState{Value: initial}
	build := func() *Element { return DateInput("d", "Date", d, DateInputOptions{}) }
	s.Mount(build())
	s.Focus("d/trigger")
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(build())
	s.Focus("d/calendar/date/2024-02-29")
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(build())
	if d.Open || d.Value.Day() != 29 || d.Value.Hour() != 23 || d.Value.Location() != zone || d.Value.Nanosecond() != 123 {
		t.Fatal(d.Value)
	}
	tm := &TimeInputState{Value: initial}
	tb := func() *Element { return TimeInput("t", "Time", tm, TimeInputOptions{TwelveHour: true}) }
	s.Mount(tb())
	s.Focus("t/minutes/input")
	s.Key(KeyEvent{Key: "Up"})
	s.Mount(tb())
	if tm.Value.Minute() != 0 || tm.Value.Day() != 28 || tm.Value.Hour() != 23 {
		t.Fatal(tm.Value)
	}
	s.Focus("t/hours/input")
	s.Key(KeyEvent{Key: "Up"})
	s.Mount(tb())
	if tm.Value.Hour() != 0 || tm.Value.Day() != 28 {
		t.Fatal(tm.Value)
	}
	s.Focus("t/period")
	s.Key(KeyEvent{Key: "Enter"})
	if tm.Value.Hour() != 12 {
		t.Fatal(tm.Value)
	}
}
func TestRatingPointerCancellationAndKeys(t *testing.T) {
	s, _ := NewSurface(300, 120, NewTheme(false))
	defer s.Close()
	value := 3
	build := func() *Element { return Rating("r", "Rating", value, 5, func(v int) { value = v }) }
	s.Mount(build())
	r := s.entries[s.index["r"]].bounds
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 70, Y: r.Y + 16})
	s.Pointer(PointerEvent{Kind: "cancel"})
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 70, Y: r.Y + 16})
	if value != 3 {
		t.Fatal("cancel activated")
	}
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 70, Y: r.Y + 16})
	s.Mount(build())
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 70, Y: r.Y + 16})
	if value != 0 {
		t.Fatal("repeat selection did not clear", value)
	}
	s.Mount(build())
	s.Focus("r")
	s.Key(KeyEvent{Key: "End"})
	if value != 5 {
		t.Fatal(value)
	}
}
func TestColorPickerHexDragAndCancel(t *testing.T) {
	s, _ := NewSurface(400, 500, NewTheme(true))
	defer s.Close()
	state := &ColorPickerState{Value: color.NRGBA{255, 0, 0, 255}}
	build := func() *Element { return ColorPicker("c", "Color", state, ColorPickerOptions{}) }
	s.Mount(build())
	s.Focus("c/hex")
	state.hex.Select(0, len(state.hex.String()))
	s.TextInput("#0080FF80")
	s.Mount(build())
	want := color.NRGBA{0, 128, 255, 128}
	if state.Value != want {
		t.Fatal(state.Value)
	}
	r := s.entries[s.index["c/square"]].bounds
	s.Pointer(PointerEvent{Kind: "down", X: r.X + r.W/2, Y: r.Y + r.H/2})
	if state.Value == want {
		t.Fatal("drag did not update")
	}
	s.Mount(build())
	s.Pointer(PointerEvent{Kind: "cancel"})
	if state.Value != want {
		t.Fatal("cancel failed", state.Value)
	}
	for _, c := range []color.NRGBA{{0, 0, 0, 255}, {255, 255, 255, 128}, {255, 10, 123, 255}, {10, 255, 80, 255}} {
		h, s, v := colorHSV(c)
		round := hsvColor(h, s, v, float64(c.A)/255)
		if round != c {
			t.Fatal(c, round)
		}
	}
	s.Mount(build().Disabled(true))
	if s.Focus("c/hex") {
		t.Fatal("disabled focus")
	}
}
func TestRangeSliderNearestThumbAndCancel(t *testing.T) {
	s, _ := NewSurface(400, 120, NewTheme(false))
	defer s.Close()
	state := &RangeSliderState{Low: 20, High: 80}
	build := func() *Element {
		return RangeSlider("r", "Range", state, RangeSliderOptions{Min: 0, Max: 100, Step: 5})
	}
	s.Mount(build())
	r := s.entries[s.index["r"]].bounds
	s.Pointer(PointerEvent{Kind: "down", X: r.X + r.W*.9, Y: r.Y + 16})
	if math.Abs(state.High-90) > .01 || state.Low != 20 {
		t.Fatal(state)
	}
	s.Mount(build())
	s.Pointer(PointerEvent{Kind: "cancel"})
	if state.High != 80 {
		t.Fatal("cancel failed")
	}
	s.Mount(build())
	s.Focus("r/minimum")
	s.Key(KeyEvent{Key: "End"})
	if state.Low != 80 {
		t.Fatal("crossed upper thumb")
	}
	s.Mount(build())
	s.Focus("r/maximum")
	s.Key(KeyEvent{Key: "Home"})
	if state.High != 80 {
		t.Fatal("crossed lower thumb")
	}
}
func TestTokensSearchAndToolbar(t *testing.T) {
	s, _ := NewSurface(600, 500, NewTheme(false))
	defer s.Close()
	state := &TokenFieldState{Tokens: []string{"Go"}}
	var callback []string
	build := func() *Element {
		return TokenField("t", "Tags", state, TokenFieldOptions{Suggestions: []string{"Go", "Rust", "Ruby"}, MaxTokens: 3, OnChange: func(v []string) { callback = v }})
	}
	s.Mount(build())
	s.Focus("t/input")
	s.TextInput("Ru")
	s.Mount(build())
	s.Key(KeyEvent{Key: "Down"})
	s.Mount(build())
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(build())
	if !reflect.DeepEqual(state.Tokens, []string{"Go", "Ruby"}) {
		t.Fatal(state.Tokens)
	}
	callback[0] = "bad"
	if state.Tokens[0] != "Go" {
		t.Fatal("callback aliases state")
	}
	s.Focus("t/input")
	s.Key(KeyEvent{Key: "Backspace"})
	if len(state.Tokens) != 1 {
		t.Fatal(state.Tokens)
	}
	b := NewTextBuffer("query")
	searched := ""
	s.Mount(SearchField("s", "Search", b, func(v string) { searched = v }))
	s.Focus("s/input")
	s.Key(KeyEvent{Key: "Enter"})
	if searched != "query" {
		t.Fatal(searched)
	}
	s.Key(KeyEvent{Key: "Escape"})
	if b.String() != "" {
		t.Fatal("not cleared")
	}
	s.Mount(Toolbar("bar", "Tools", []ToolbarItem{{ID: "a", Label: "A", Action: func() {}}, {ID: "b", Label: "B", Disabled: true}, {ID: "c", Label: "C", Action: func() {}}}))
	s.Focus("bar/a")
	s.Key(KeyEvent{Key: "Right"})
	s.Mount(Toolbar("bar", "Tools", []ToolbarItem{{ID: "a", Label: "A", Action: func() {}}, {ID: "b", Label: "B", Disabled: true}, {ID: "c", Label: "C", Action: func() {}}}))
	if s.Focused() != "bar/c" {
		t.Fatal(s.Focused())
	}
}

func TestContentControlsOwnPixelsSelectionAndHistory(t *testing.T) {
	s, _ := NewSurface(400, 400, NewTheme(false))
	defer s.Close()
	source := image.NewNRGBA(image.Rect(5, 7, 9, 9))
	draw.Draw(source, source.Bounds(), &image.Uniform{color.NRGBA{255, 0, 0, 255}}, image.Point{}, draw.Src)
	picture := Image("picture", "Red", source, ImageOptions{Width: 80, Height: 80, Fit: ImageContain})
	s.Mount(picture)
	before, _ := s.Render()
	source.SetNRGBA(5, 7, color.NRGBA{0, 255, 0, 255})
	after, _ := s.Render()
	if !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("image aliases caller pixels")
	}
	child := Button("child", "Child", func() {})
	s.Mount(Grid("grid", 2, 8, child, Text("other", "Other"), Text("last", "Last")))
	if child.grow != 0 {
		t.Fatal("grid mutated child")
	}
	if s.entries[s.index["last"]].bounds.Y <= s.entries[s.index["child"]].bounds.Y {
		t.Fatal("grid did not wrap")
	}
	selected := map[string]bool{"one": true, "locked": true}
	build := func() *Element {
		return CheckboxGroup("g", "Group", selected, []Option{{ID: "one", Label: "One"}, {ID: "two", Label: "Two"}, {ID: "locked", Label: "Locked", Disabled: true}}, func(v map[string]bool) { selected = v })
	}
	s.Mount(build())
	if !s.entries[s.index["g/all"]].element.mixed {
		t.Fatal("aggregate not mixed")
	}
	s.Focus("g/all")
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(build())
	if !selected["two"] {
		t.Fatal(selected)
	}
	s.Key(KeyEvent{Key: "Enter"})
	if selected["one"] || selected["two"] || !selected["locked"] {
		t.Fatal(selected)
	}
	edit := &EditableTextState{Value: "report.txt"}
	eb := func() *Element { return EditableText("e", "Filename", edit, nil) }
	s.Mount(eb())
	s.Focus("e/input")
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(eb())
	if edit.draft.Selected() != "report" || s.Focused() != "e/input" {
		t.Fatal("rename selection/focus")
	}
	s.TextInput("draft")
	s.Key(KeyEvent{Key: "Escape"})
	s.Mount(eb())
	if edit.Editing || edit.Value != "report.txt" {
		t.Fatal("cancel did not restore")
	}
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(eb())
	s.TextInput("final")
	s.Key(KeyEvent{Key: "Enter"})
	s.Mount(eb())
	if edit.Value != "final.txt" || edit.Editing {
		t.Fatal(edit.Value)
	}
	find := &FindBarState{Open: true}
	fb := func() *Element { return FindBar("f", find, FindBarOptions{Matches: 3}) }
	s.Mount(fb())
	s.Focus("f/query")
	s.Key(KeyEvent{Key: "Enter", Shift: true})
	s.Mount(fb())
	if find.Current != 2 {
		t.Fatal(find.Current)
	}
	s.Key(KeyEvent{Key: "Enter"})
	if find.Current != 0 {
		t.Fatal(find.Current)
	}
	s.Key(KeyEvent{Key: "Escape"})
	s.Mount(fb())
	if find.Open || s.Focus("f/query") {
		t.Fatal("find not closed")
	}
}
