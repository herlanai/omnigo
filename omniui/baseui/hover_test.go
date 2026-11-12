package baseui

import (
	"testing"
	"time"
)

func TestTooltipClockFocusEscapeAndUnmount(t *testing.T) {
	s := surface(t)
	view := func() *Element {
		return Column("root", Button("anchor", "Anchor", nil), Button("other", "Other", nil), Tooltip("tip", "anchor", "Helpful text"))
	}
	mount(t, s, view())
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("idle tooltip scheduled timer")
	}
	s.Pointer(PointerEvent{Kind: "move", X: 10, Y: 10})
	if delay, ok := s.NextWakeup(); !ok || delay != 500*time.Millisecond {
		t.Fatal(delay, ok)
	}
	s.Advance(499 * time.Millisecond)
	if _, ok := s.Bounds("tip"); ok {
		t.Fatal("tooltip opened too soon")
	}
	s.Advance(500 * time.Millisecond)
	if _, ok := s.Bounds("tip"); !ok {
		t.Fatal("tooltip did not open")
	}
	if s.modal != "" || s.Focused() != "" {
		t.Fatal("tooltip stole focus")
	}
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("stable tooltip is polling")
	}
	s.Key(KeyEvent{Key: "Escape"})
	if _, ok := s.Bounds("tip"); ok {
		t.Fatal("Escape did not dismiss")
	}
	s.Advance(time.Second)
	mount(t, s, view())
	if _, ok := s.Bounds("tip"); ok {
		t.Fatal("dismissed tooltip reopened under pointer")
	}
	s.Pointer(PointerEvent{Kind: "move", X: 630, Y: 470})
	s.Focus("anchor")
	if _, ok := s.Bounds("tip"); !ok {
		t.Fatal("keyboard focus did not open immediately")
	}
	s.Focus("other")
	if delay, ok := s.NextWakeup(); !ok || delay != 100*time.Millisecond {
		t.Fatal("close deadline", delay, ok)
	}
	mount(t, s, Button("plain", "Plain", nil))
	if len(s.hoverStates) != 0 {
		t.Fatal("unmounted tooltip retained state")
	}
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("unmounted tooltip retained timer")
	}
}
func TestHoverCardCrossesGapAndKeepsFocus(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Button("anchor", "Details", nil).Width(160), Button("outside", "Outside", nil), HoverCard("card", "anchor", Button("action", "Inspect", nil)).Width(180)))
	s.Pointer(PointerEvent{Kind: "move", X: 20, Y: 15})
	s.Advance(250 * time.Millisecond)
	card, _ := s.Bounds("card")
	s.Pointer(PointerEvent{Kind: "move", X: 20, Y: 38}) // 4 DIP gap
	s.Advance(350 * time.Millisecond)
	s.Pointer(PointerEvent{Kind: "move", X: card.X + 20, Y: card.Y + 20})
	s.Advance(time.Second)
	if _, ok := s.Bounds("card"); !ok {
		t.Fatal("panel closed while hovered")
	}
	s.Focus("action")
	s.Pointer(PointerEvent{Kind: "move", X: 630, Y: 470})
	s.Advance(2 * time.Second)
	if _, ok := s.Bounds("card"); !ok {
		t.Fatal("focused panel closed")
	}
	s.Key(KeyEvent{Key: "Escape"})
	if s.Focused() != "anchor" {
		t.Fatal("panel dismissal lost focus", s.Focused())
	}
	if _, ok := s.Bounds("card"); ok {
		t.Fatal("panel still visible")
	}
}
func TestHoverDisabledHiddenAndReduceMotion(t *testing.T) {
	theme := NewTheme(false)
	theme.ReduceMotion = true
	s, err := NewSurface(320, 240, theme)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	view := func(disabled, hidden bool) *Element {
		return Column("root", Button("anchor", "Anchor", nil).Disabled(disabled).Hidden(hidden), Tooltip("tip", "anchor", "Text"))
	}
	mount(t, s, view(true, false))
	s.Pointer(PointerEvent{Kind: "move", X: 10, Y: 10})
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("disabled anchor scheduled tooltip")
	}
	mount(t, s, view(false, false))
	s.Advance(500 * time.Millisecond)
	if _, ok := s.Bounds("tip"); !ok {
		t.Fatal("ReduceMotion suppressed behavioral delay")
	}
	mount(t, s, view(false, true))
	if _, ok := s.Bounds("tip"); ok {
		t.Fatal("hidden anchor retained tooltip")
	}
	s.Close()
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("closed surface scheduled timer")
	}
}
func TestHoverCardDoesNotClickThroughBlankPanel(t *testing.T) {
	s := surface(t)
	calls := 0
	mount(t, s, Column("root", Button("anchor", "Anchor", nil), Button("behind", "Behind", func() { calls++ }), HoverCard("card", "anchor", Text("body", "Noninteractive text")).HoverDelay(0, 0)))
	s.Pointer(PointerEvent{Kind: "move", X: 10, Y: 10})
	panel, _ := s.Bounds("card")
	s.Pointer(PointerEvent{Kind: "down", X: panel.X + 5, Y: panel.Y + 5})
	s.Pointer(PointerEvent{Kind: "up", X: panel.X + 5, Y: panel.Y + 5})
	if calls != 0 {
		t.Fatal("click passed through hover card")
	}
}

func TestHoverDeclaredBeforeModalAnchorAndDisabledCleanup(t *testing.T) {
	s := surface(t)
	view := func(disabled bool) *Element {
		return Column("root", HoverCard("hover", "anchor", Button("inspect", "Inspect", nil)), Dialog("modal", "Dialog", true, nil, Button("anchor", "Details", nil).Disabled(disabled)))
	}
	mount(t, s, view(false))
	s.Focus("anchor")
	if _, ok := s.Bounds("hover"); !ok {
		t.Fatal("hover overlay depends on source order")
	}
	if !s.Focus("inspect") {
		t.Fatal("hover card failed to inherit modal scope")
	}
	mount(t, s, view(true))
	if _, ok := s.Bounds("hover"); ok {
		t.Fatal("disabled anchor left panel visible")
	}
	if s.Focused() == "inspect" {
		t.Fatal("unmounted panel retained focus")
	}
}
