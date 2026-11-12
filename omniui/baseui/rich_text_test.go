package baseui

import (
	"bytes"
	"strings"
	"testing"
)

func TestRichTextWrapStyleSnapshotAndLinks(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, _ := NewSurface(180, 480, NewTheme(dark))
		defer s.Close()
		state := RichTextState{}
		opened := ""
		spans := []TextSpan{{Text: "Heading ", Bold: true}, {Text: "first", Link: "https://one.test"}, {Text: " second", Link: "https://two.test", Italic: true}, {Text: " inline code", Code: true}}
		build := func() *Element {
			return RichText("rich", spans, RichTextOptions{Size: 18, State: &state, OnLink: func(v string) { opened = v }})
		}
		mount(t, s, build())
		b, _ := s.Bounds("rich")
		if b.H < 40 {
			t.Fatal("no wrapping", b)
		}
		a, e := s.RenderScaled(2, nil)
		if e != nil {
			t.Fatal(e)
		}
		spans[0].Text = "MUTATED"
		again, _ := s.RenderScaled(2, nil)
		if !bytes.Equal(a.Pix, again.Pix) {
			t.Fatal("caller mutated mounted runs")
		}
		if !s.Focus("rich") {
			t.Fatal("link paragraph not focusable")
		}
		s.Key(KeyEvent{Key: "Right"})
		s.Key(KeyEvent{Key: "Enter"})
		if opened != "https://two.test" {
			t.Fatal(opened)
		}
		opened = ""
		mount(t, s, build().Disabled(true))
		if s.Focus("rich") {
			t.Fatal("disabled focused")
		}
		mount(t, s, build().ReadOnly(true))
		s.Focus("rich")
		s.Key(KeyEvent{Key: "Enter"})
		if opened != "" {
			t.Fatal("read-only link activated")
		}
	}
}
func TestRichTextLongWordAndCanceledPointer(t *testing.T) {
	s, _ := NewSurface(100, 120, NewTheme(false))
	defer s.Close()
	called := 0
	mount(t, s, RichText("rich", []TextSpan{{Text: strings.Repeat("w", 4000), Link: "https://example.test"}}, RichTextOptions{OnLink: func(string) { called++ }}))
	s.Pointer(PointerEvent{Kind: "down", X: 2, Y: 2})
	s.Pointer(PointerEvent{Kind: "cancel"})
	s.Pointer(PointerEvent{Kind: "up", X: 2, Y: 2})
	if called != 0 {
		t.Fatal("canceled click")
	}
	if _, e := s.Render(); e != nil {
		t.Fatal(e)
	}
}
