package icon

import (
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestInvalidSVGAndRasterLimits(t *testing.T) {
	for _, svg := range []string{"", `<svg/>`, `<svg><script/></svg>`, `<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="url(https://example.test/x)"/></svg>`, `<svg><image href="https://example.test/x"/></svg>`, `<!DOCTYPE svg><svg/>`, `<svg viewBox="0 0 24 24"><path d="M0 0L1e99 2"/></svg>`, `<svg viewBox="0 0 24 24"><path transform="scale(2)" d="M1 1L2 2"/></svg>`, `<svg/><svg/>`, strings.Repeat("x", MaxSVGBytes+1)} {
		if _, e := ParseSVG([]byte(svg)); e == nil {
			t.Fatalf("accepted invalid SVG %.100s", svg)
		}
	}
	v := MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 12H20"/></svg>`))
	for _, size := range [][2]int{{0, 24}, {2049, 24}, {2048, 2048}} {
		if _, e := v.Rasterize(size[0], size[1], color.NRGBA{}, 2); e == nil {
			t.Fatal("accepted size", size)
		}
	}
	if _, e := v.Rasterize(24, 24, color.NRGBA{}, math.NaN()); e == nil {
		t.Fatal("NaN")
	}
}
func TestStrokeScaleAndTint(t *testing.T) {
	v := MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M4 12H20"/></svg>`))
	for _, size := range []int{24, 48, 96} {
		img, e := v.Rasterize(size, size, color.NRGBA{255, 0, 0, 128}, 0)
		if e != nil {
			t.Fatal(e)
		}
		_, _, _, a := img.At(size/2, size/2).RGBA()
		if a < 30000 || a > 34000 {
			t.Fatalf("alpha: %d", a)
		}
		_, _, _, edge := img.At(size/2, size/2+size/12).RGBA()
		if edge != 0 {
			t.Fatal("stroke too thick")
		}
	}
	thin, _ := v.Rasterize(48, 48, color.NRGBA{255, 0, 0, 255}, 1)
	thick, _ := v.Rasterize(48, 48, color.NRGBA{255, 0, 0, 255}, 4)
	_, _, _, a := thin.At(24, 27).RGBA()
	_, _, _, b := thick.At(24, 27).RGBA()
	if a >= b {
		t.Fatal("stroke width ignored")
	}
}

func TestCompactArcFlags(t *testing.T) {
	compact := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 20a10 10 0 0010-10"/></svg>`
	expanded := strings.Replace(compact, "0 0010-10", "0 0 0 10 -10", 1)
	a := MustParseSVG([]byte(compact))
	b := MustParseSVG([]byte(expanded))
	ia, _ := a.Rasterize(48, 48, color.NRGBA{255, 0, 0, 255}, 0)
	ib, _ := b.Rasterize(48, 48, color.NRGBA{255, 0, 0, 255}, 0)
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			if ia.At(x, y) != ib.At(x, y) {
				t.Fatal("arc normalization changed geometry")
			}
		}
	}
	for _, path := range []string{"M1", "M1 1 A2 2 0 2 0 4 4", "M1 1Z2", "L1 1"} {
		if _, e := normalizePath(path); e == nil {
			t.Fatal(path)
		}
	}
}
