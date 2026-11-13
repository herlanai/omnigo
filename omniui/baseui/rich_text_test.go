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

func TestRichTextAlignmentTracksLinkHitRegions(t *testing.T) {
	for _, align := range []TextAlignment{TextAlignCenter, TextAlignRight} {
		s, _ := NewSurface(240, 100, NewTheme(false))
		opened := 0
		root := RichText("aligned", []TextSpan{{Text: "Open", Link: "/docs"}}, RichTextOptions{Size: 14, LineHeight: 1.5, Align: align, OnLink: func(string) { opened++ }})
		mount(t, s, Column("root", root))
		b, _ := s.Bounds("aligned")
		if b.H != 21 {
			t.Fatalf("line height: %v", b)
		}
		s.Pointer(PointerEvent{Kind: "down", X: 2, Y: 10})
		s.Pointer(PointerEvent{Kind: "up", X: 2, Y: 10})
		if opened != 0 {
			t.Fatal("link hit at old alignment position")
		}
		x := 120.
		if align == TextAlignRight {
			x = 230
		}
		s.Pointer(PointerEvent{Kind: "down", X: x, Y: 10})
		s.Pointer(PointerEvent{Kind: "up", X: x, Y: 10})
		if opened != 1 {
			t.Fatal("aligned link did not open")
		}
		s.Close()
	}
}

func TestRichTextNoWrapAndCodeIndentation(t *testing.T) {
	s, _ := NewSurface(120, 400, NewTheme(false))
	defer s.Close()
	scroll := ScrollState{}
	mount(t, s, ScrollArea("scroll", &scroll, RichText("code", []TextSpan{{Text: strings.Repeat("x", 80) + "\n    indented", Monospace: true}}, RichTextOptions{Size: 12, NoWrap: true, PreserveWhitespace: true})).HorizontalScroll(true))
	x, _ := scroll.Limits()
	if x <= 0 {
		t.Fatal("unwrapped line has no horizontal extent")
	}
	if got := expandCodeTabs("a\tb\r\n\tc"); got != "a   b\n    c" {
		t.Fatal(got)
	}
}
