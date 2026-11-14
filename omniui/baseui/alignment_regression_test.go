package baseui

import (
	"fmt"
	"strings"
	"testing"
)

func TestNumberTinyStepAndBlur(t *testing.T) {
	s, _ := NewSurface(500, 160, NewTheme(false))
	defer s.Close()
	state := &NumberInputState{}
	calls := 0
	build := func() *Element {
		return NumberInput("n", "Number", state, NumberInputOptions{Min: 0, Max: 1, Step: 1e-14, OnChange: func(float64) { calls++ }})
	}
	if err := s.Mount(build()); err != nil {
		t.Fatal(err)
	}
	s.Focus("n/input")
	s.Key(KeyEvent{Key: "Up"})
	if state.Value != 1e-14 || calls != 1 {
		t.Fatal(state.Value, calls)
	}
	state.Text.Set("-")
	s.Focus("n/increase")
	if state.Text.String() == "-" || calls != 1 {
		t.Fatal("blur must restore draft without notification")
	}
}

func TestOutlineVirtualEndAndShrink(t *testing.T) {
	roots := make([]int, 10000)
	for i := range roots {
		roots[i] = i
	}
	state := &OutlineState[int]{}
	cells := 0
	o := OutlineOptions[int]{ID: func(k int) string { return fmt.Sprint(k) }, Label: func(k int) string { return fmt.Sprint(k) }, Children: func(int) []int { return nil }, RowHeight: 40}
	cols := []OutlineColumn[int]{{Title: "Name"}, {Title: "Value", Cell: func(id string, k int) *Element { cells++; return Text(id, fmt.Sprint(k)) }}}
	s, _ := NewSurface(600, 400, NewTheme(false))
	defer s.Close()
	build := func() *Element { return OutlineTable("tree", state, roots, o, cols) }
	if err := s.Mount(build()); err != nil {
		t.Fatal(err)
	}
	if cells > 12 {
		t.Fatal("built offscreen cells", cells)
	}
	s.Focus("tree/item/0/select")
	s.Key(KeyEvent{Key: "End"})
	if err := s.Mount(build()); err != nil {
		t.Fatal(err)
	}
	if s.Focused() != "tree/item/9999/select" || state.Selected != 9999 {
		t.Fatal(s.Focused(), state.Selected)
	}
	roots = roots[:2]
	if err := s.Mount(build()); err != nil {
		t.Fatal(err)
	}
	if state.Scroll.Y != 0 {
		t.Fatal("stale scroll", state.Scroll.Y)
	}
}

func TestDiffCachedMappingInvalidatesAndRetainsSnapshot(t *testing.T) {
	s := &DiffState{Old: "a\nx\n", New: "a\ny\n", Unified: true}
	DiffView("d", s, 200, nil)
	previous := s.unifiedRows
	DiffView("d", s, 200, nil)
	if &previous[0] != &s.unifiedRows[0] {
		t.Fatal("mapping rebuilt")
	}
	s.New = "a\nx\n"
	DiffView("d", s, 200, nil)
	if len(s.unifiedRows) != 2 || len(previous) != 3 {
		t.Fatal("invalid mapping", s.unifiedRows, previous)
	}
}

func BenchmarkDiffViewCached(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := &DiffState{Old: strings.Repeat("same\n", n), New: strings.Repeat("same\n", n), Unified: true}
			DiffView("d", s, 280, nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				DiffView("d", s, 280, nil)
			}
		})
	}
}
