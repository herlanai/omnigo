package baseui

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestTypographyMetricsDPIAndFontLifetime(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(360, 400, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		root := Column("root", Heading("title", "A clearer hierarchy", 2), Text("body", "Readable body text"), Text("note", "Supporting information").Role("note"), Badge("badge", "Preview")).Padding(12)
		if err = s.Mount(root); err != nil {
			t.Fatal(err)
		}
		title, body, note := s.entries[s.index["title"]], s.entries[s.index["body"]], s.entries[s.index["note"]]
		if title.bounds.H <= body.bounds.H || note.bounds.H > body.bounds.H {
			t.Fatal("font metrics not used by layout")
		}
		if s.entries[s.index["badge"]].bounds.H != 20 {
			t.Fatal("badge not compact")
		}
		for _, scale := range []float64{1, 1.25, 2} {
			if _, err = s.RenderScaled(scale, nil); err != nil {
				t.Fatal(err)
			}
		}
		if len(s.textFaces) == 0 {
			t.Fatal("styled font not created")
		}
		s.Close()
		if len(s.textFaces) != 0 {
			t.Fatal("styled fonts retained after close")
		}
	}
}
func TestFocusVisibleOutsideControlAndPointerFocus(t *testing.T) {
	s, _ := NewSurface(180, 90, NewTheme(false))
	defer s.Close()
	build := func() *Element {
		return Column("root", Row("row", Button("b", "Continue", func() {}).Variant(Outline))).Padding(14)
	}
	s.Mount(build())
	r := s.entries[s.index["b"]].bounds
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 10, Y: r.Y + 10})
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 10, Y: r.Y + 10})
	s.Mount(build())
	if s.focusVisible {
		t.Fatal("pointer enabled keyboard ring")
	}
	pointer, _ := s.Render()
	s.Key(KeyEvent{Key: "Tab"})
	keyboard, _ := s.Render()
	if !s.focusVisible || bytes.Equal(pointer.Pix, keyboard.Pix) {
		t.Fatal("keyboard focus not distinguished")
	}
	x, y := int(r.X-2), int(r.Y+r.H/2)
	if pointer.RGBAAt(x, y) == keyboard.RGBAAt(x, y) {
		t.Fatal("no outer ring")
	}
	s.Mount(build().Disabled(true))
	if s.Focus("b") {
		t.Fatal("disabled focus")
	}
}
func TestSizedInputHitTestingAndButtonMetrics(t *testing.T) {
	s, _ := NewSurface(300, 200, NewTheme(false))
	defer s.Close()
	buffer := NewTextBuffer("WWWW")
	input := Input("input", "Input", buffer).Size(ControlXS)
	s.Mount(Column("root", input, Button("small", "Action", func() {}).Size(ControlSmall), Button("large", "Action", func() {}).Size(ControlLarge)).Padding(12))
	entry := s.entries[s.index["input"]]
	metrics := s.textSurface(entry.element)
	x := entry.bounds.X + inputTextInset(entry.element) + metrics.textWidth("W") + .1
	s.Pointer(PointerEvent{Kind: "down", X: x, Y: entry.bounds.Y + 8})
	_, caret := buffer.Selection()
	if caret != 1 {
		t.Fatalf("styled input hit caret %d", caret)
	}
	if s.entries[s.index["small"]].bounds.H != 32 || s.entries[s.index["large"]].bounds.H != 40 {
		t.Fatal("control sizes ignored")
	}
}
func TestShadowClippingAndAlpha(t *testing.T) {
	pixels := image.NewRGBA(image.Rect(0, 0, 80, 80))
	p := Painter{image: pixels, clip: Rect{10, 10, 60, 60}, scale: 1}
	p.Shadow(Rect{25, 25, 30, 25}, 8, 4, 2, color.NRGBA{0, 0, 0, 40})
	if pixels.RGBAAt(9, 30).A != 0 {
		t.Fatal("shadow escaped clip")
	}
	if pixels.RGBAAt(22, 38).A == 0 || pixels.RGBAAt(22, 38).A > 40 {
		t.Fatal("shadow falloff invalid")
	}
}

func TestCompactBadgesAndLocalTypography(t *testing.T) {
	s, _ := NewSurface(400, 300, NewTheme(false))
	defer s.Close()
	s.Mount(Column("root", Badge("tag", "Preview"), Text("body", "Body"), Column("group", Text("child", "Body")).FontSize(28)).Padding(12))
	if s.entries[s.index["tag"]].bounds.W > 120 {
		t.Fatal("badge stretches across column")
	}
	if s.entries[s.index["body"]].bounds.H != s.entries[s.index["child"]].bounds.H {
		t.Fatal("local container font leaked into child measurement")
	}
}

func TestMinimumHeightRetainsTallContentAndHiddenLayout(t *testing.T) {
	s := surface(t)
	for _, tc := range []struct{ height, minimum, want float64 }{{36, 230, 230}, {280, 230, 280}} {
		root := Column("root", Button("button", "Action", nil).Height(tc.height)).MinHeight(tc.minimum)
		mount(t, s, root)
		r, _ := s.Bounds("root")
		if r.H < tc.want {
			t.Fatalf("minimum clipped content: %+v", r)
		}
		root.Hidden(true)
		mount(t, s, root)
		if _, ok := s.Bounds("button"); ok {
			t.Fatal("hidden subtree kept layout")
		}
	}
}
