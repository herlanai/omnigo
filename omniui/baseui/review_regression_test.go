package baseui

import (
	"image/color"
	"testing"
)

func TestInputGroupSingleFrameCloneAndHitTesting(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(420, 140, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		buffer := NewTextBuffer("abcd")
		input := Input("input", "Name", buffer)
		view := func() *Element {
			return Column("root", InputGroup("group", Text("prefix", "https://"), input, Button("suffix", "Go", nil)).Width(360))
		}
		mount(t, s, view())
		if input.groupedInput || input.grow != 0 {
			t.Fatal("composition mutated caller builder")
		}
		r, _ := s.Bounds("input")
		s.Pointer(PointerEvent{Kind: "down", X: r.X + 1, Y: r.Y + r.H/2})
		s.Pointer(PointerEvent{Kind: "up", X: r.X + 1, Y: r.Y + r.H/2})
		a, c := buffer.Selection()
		if a != 0 || c != 0 {
			t.Fatal("grouped input hit offset", a, c)
		}
		s.TextInput("X")
		if buffer.String() != "Xabcd" {
			t.Fatal(buffer.String())
		}
		// Compare a pixel in the former nested border location with the group fill.
		s.Focus("suffix")
		img, err := s.Render()
		if err != nil {
			t.Fatal(err)
		}
		got := img.RGBAAt(int(r.X+r.W-1), int(r.Y+r.H/2))
		want := img.RGBAAt(int(r.X+5), int(r.Y+4))
		if got != want {
			t.Fatal("nested input frame remains", got, want)
		}
		mount(t, s, view().ReadOnly(true))
		s.Focus("input")
		s.TextInput("bad")
		if buffer.String() != "Xabcd" {
			t.Fatal("readonly group edited")
		}
	}
}
func TestVerticalSeparatorAndMarkerAlignment(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Row("row", Text("left", "Left"), Separator("vertical", true), Button("right", "Right", nil)).Height(40), Marker("marker", "Today")))
	bar, _ := s.Bounds("vertical")
	row, _ := s.Bounds("row")
	if bar.W != 1 || bar.H != row.H {
		t.Fatal("vertical separator geometry", bar, row)
	}
	line, _ := s.Bounds("marker/before")
	text, _ := s.Bounds("marker/text")
	if line.Y+line.H/2 != text.Y+text.H/2 {
		t.Fatal("marker not centered", line, text)
	}
}
func TestLinkVariantRemainsInteractiveAndTransparent(t *testing.T) {
	s := surface(t)
	calls := 0
	mount(t, s, Column("root", Button("link", "Open docs", func() { calls++ }).Variant(Link).Width(160)))
	img, err := s.Render()
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(4, 4); got != color.RGBAModel.Convert(s.theme.Background).(color.RGBA) {
		t.Fatal("link has filled frame", got)
	}
	click(t, s, "link")
	if calls != 1 {
		t.Fatal(calls)
	}
	s.Focus("link")
	s.Key(KeyEvent{Key: "Enter"})
	if calls != 2 {
		t.Fatal("link keyboard activation")
	}
	mount(t, s, Button("link", "Open docs", func() { calls++ }).Variant(Link).Disabled(true))
	click(t, s, "link")
	if calls != 2 {
		t.Fatal("disabled link activated")
	}
}
func TestSourcesSkipEmptyAndFallbackToHost(t *testing.T) {
	s := surface(t)
	mount(t, s, Sources("sources", []Source{{URL: ""}, {URL: "https://example.com/guide"}, {URL: "https://example.com/guide"}}, &DisclosureState{Open: true}, nil))
	if _, ok := s.Bounds("sources/source/1"); ok {
		t.Fatal("empty/duplicate source counted")
	}
	if got := s.entries[s.index["sources/source/0/title"]].element.text; got != "example.com" {
		t.Fatal(got)
	}
	mount(t, s, Context("context", ContextUsage{}))
	if got := s.entries[s.index["context/anchor"]].element.text; got != "Context · unknown limit" {
		t.Fatal("unknown limit hidden", got)
	}
}

func TestMessageAuthorTracksEndAlignmentAndEmptyHeader(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Message("message", "You", true, Bubble("bubble", "Hello"))).Width(300))
	author, _ := s.Bounds("message/author")
	body, _ := s.Bounds("message/body")
	if author.X+author.W != body.X+body.W {
		t.Fatal("author not aligned to outgoing content", author, body)
	}
	mount(t, s, Message("message", "", false, Bubble("bubble", "Hello")))
	if _, ok := s.Bounds("message/author"); ok {
		t.Fatal("empty author takes space")
	}
}
func TestInputGroupKeepsValidationAndTallEditor(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", InputGroup("group", nil, Textarea("input", "Notes", NewTextBuffer("line one\nline two")).Invalid(true), nil)))
	group := s.entries[s.index["group"]]
	input := s.entries[s.index["input"]]
	if !group.element.invalid || group.bounds.H < input.bounds.H {
		t.Fatal("group lost error or clipped editor", group.bounds, input.bounds)
	}
}
