package baseui

import (
	"bytes"
	"testing"
	"time"
)

func TestResizableBoundsKeyboardAndCancellation(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		s := surface(t)
		state := &SplitState{Fraction: .5, MinFirst: 40, MinSecond: 50}
		view := func() *Element {
			return Resizable("split", state, vertical, Text("first", "First"), Text("second", "Second")).Width(300).Height(200)
		}
		mount(t, s, view())
		s.Focus("split/divider")
		s.Key(KeyEvent{Key: "End"})
		second, _ := s.Bounds("second")
		extent := second.W
		if vertical {
			extent = second.H
		}
		if extent < 49.99 {
			t.Fatal("minimum second pane ignored", second)
		}
		s.Key(KeyEvent{Key: "Home"})
		first, _ := s.Bounds("first")
		extent = first.W
		if vertical {
			extent = first.H
		}
		if extent != 40 {
			t.Fatal("minimum first pane ignored", first)
		}
		before := state.Fraction
		grip, _ := s.Bounds("split/divider")
		s.Pointer(PointerEvent{Kind: "down", X: grip.X + grip.W/2, Y: grip.Y + grip.H/2})
		s.Pointer(PointerEvent{Kind: "move", X: 1000, Y: 1000})
		if state.Fraction == before {
			t.Fatal("divider drag ignored")
		}
		s.Pointer(PointerEvent{Kind: "cancel"})
		if state.Fraction != before {
			t.Fatal("cancel did not restore fraction")
		}
		mount(t, s, view().ReadOnly(true))
		s.Focus("split/divider")
		s.Key(KeyEvent{Key: "End"})
		if state.Fraction != before {
			t.Fatal("read-only split changed")
		}
	}
}
func TestAspectDirectionAndSheet(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", AspectRatio("ratio", 2, Text("child", "Aspect")).Width(200), Direction("rtl", RightToLeft, Row("row", Button("a", "A", nil).Width(40), Button("b", "B", nil).Width(40))).Width(200)))
	r, _ := s.Bounds("ratio")
	if r.H != 100 {
		t.Fatal(r)
	}
	a, _ := s.Bounds("a")
	b, _ := s.Bounds("b")
	if a.X <= b.X {
		t.Fatal("RTL row order", a, b)
	}
	for _, side := range []Side{Left, Right, Top, Bottom} {
		mount(t, s, Column("root", Button("launch", "Launch", nil), Sheet("sheet", "Details", true, side, nil, Text("body", "Body"))))
		panel, _ := s.Bounds("sheet")
		switch side {
		case Left:
			if panel.X != 0 || panel.H != 480 {
				t.Fatal(panel)
			}
		case Right:
			if panel.X+panel.W != 640 || panel.H != 480 {
				t.Fatal(panel)
			}
		case Top:
			if panel.Y != 0 || panel.W != 640 {
				t.Fatal(panel)
			}
		case Bottom:
			if panel.Y+panel.H != 480 || panel.W != 640 {
				t.Fatal(panel)
			}
		}
	}
}
func TestSkeletonExplicitClockAndReducedMotion(t *testing.T) {
	for _, dark := range []bool{false, true} {
		theme := NewTheme(dark)
		s, err := NewSurface(160, 80, theme)
		if err != nil {
			t.Fatal(err)
		}
		mount(t, s, Skeleton("skeleton", 140, 40))
		a, _ := s.Render()
		s.Advance(500 * time.Millisecond)
		b, _ := s.Render()
		if bytes.Equal(a.Pix, b.Pix) || !s.NeedsAnimation() {
			t.Fatal("skeleton did not animate")
		}
		s.Close()
		theme.ReduceMotion = true
		s, err = NewSurface(160, 80, theme)
		if err != nil {
			t.Fatal(err)
		}
		mount(t, s, Skeleton("skeleton", 140, 40))
		a, _ = s.Render()
		s.Advance(time.Second)
		b, _ = s.Render()
		if !bytes.Equal(a.Pix, b.Pix) || s.NeedsAnimation() {
			t.Fatal("reduced motion ignored")
		}
		s.Close()
	}
}

func TestDraggingStateReleasedOnUnmountAndClose(t *testing.T) {
	s := surface(t)
	state := &SplitState{Fraction: .5}
	mount(t, s, Resizable("split", state, false, Text("a", "A"), Text("b", "B")))
	grip, _ := s.Bounds("split/divider")
	s.Pointer(PointerEvent{Kind: "down", X: grip.X + 2, Y: grip.Y + 2})
	mount(t, s, Button("split/divider", "Reused ID", nil))
	if err := s.Pointer(PointerEvent{Kind: "cancel"}); err != nil {
		t.Fatal(err)
	}
	scroll := &ScrollState{}
	mount(t, s, ScrollArea("scroll", scroll, Text("long", "a\nb\nc\nd\ne\nf")).Height(40))
	_, thumb := s.scrollbar(s.entries[s.index["scroll"]], false)
	s.Pointer(PointerEvent{Kind: "down", X: thumb.X + 2, Y: thumb.Y + 2})
	mount(t, s, Button("scroll", "Reused ID", nil))
	s.Pointer(PointerEvent{Kind: "move", X: 10, Y: 10})
	s.Close()
	if err := s.Wheel(WheelEvent{}); err == nil {
		t.Fatal("closed surface accepted wheel")
	}
}
