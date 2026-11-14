package baseui

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestNativeTextUTF16MarkedReplacement(t *testing.T) {
	s, _ := NewSurface(400, 200, NewTheme(false))
	defer s.Close()
	b := NewTextBuffer("A😀B")
	s.Mount(Input("input", "Text", b))
	s.Focus("input")
	selected := TextRange{3, 0}
	if err := s.EditNativeText("拼😀", TextRange{-1, 0}, &selected); err != nil {
		t.Fatal(err)
	}
	state, _ := s.NativeText()
	if state.Text != "A😀B拼😀" || state.Marked != (TextRange{4, 3}) || state.Selected != (TextRange{7, 0}) {
		t.Fatal(state)
	}
	if b.String() != "A😀B" {
		t.Fatal("composition committed early")
	}
	if _, ok := s.NativeTextRect(2); ok {
		t.Fatal("surrogate interior accepted")
	}
	if err := s.EditNativeText("bad", TextRange{2, 0}, nil); err == nil {
		t.Fatal("split surrogate edit accepted")
	}
	if err := s.EditNativeText("音", TextRange{5, 2}, nil); err != nil {
		t.Fatal(err)
	}
	if b.String() != "A😀B拼音" {
		t.Fatal(b.String())
	}
	b.Undo()
	if b.String() != "A😀B" {
		t.Fatal("composition was not one undo step")
	}
	b.Select(1, 2)
	selected = TextRange{1, 0}
	s.EditNativeText("中", TextRange{-1, 0}, &selected)
	state, _ = s.NativeText()
	if state.Text != "A中B" {
		t.Fatal(state)
	}
	s.CommitComposition()
	if b.String() != "A中B" {
		t.Fatal(b.String())
	}
}
func TestAccessibilityActionsRespectState(t *testing.T) {
	s, _ := NewSurface(400, 200, NewTheme(false))
	defer s.Close()
	calls := 0
	b := NewTextBuffer("old")
	mount := func(disabled, readonly bool) {
		s.Mount(Column("root", Button("button", "Press", func() { calls++ }).Disabled(disabled), Input("text", "Text", b).ReadOnly(readonly)))
	}
	mount(false, false)
	if err := s.AccessibilityAction("button", "press", ""); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	if err := s.AccessibilityAction("text", "value", "new"); err != nil || b.String() != "new" {
		t.Fatal(err)
	}
	mount(true, true)
	if s.AccessibilityAction("button", "press", "") == nil || s.AccessibilityAction("text", "value", "bad") == nil {
		t.Fatal("inaccessible mutation accepted")
	}
	s.Mount(Text("gone", "Gone"))
	if s.AccessibilityAction("button", "press", "") == nil {
		t.Fatal("stale action accepted")
	}
}
func TestIncrementalHighlightMatchesFreshScanner(t *testing.T) {
	sources := []string{"package main\nvar x = 1\n", "/* open\ninside\n*/ var x = `raw\ntext`\n", "var s = `hello\nworld`\n", "/*\r\nx\r\n*/\n", "// 😀\nvar 中 = 1i\n"}
	cache := codeEditorCache{}
	rng := rand.New(rand.NewSource(7))
	pieces := []string{"/*", "*/", "`", "\n", "x", "//", "\"", "中", "\r"}
	for _, source := range sources {
		for i := 0; i < 60; i++ {
			next := incrementalCode(cache, source, "go")
			fresh := incrementalCode(codeEditorCache{}, source, "go")
			if !reflect.DeepEqual(next.rows, fresh.rows) || !reflect.DeepEqual(next.states, fresh.states) {
				t.Fatalf("incremental divergence: %q", source)
			}
			cache = next
			runes := []rune(source)
			at := rng.Intn(len(runes) + 1)
			if len(runes) > 0 && rng.Intn(2) == 0 {
				end := min(len(runes), at+1)
				source = string(runes[:at]) + string(runes[end:])
			} else {
				source = string(runes[:at]) + pieces[rng.Intn(len(pieces))] + string(runes[at:])
			}
		}
	}
	source := strings.Repeat("var x = 1\n", 10000)
	cache = incrementalCode(codeEditorCache{}, source, "go")
	next := incrementalCode(cache, "var x = 2\n"+source[len("var x = 1\n"):], "go")
	if next.lexed != 1 {
		t.Fatalf("small edit relexed %d lines", next.lexed)
	}
}
func TestIncrementalOutlineMatchesFullAndRetainsSnapshots(t *testing.T) {
	state := &OutlineState[int]{Open: map[int]bool{}}
	reads := 0
	children := func(k int) []int {
		reads++
		if k < 40 {
			return []int{k*3 + 1, k*3 + 2, k*3 + 3}
		}
		return nil
	}
	o := OutlineOptions[int]{ID: func(k int) string { return fmt.Sprint(k) }, Label: func(k int) string { return fmt.Sprint(k) }, Children: children, Incremental: true}
	roots := []int{0}
	state.indexRows(roots, o)
	reads = 0
	state.indexRows(roots, o)
	if reads != 0 {
		t.Fatal("unchanged tree traversed")
	}
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 100; i++ {
		k := rng.Intn(40)
		state.Open[k] = !state.Open[k]
		old := append([]outlineEntry[int](nil), state.index.rows...)
		held := state.index.rows
		got := state.indexRows(roots, o)
		want := state.fullRows(roots, o)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("index mismatch on toggle %d", k)
		}
		if !reflect.DeepEqual(old, held) {
			t.Fatal("old mounted index mutated")
		}
	}
	o.Revision++
	reads = 0
	state.indexRows(roots, o)
	if reads == 0 {
		t.Fatal("model revision ignored")
	}
}

func TestAccessibleSelectionReadOnlyAndUTF16(t *testing.T) {
	s, _ := NewSurface(300, 100, NewTheme(false))
	defer s.Close()
	b := NewTextBuffer("A😀B")
	s.Mount(Input("input", "Text", b).ReadOnly(true))
	if err := s.SelectAccessibleText("input", TextRange{1, 2}); err != nil {
		t.Fatal(err)
	}
	a, c := b.Selection()
	if a != 1 || c != 2 {
		t.Fatal(a, c)
	}
	if s.SelectAccessibleText("input", TextRange{2, 1}) == nil {
		t.Fatal("split-surrogate selection accepted")
	}
	if s.AccessibilityAction("input", "value", "changed") == nil {
		t.Fatal("read-only edit accepted")
	}
}
func BenchmarkIncrementalHighlightEdit(b *testing.B) {
	source := strings.Repeat("var x = 1\n", 10000)
	changed := "var x = 2\n" + source[len("var x = 1\n"):]
	cache := incrementalCode(codeEditorCache{}, source, "go")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		next := source
		if i%2 == 0 {
			next = changed
		}
		cache = incrementalCode(cache, next, "go")
	}
}
func BenchmarkIncrementalOutlineWarm(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			roots := make([]int, n)
			for i := range roots {
				roots[i] = i
			}
			state := &OutlineState[int]{}
			o := OutlineOptions[int]{ID: func(k int) string { return fmt.Sprint(k) }, Label: func(k int) string { return fmt.Sprint(k) }, Children: func(int) []int { return nil }, RowHeight: 32, Height: 200, Incremental: true}
			TreeOutline("tree", state, roots, o)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				TreeOutline("tree", state, roots, o)
			}
		})
	}
}

func TestIncrementalOutlineLocalModelChanges(t *testing.T) {
	model := map[int][]int{0: {1, 2}, 1: {3, 4}, 2: {5, 6}}
	calls := map[int]int{}
	state := &OutlineState[int]{Open: map[int]bool{0: true, 1: true, 2: true}}
	o := OutlineOptions[int]{ID: func(k int) string { return fmt.Sprint(k) }, Label: func(k int) string { return fmt.Sprint(k) }, Children: func(k int) []int { calls[k]++; return model[k] }, Incremental: true}
	state.indexRows([]int{0}, o)
	clear(calls)
	model[1] = []int{4, 7, 3}
	state.Invalidate(1)
	got := state.indexRows([]int{0}, o)
	if calls[0] != 0 || calls[2] != 0 || calls[5] != 0 || calls[6] != 0 {
		t.Fatal("unrelated subtree fetched", calls)
	}
	if !reflect.DeepEqual(got, state.fullRows([]int{0}, o)) {
		t.Fatal("local insert/reorder not indexed")
	}
	model[1] = nil
	state.Invalidate(1)
	got = state.indexRows([]int{0}, o)
	if !reflect.DeepEqual(got, state.fullRows([]int{0}, o)) {
		t.Fatal("local removal not indexed")
	}
}

func TestIncrementalLineTokensMatchWholeDocumentScanner(t *testing.T) {
	for _, source := range []string{
		"package main\nvar x = 123.4\n",
		"/* comment\ncontinued */ var x = `raw\ntext`\n",
		"var s = \"quoted\\\"text\" // note\nvar 中 = 42\n",
		"/* outer\n// inside comment\n*/ // outside\n",
	} {
		got := incrementalCode(codeEditorCache{}, source, "go").rows
		want := codeLines(source, "go")
		if len(got) != len(want) {
			t.Fatal("line count")
		}
		for i := range got {
			if !slices.Equal(got[i], want[i]) {
				t.Fatalf("line %d token mismatch for %q: %+v vs %+v", i, source, got[i], want[i])
			}
		}
	}
}
