package baseui

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"testing"
)

// Pre-optimization rasterizer is a pixel oracle for fractional clips, DPI,
// rounded corners and premultiplied blending. It is intentionally unoptimized.
func (p *Painter) referenceRasterShade(r Rect, distance func(float64, float64) float64, shade func(float64, float64) color.NRGBA) {
	if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || r.W <= 0 || r.H <= 0 {
		return
	}
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	area := intersect(r.inset(-1/scale), p.clip)
	for y := max(0, int(math.Floor(area.Y*scale))); y < min(p.image.Bounds().Dy(), int(math.Ceil((area.Y+area.H)*scale))); y++ {
		for x := max(0, int(math.Floor(area.X*scale))); x < min(p.image.Bounds().Dx(), int(math.Ceil((area.X+area.W)*scale))); x++ {
			px, py := (float64(x)+.5)/scale, (float64(y)+.5)/scale
			// Fractional clip coverage prevents painting beyond an ancestor's edge.
			cx := max(0, min(float64(x+1), (p.clip.X+p.clip.W)*scale)-max(float64(x), p.clip.X*scale))
			cy := max(0, min(float64(y+1), (p.clip.Y+p.clip.H)*scale)-max(float64(y), p.clip.Y*scale))
			c := shade(px, py)
			alpha := float64(c.A) / 255 * max(0, min(1, .5-distance(px, py)*scale)) * cx * cy
			if alpha == 0 {
				continue
			}
			at := p.image.PixOffset(x, y)
			for i, v := range [3]uint8{c.R, c.G, c.B} {
				p.image.Pix[at+i] = uint8(math.Round(float64(v)*alpha + float64(p.image.Pix[at+i])*(1-alpha)))
			}
			p.image.Pix[at+3] = uint8(math.Round(255*alpha + float64(p.image.Pix[at+3])*(1-alpha)))
		}
	}
}
func referenceRoundedDistance(r Rect, radius, x, y float64) float64 {
	radius = max(0, min(radius, min(r.W, r.H)/2))
	dx := math.Abs(x-r.X-r.W/2) - r.W/2 + radius
	dy := math.Abs(y-r.Y-r.H/2) - r.H/2 + radius
	return math.Hypot(max(dx, 0), max(dy, 0)) + min(max(dx, dy), 0) - radius
}

func TestOptimizedRasterMatchesReference(t *testing.T) {
	random := rand.New(rand.NewPCG(17, 23))
	for _, scale := range []float64{1, 1.25, 2, 3} {
		for n := 0; n < 40; n++ {
			actual, expected := image.NewRGBA(image.Rect(0, 0, 96, 96)), image.NewRGBA(image.Rect(0, 0, 96, 96))
			for i := range actual.Pix {
				actual.Pix[i] = uint8(random.IntN(256))
			}
			copy(expected.Pix, actual.Pix)
			clip := Rect{random.Float64() * 5, random.Float64() * 5, 20 + random.Float64()*60, 20 + random.Float64()*60}
			r := Rect{random.Float64()*20 - 5, random.Float64()*20 - 5, random.Float64() * 50, random.Float64() * 50}
			radius, width := random.Float64()*18, random.Float64()*5+.1
			c := color.NRGBA{uint8(random.IntN(256)), uint8(random.IntN(256)), uint8(random.IntN(256)), uint8(random.IntN(256))}
			if n%2 == 0 {
				c.A = 255
			}
			p := Painter{image: actual, clip: clip, scale: scale}
			q := Painter{image: expected, clip: clip, scale: scale}
			if n%3 == 0 {
				p.Stroke(r, c, radius, width)
				inner := r.inset(width)
				q.referenceRasterShade(r, func(x, y float64) float64 {
					d := referenceRoundedDistance(r, radius, x, y)
					if inner.W == 0 || inner.H == 0 {
						return d
					}
					return max(d, -referenceRoundedDistance(inner, max(0, radius-width), x, y))
				}, func(float64, float64) color.NRGBA { return c })
			} else {
				p.Fill(r, c, radius)
				q.referenceRasterShade(r, func(x, y float64) float64 { return referenceRoundedDistance(r, radius, x, y) }, func(float64, float64) color.NRGBA { return c })
			}
			if !bytes.Equal(actual.Pix, expected.Pix) {
				t.Fatalf("pixels differ: scale %v case %d", scale, n)
			}
		}
	}
}
func TestRenderScaledIntoOwnershipAndValidation(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(180, 90, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		for _, scale := range []float64{1, 1.25, 2} {
			mount(t, s, Button("button", "First frame", nil))
			snapshot, err := s.RenderScaled(scale, nil)
			if err != nil {
				t.Fatal(err)
			}
			preserved := append([]byte(nil), snapshot.Pix...)
			dst := image.NewRGBA(snapshot.Bounds())
			if err = s.RenderScaledInto(dst, scale, nil); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(dst.Pix, snapshot.Pix) {
				t.Fatal("reused output differs")
			}
			mount(t, s, Text("second", "Second frame"))
			if err = s.RenderScaledInto(dst, scale, nil); err != nil {
				t.Fatal(err)
			}
			fresh, err := s.RenderScaled(scale, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(dst.Pix, fresh.Pix) {
				t.Fatal("stale pixels in reused frame")
			}
			if !bytes.Equal(snapshot.Pix, preserved) {
				t.Fatal("snapshot was reused")
			}
		}
		for _, dst := range []*image.RGBA{nil, image.NewRGBA(image.Rect(0, 0, 1, 1)), {Rect: image.Rect(0, 0, 180, 90), Stride: 720}} {
			if err = s.RenderScaledInto(dst, 1, nil); err == nil {
				t.Fatal("invalid target accepted")
			}
		}
		s.Close()
		if err = s.RenderScaledInto(image.NewRGBA(image.Rect(0, 0, 180, 90)), 1, nil); err == nil {
			t.Fatal("closed surface rendered")
		}
	}
}

func TestRasterInteriorSpansAtFractionalBounds(t *testing.T) {
	for _, scale := range []float64{.75, 1, 1.25, 2, 3} {
		for _, radius := range []float64{0, 1, 8, 40, 100} {
			for _, width := range []float64{0, .25, 1, 12, 100} {
				a, b := image.NewRGBA(image.Rect(0, 0, 180, 150)), image.NewRGBA(image.Rect(0, 0, 180, 150))
				p := Painter{image: a, clip: Rect{2.3, 1.75, 135.25, 130.5}, scale: scale}
				q := p
				q.image = b
				r := Rect{4.25, 5.75, 130.5, 120.25}
				c := color.NRGBA{27, 49, 73, 255}
				if width == 0 {
					p.Fill(r, c, radius)
					q.referenceRasterShade(r, func(x, y float64) float64 { return referenceRoundedDistance(r, radius, x, y) }, func(float64, float64) color.NRGBA { return c })
				} else {
					p.Stroke(r, c, radius, width)
					inner := r.inset(width)
					q.referenceRasterShade(r, func(x, y float64) float64 {
						d := referenceRoundedDistance(r, radius, x, y)
						if inner.W == 0 || inner.H == 0 {
							return d
						}
						return max(d, -referenceRoundedDistance(inner, max(0, radius-width), x, y))
					}, func(float64, float64) color.NRGBA { return c })
				}
				if !bytes.Equal(a.Pix, b.Pix) {
					t.Fatalf("span changed pixels: scale=%g radius=%g width=%g", scale, radius, width)
				}
			}
		}
	}
}
func TestBackingScaleFontCacheLifetime(t *testing.T) {
	s, err := NewSurface(240, 120, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	mount(t, s, Column("root", Heading("heading", "Heading", 3), Text("body", "Body")))
	first, err := s.RenderScaled(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	face := s.rasterFaces[textFaceKey{size: s.theme.FontSize}]
	if face == nil {
		t.Fatal("font not cached")
	}
	second, err := s.RenderScaled(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.rasterFaces[textFaceKey{size: s.theme.FontSize}] != face || !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("steady frame recreated font or changed pixels")
	}
	for _, scale := range []float64{1.25, 3, 2, 1.5} {
		if _, err = s.RenderScaled(scale, nil); err != nil {
			t.Fatal(err)
		}
		if len(s.rasterFaces) != 2 || s.rasterScale != scale {
			t.Fatal("stale scales accumulated")
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if len(s.rasterFaces) != 0 {
		t.Fatal("backing fonts retained on close")
	}
}
