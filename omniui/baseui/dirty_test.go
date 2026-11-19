package baseui

import (
	"fmt"
	"testing"
)

// TakeDirty must never miss a visible change. A false result lets hosts skip
// the entire mount-render pass after input events that changed nothing, so
// these tests cover the paths that must and must not mark the surface dirty.
func TestTakeDirtySkipsNoopPointerMove(t *testing.T) {
	s := surface(t)
	pressed := false
	mount(t, s, Column("root",
		Button("b", "Press", func() { pressed = true }),
		Text("t", "static"),
	).Width(640).Height(480))
	s.TakeDirty() // consume the initial Mount

	// Moves over static content with an unchanged hover target change nothing.
	if err := s.Pointer(PointerEvent{X: 300, Y: 300, Kind: "move"}); err != nil {
		t.Fatal(err)
	}
	if s.TakeDirty() {
		t.Fatal("noop move marked dirty")
	}
	if err := s.Pointer(PointerEvent{X: 320, Y: 310, Kind: "move"}); err != nil {
		t.Fatal(err)
	}
	if s.TakeDirty() {
		t.Fatal("second noop move marked dirty")
	}

	// Entering a button changes hover state.
	if err := s.Pointer(PointerEvent{X: 20, Y: 20, Kind: "move"}); err != nil {
		t.Fatal(err)
	}
	if !s.TakeDirty() {
		t.Fatal("hover change was not marked dirty")
	}
	// Staying on the same button is a no-op again.
	if err := s.Pointer(PointerEvent{X: 25, Y: 25, Kind: "move"}); err != nil {
		t.Fatal(err)
	}
	if s.TakeDirty() {
		t.Fatal("move within same hover target marked dirty")
	}

	// Press and release run callbacks and pressed-state changes.
	if err := s.Pointer(PointerEvent{X: 25, Y: 25, Kind: "down"}); err != nil {
		t.Fatal(err)
	}
	if !s.TakeDirty() {
		t.Fatal("press was not marked dirty")
	}
	if err := s.Pointer(PointerEvent{X: 25, Y: 25, Kind: "up"}); err != nil {
		t.Fatal(err)
	}
	if !s.TakeDirty() {
		t.Fatal("release was not marked dirty")
	}
	if !pressed {
		t.Fatal("button activation lost")
	}
}

func TestTakeDirtyCoversFocusKeyboardAndAdvance(t *testing.T) {
	s := surface(t)
	input := NewTextBuffer("")
	mount(t, s, Column("root", Input("in", "Label", input).Width(200)).Width(640).Height(480))
	s.TakeDirty()

	if err := s.Key(KeyEvent{Key: "Tab"}); err != nil {
		t.Fatal(err)
	}
	if !s.TakeDirty() {
		t.Fatal("focus change was not marked dirty")
	}
	if s.Focused() != "in" {
		t.Fatalf("focus = %q, want input", s.Focused())
	}

	// Time without animated content changes nothing visible.
	if err := s.Advance(1 << 20); err != nil {
		t.Fatal(err)
	}
	if s.TakeDirty() {
		t.Fatal("idle advance marked dirty")
	}

	// Typing into the focused buffer changes the frame.
	if err := s.TextInput("x"); err != nil {
		t.Fatal(err)
	}
	if !s.TakeDirty() {
		t.Fatal("text input was not marked dirty")
	}
}

func TestTakeDirtyTracksAnimation(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Spinner("spin", "Loading")).Width(640).Height(480))
	s.TakeDirty()
	if !s.NeedsAnimation() {
		t.Fatal("spinner should animate")
	}
	if err := s.Advance(16 << 20); err != nil {
		t.Fatal(err)
	}
	if !s.TakeDirty() {
		t.Fatal("animated advance was not marked dirty")
	}
}

func TestMeasureCacheStaysConsistentAcrossRelayout(t *testing.T) {
	s := surface(t)
	scroll := &ScrollState{}
	items := []*Element{}
	for i := range 20 {
		items = append(items, Text(fmt.Sprintf("row-%d", i), "measure me"))
	}
	mount(t, s, Column("root", ScrollArea("list", scroll, items...).Width(240)).Width(640).Height(480))
	before := make([]Rect, len(s.entries))
	for i, item := range s.entries {
		before[i] = item.bounds
	}
	// ScrollTo forces a relayout; the memoized measurements must reproduce the
	// same geometry after the cache is cleared.
	if err := s.ScrollTo("list", 0, 40); err != nil {
		t.Fatal(err)
	}
	for i, item := range s.entries {
		if item.bounds.W != before[i].W || item.bounds.H != before[i].H {
			t.Fatalf("entry %d bounds = %+v, want %+v", i, item.bounds, before[i])
		}
	}
}
