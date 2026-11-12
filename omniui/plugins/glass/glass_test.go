package glass

import (
	"image"
	"image/color"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
)

func TestBlurOwnsPixelsAndSpreadsImpulse(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 5, 5))
	src.SetNRGBA(2, 2, color.NRGBA{255, 255, 255, 255})
	out := Blur(src, 1)
	if out.NRGBAAt(1, 2).A == 0 || out.NRGBAAt(2, 2).A == 255 {
		t.Fatal("not blurred")
	}
	out.Set(2, 2, color.Transparent)
	if src.NRGBAAt(2, 2).A != 255 {
		t.Fatal("aliased source")
	}
}
func TestGlassPreservesChildInteractionAndMotion(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, _ := ui.NewSurface(240, 120, ui.NewTheme(dark))
		clicked := false
		root := Box("glass", Options{Refraction: 3}, ui.Button("button", "Save", func() { clicked = true }))
		if e := s.Mount(root); e != nil {
			t.Fatal(e)
		}
		if _, e := s.RenderScaled(2, nil); e != nil {
			t.Fatal(e)
		}
		s.Focus("button")
		s.Key(ui.KeyEvent{Key: "Enter"})
		if !clicked || s.NeedsAnimation() {
			t.Fatal("glass input or scheduling")
		}
		s.Close()
	}
}
