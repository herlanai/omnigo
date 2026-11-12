package render

import (
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"testing"
)

func TestBundledChineseFaces(t *testing.T) {
	for _, bold := range []bool{false, true} {
		f := newStyledFace(18, bold)
		for _, r := range "中文简体繁體搜索，。！？你好世界" {
			if _, ok := f.GlyphAdvance(r); !ok {
				t.Fatalf("missing glyph %U bold=%v", r, bold)
			}
		}
		if f.Metrics().Height < f.Metrics().Ascent+f.Metrics().Descent {
			t.Fatal("inconsistent line metrics")
		}
		f.Close()
	}
}

func TestChineseBoldChangesActualInk(t *testing.T) {
	ink := func(bold bool) uint64 {
		face := newStyledFace(24, bold)
		defer face.Close()
		dst := image.NewRGBA(image.Rect(0, 0, 240, 60))
		d := font.Drawer{Dst: dst, Src: image.NewUniform(color.White), Face: face, Dot: fixed.P(2, 35)}
		d.DrawString("中文搜索 ABC")
		var sum uint64
		for i := 3; i < len(dst.Pix); i += 4 {
			sum += uint64(dst.Pix[i])
		}
		return sum
	}
	regular, bold := ink(false), ink(true)
	if regular == 0 || bold <= regular {
		t.Fatal("regular and bold glyphs not differentiated", regular, bold)
	}
}
