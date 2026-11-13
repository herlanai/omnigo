package baseui

import (
	"testing"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func TestFallbackKeepsPrimaryLatinAndClosesOwnedFaces(t *testing.T) {
	primary, err := opentype.NewFace(interFonts[400](), &opentype.FaceOptions{Size: 14, DPI: 72})
	if err != nil {
		t.Fatal(err)
	}
	f := withSystemFallback(primary, 14).(*fallbackFace)
	if f.face('A') != primary || f.initialized {
		t.Fatal("Latin text loaded fallback")
	}
	if f.Kern('A', 'V') != primary.Kern('A', 'V') {
		t.Fatal("Latin kerning changed")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInstalledSystemFontRendersChinese(t *testing.T) {
	if systemFallbackFont() == nil {
		t.Skip("no supported system fallback font installed")
	}
	for _, size := range []float64{14, 28} {
		for _, weight := range []int{400, 500, 600, 700} {
			f := weightedFace(weight, false, size)
			for _, r := range "简体中文繁體檔案日本語" {
				advance, ok := f.GlyphAdvance(r)
				if !ok || advance <= 0 {
					t.Fatalf("missing glyph %c, size %v weight %d", r, size, weight)
				}
				bounds, mask, _, _, ok := f.Glyph(fixed.P(0, 40), r)
				if !ok || mask == nil || bounds.Empty() {
					t.Fatalf("missing raster %c", r)
				}
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
