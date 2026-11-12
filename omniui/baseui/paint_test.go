package baseui

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"reflect"
	"testing"

	"golang.org/x/image/font/basicfont"
)

func TestTranslucentLineCoverage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	p := Painter{image: img, clip: Rect{W: 40, H: 20}}
	p.Line(5, 10.5, 35, 10.5, 1, color.NRGBA{R: 255, A: 128})
	for x := 6; x < 34; x++ {
		if got := img.RGBAAt(x, 10); got != (color.RGBA{R: 128, A: 128}) {
			t.Fatalf("line accumulates alpha at %d: %v", x, got)
		}
	}
	if img.RGBAAt(20, 9).A != 0 {
		t.Fatal("one pixel stroke grew beyond its bounds")
	}
}
func TestRoundedStrokeAndFractionalClip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 30, 30))
	p := Painter{image: img, clip: Rect{W: 30, H: 30}}
	p.Stroke(Rect{2, 2, 24, 24}, color.NRGBA{R: 255, A: 255}, 8, 2)
	if img.RGBAAt(14, 14).A != 0 || img.RGBAAt(2, 2).A != 0 {
		t.Fatal("stroke filled interior or rounded corner")
	}
	if img.RGBAAt(14, 2).A != 255 {
		t.Fatal("stroke missing top edge")
	}
	p.clip = Rect{0, 0, 10.25, 30}
	p.Fill(Rect{0, 0, 30, 30}, color.NRGBA{G: 255, A: 255}, 0)
	if got := img.RGBAAt(10, 15); got.A < 63 || got.A > 64 {
		t.Fatal("fractional clip", got)
	}
	if img.RGBAAt(11, 15).A != 0 {
		t.Fatal("paint escaped clip")
	}
}
func TestScaledRenderKeepsLogicalGeometry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(180, 90, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		calls := 0
		mount(t, s, Column("root", Button("run", "Run", func() { calls++ }).Width(100)).Padding(8))
		before := s.Accessibility()
		normal, err := s.Render()
		if err != nil {
			t.Fatal(err)
		}
		for _, scale := range []float64{1, 1.25, 2, 3} {
			img, err := s.RenderScaled(scale, nil)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != int(math.Ceil(180*scale)) || img.Bounds().Dy() != int(math.Ceil(90*scale)) {
				t.Fatal(img.Bounds())
			}
			if !reflect.DeepEqual(before, s.Accessibility()) {
				t.Fatal("scaling changed logical geometry")
			}
			if scale == 1 && !bytes.Equal(normal.Pix, img.Pix) {
				t.Fatal("scale 1 changed rendering")
			}
		}
		click(t, s, "run")
		if calls != 1 {
			t.Fatal("physical scale changed hit testing")
		}
		for _, scale := range []float64{0, -1, 9, math.NaN(), math.Inf(1)} {
			if _, err := s.RenderScaled(scale, nil); err == nil {
				t.Fatal("invalid scale accepted", scale)
			}
		}
		s.Close()
		if _, err := s.RenderScaled(2, nil); err == nil {
			t.Fatal("closed surface rendered")
		}
	}
}
func TestScaledCustomFontAndBudget(t *testing.T) {
	s, err := NewSurfaceWithFont(20, 20, NewTheme(false), basicfont.Face7x13)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.RenderScaled(2, nil); err == nil {
		t.Fatal("custom font silently replaced")
	}
	if _, err := s.RenderScaled(2, basicfont.Face7x13); err != nil {
		t.Fatal(err)
	}
	huge, err := NewSurface(8000, 8000, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer huge.Close()
	if _, err := huge.RenderScaled(2, nil); err == nil {
		t.Fatal("pixel budget ignored")
	}
}
func TestEmptyInputPlaceholderDoesNotChangeValue(t *testing.T) {
	s := surface(t)
	b := NewTextBuffer("")
	mount(t, s, Input("name", "Name", b).Placeholder("Your name"))
	with, _ := s.Render()
	mount(t, s, Input("name", "Name", b))
	without, _ := s.Render()
	if bytes.Equal(with.Pix, without.Pix) {
		t.Fatal("placeholder not painted")
	}
	if b.String() != "" || s.Accessibility()[0].Value != "" {
		t.Fatal("placeholder became input value")
	}
}
