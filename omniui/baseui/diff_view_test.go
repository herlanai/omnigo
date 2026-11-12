package baseui

import (
	"strings"
	"testing"
)

func TestDiffRevertExactLinesAndUndo(t *testing.T) {
	for _, pair := range [][2]string{{"", ""}, {"", "hello\n"}, {"old\n", ""}, {"a\r\nb\r\n", "a\r\nc\r\n"}, {"a\nb", "a\nb\n"}, {"a\nx\nb\ny\nc\n", "a\nX\nb\nY\nc\n"}, {strings.Repeat("old\n", 1100), strings.Repeat("new\n", 1100)}} {
		s := &DiffState{Old: pair[0], New: pair[1]}
		count := len(s.Hunks())
		for len(s.Hunks()) > 0 {
			if !s.Revert(0) {
				t.Fatal("revert failed")
			}
		}
		if s.New != s.Old {
			t.Fatalf("lost line endings: %q != %q", s.New, s.Old)
		}
		for i := 0; i < count; i++ {
			if !s.Undo() {
				t.Fatal("missing undo")
			}
		}
		if s.New != pair[1] {
			t.Fatal("undo lost source")
		}
		if s.Revert(-1) || s.Revert(len(s.Hunks())) {
			t.Fatal("invalid hunk")
		}
	}
	s := &DiffState{Old: "a\nx\nb\ny\nc", New: "a\nX\nb\nY\nc"}
	h := s.Hunks()
	if len(h) != 2 || h[0] != (DiffHunk{1, 2, 1, 2}) || h[1] != (DiffHunk{3, 4, 3, 4}) {
		t.Fatal(h)
	}
	h[0].OldStart = 100
	if s.Hunks()[0].OldStart != 1 {
		t.Fatal("escaped state")
	}
	s.SetText("new", "new")
	if len(s.Hunks()) != 0 || s.Undo() {
		t.Fatal("stale history")
	}
}
func TestDiffViewActionsAndReadOnly(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(720, 480, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		state := &DiffState{Old: "a\nx\nb\ny\nc", New: "a\nX\nb\nY\nc"}
		calls := 0
		build := func() *Element { return DiffView("diff", state, 200, func(string) { calls++ }) }
		mount(t, s, build())
		click(t, s, "diff/next")
		if state.Current != 1 {
			t.Fatal("navigation")
		}
		mount(t, s, build())
		click(t, s, "diff/revert")
		if state.New != "a\nX\nb\ny\nc" || calls != 1 {
			t.Fatal("revert action", state.New)
		}
		mount(t, s, build())
		click(t, s, "diff/undo")
		if state.New != "a\nX\nb\nY\nc" {
			t.Fatal("undo action")
		}
		for _, blocked := range []*Element{build().Disabled(true), build().ReadOnly(true)} {
			mount(t, s, blocked)
			click(t, s, "diff/revert")
			if calls != 2 {
				t.Fatal("blocked mutation")
			}
		}
		mount(t, s, build())
		click(t, s, "diff/layout")
		if !state.Unified {
			t.Fatal("layout")
		}
		mount(t, s, build())
		if _, err := s.RenderScaled(1.25, nil); err != nil {
			t.Fatal(err)
		}
	}
}
