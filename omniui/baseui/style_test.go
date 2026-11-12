package baseui

import (
	"bytes"
	"image/color"
	"testing"
)

func TestMenuSelectionPaletteAcrossStyles(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, style := range []VisualStyle{StyleMaia, StyleNova, StyleLyra} {
			for _, bold := range []bool{false, true} {
				theme := ThemePreset(dark, style, AccentBlue, bold)
				s, err := NewSurface(240, 120, theme)
				if err != nil {
					t.Fatal(err)
				}
				item := Button("item", "Selected", nil).Variant(Ghost).Role("menuitem")
				item.selected = true
				mount(t, s, Column("root", item).Padding(16))
				r, _ := s.Bounds("item")
				// Hover must not replace the selected palette with the generic variant.
				s.Pointer(PointerEvent{Kind: "move", X: r.X + r.W/2, Y: r.Y + 5})
				img, err := s.Render()
				if err != nil {
					t.Fatal(err)
				}
				want := theme.Hover
				if bold {
					want = theme.Accent
				}
				got := color.NRGBAModel.Convert(img.At(int(r.X+r.W/2), int(r.Y+5)))
				if got != want {
					t.Errorf("dark=%t style=%s bold=%t: selected fill %v, want %v", dark, style, bold, got, want)
				}
				s.Close()
			}
		}
	}
}

func TestThemeSwitchRetainsStateAndReleasesFonts(t *testing.T) {
	s, err := NewSurface(400, 240, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	buffer := NewTextBuffer("Inter sample")
	buffer.Select(2, 5)
	calls := 0
	view := func() *Element {
		return Column("root", Input("input", "Name", buffer), Button("button", "Save", func() { calls++ }), Heading("title", "Heading", 3)).Padding(12)
	}
	mount(t, s, view())
	s.Focus("input")
	before, err := s.RenderScaled(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, style := range []VisualStyle{StyleNova, StyleLyra, StyleMaia} {
		if err = s.SetTheme(ThemePreset(true, style, AccentBlue, true)); err != nil {
			t.Fatal(err)
		}
		if len(s.rasterFaces) != 0 {
			t.Fatal("old font cache retained")
		}
		if s.focus != "input" || buffer.String() != "Inter sample" {
			t.Fatal("theme reset editing state")
		}
		a, b := buffer.Selection()
		if a != 2 || b != 5 {
			t.Fatal("selection reset")
		}
		after, err := s.RenderScaled(2, nil)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(before.Pix, after.Pix) {
			t.Fatal("appearance did not change")
		}
	}
	r, _ := s.Bounds("button")
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 10, Y: r.Y + 10})
	if err = s.SetTheme(NewTheme(false)); err != nil {
		t.Fatal(err)
	}
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 10, Y: r.Y + 10})
	if calls != 0 {
		t.Fatal("theme change committed pending press")
	}
	s.Close()
	if s.SetTheme(NewTheme(false)) == nil {
		t.Fatal("closed theme switch accepted")
	}
}
func TestInterWeightsAreDistinctAndStable(t *testing.T) {
	s, err := NewSurface(400, 200, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	faces := map[any]bool{}
	for _, weight := range []int{400, 500, 600, 700} {
		f := s.elementFace(Text("text", "Typography").FontWeight(weight))
		if faces[f] {
			t.Fatal("different weights share a face")
		}
		faces[f] = true
		if s.elementFace(Text("text", "Typography").FontWeight(weight)) != f {
			t.Fatal("font cache unstable")
		}
	}
	for _, style := range []VisualStyle{StyleMaia, StyleNova, StyleLyra} {
		theme := ThemePreset(false, style, AccentNeutral, false)
		button := Button("b", "Button", nil)
		expected := map[VisualStyle]float64{StyleMaia: 32, StyleNova: 8, StyleLyra: 0}[style]
		if controlRadius(theme, button) != expected {
			t.Fatal("wrong radius", style)
		}
	}
}
