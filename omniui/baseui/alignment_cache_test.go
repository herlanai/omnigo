package baseui

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"strings"
	"testing"
)

func TestEditorSnapshotInvalidation(t *testing.T) {
	b := NewTextBuffer("package main\nvar x = 1")
	state := &CodeEditorState{Text: b}
	CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
	before := state.cache.rows
	if before[0][0].style != 1 {
		t.Fatal("missing Go keyword")
	}
	b.Select(0, 0)
	b.SetComposition("中文")
	CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
	if b.Composition() != "中文" {
		t.Fatal("rebuild changed composition")
	}
	b.CancelComposition()
	b.Set("// comment\nvar y = 2\n")
	CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
	if state.cache.lineCount != 3 || state.cache.rows[0][0].style != 3 {
		t.Fatal("stale syntax after edit")
	}
	if before[0][0].text != "package" {
		t.Fatal("old mounted syntax was mutated")
	}
	b.Undo()
	CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
	if state.cache.lineCount != 2 || state.cache.rows[0][0].style != 1 {
		t.Fatal("stale undo")
	}
	CodeEditor("editor", state, CodeEditorOptions{Language: "text"})
	if state.cache.rows[0][0].style != 0 {
		t.Fatal("language change retained Go syntax")
	}
	state.Text = NewTextBuffer("replacement")
	CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
	if state.cache.rows[0][0].text != "replacement" {
		t.Fatal("buffer replacement retained old cache")
	}
}
func TestOutlineLabelsOnlyBuiltForViewport(t *testing.T) {
	roots := make([]int, 10000)
	for i := range roots {
		roots[i] = i
	}
	calls := 0
	state := &OutlineState[int]{}
	TreeOutline("tree", state, roots, OutlineOptions[int]{ID: func(k int) string { return fmt.Sprint(k) }, Label: func(k int) string { calls++; return fmt.Sprint(k) }, Children: func(int) []int { return nil }, RowHeight: 32, Height: 200})
	if calls == 0 || calls > 12 {
		t.Fatalf("viewport called %d labels", calls)
	}
}
func TestCompositionCanceledOnFocusTransfer(t *testing.T) {
	a, b := NewTextBuffer("a"), NewTextBuffer("b")
	s, _ := NewSurface(400, 200, NewTheme(false))
	defer s.Close()
	if err := s.Mount(Column("root", Input("a", "A", a), Input("b", "B", b))); err != nil {
		t.Fatal(err)
	}
	s.Focus("a")
	s.SetComposition("拼音")
	s.Focus("b")
	if a.Composition() != "" || a.String() != "a" {
		t.Fatal("blur left stale composition or committed draft")
	}
	s.CommitComposition()
	if b.String() != "b" {
		t.Fatal("composition crossed input boundaries")
	}
	s.Focus("a")
	if a.Undo() {
		t.Fatal("canceled composition created undo entry")
	}
}
func BenchmarkCodeEditorWarmBuild(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			state := &CodeEditorState{Text: NewTextBuffer(strings.Repeat("var value = 42\n", n))}
			CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				CodeEditor("editor", state, CodeEditorOptions{Language: "go"})
			}
		})
	}
}

type viewportFace struct {
	font.Face
	hidden int
}

func (f *viewportFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	if r == 'Z' {
		f.hidden++
	}
	return f.Face.GlyphAdvance(r)
}
func TestTextPaintSkipsOffscreenLines(t *testing.T) {
	face := &viewportFace{Face: basicfont.Face7x13}
	s, err := NewSurfaceWithFont(300, 100, NewTheme(false), face)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	text := NewTextBuffer(strings.Repeat("visible\n", 20) + strings.Repeat("ZZZZZZ\n", 1000))
	scroll := &ScrollState{}
	if err = s.Mount(ScrollArea("scroll", scroll, Textarea("text", "Text", text).Height(15000)).Height(90)); err != nil {
		t.Fatal(err)
	}
	face.hidden = 0
	if _, err = s.Render(); err != nil {
		t.Fatal(err)
	}
	if face.hidden != 0 {
		t.Fatalf("measured %d offscreen glyphs", face.hidden)
	}
}
func TestCompositionCanceledWhenInputUnavailable(t *testing.T) {
	for _, mode := range []string{"removed", "disabled", "readonly", "replaced", "closed"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := NewSurface(300, 100, NewTheme(false))
			defer s.Close()
			b := NewTextBuffer("original")
			s.Mount(Input("input", "Input", b))
			s.Focus("input")
			s.SetComposition("pending")
			switch mode {
			case "removed":
				s.Mount(Text("label", "gone"))
			case "disabled":
				s.Mount(Input("input", "Input", b).Disabled(true))
			case "readonly":
				s.Mount(Input("input", "Input", b).ReadOnly(true))
			case "replaced":
				s.Mount(Input("input", "Input", NewTextBuffer("replacement")))
			case "closed":
				s.Close()
			}
			if b.Composition() != "" || b.String() != "original" {
				t.Fatal("invalidated input retained or committed composition")
			}
		})
	}
}
