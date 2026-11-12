package baseui

import (
	"image"
	"image/color"
	"math"
)

// Snapshot copies already painted pixels into a logical-resolution image.
// It is bounded and independent of this transient Painter's lifetime.
func (p *Painter) Snapshot(r Rect) *image.NRGBA {
	if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || r.W <= 0 || r.H <= 0 || r.W > 4096 || r.H > 4096 || r.W*r.H > 4194304 {
		return nil
	}
	out := image.NewNRGBA(image.Rect(0, 0, int(math.Ceil(r.W)), int(math.Ceil(r.H))))
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	for y := 0; y < out.Bounds().Dy(); y++ {
		for x := 0; x < out.Bounds().Dx(); x++ {
			lx, ly := r.X+float64(x)+.5, r.Y+float64(y)+.5
			if lx < p.clip.X || ly < p.clip.Y || lx >= p.clip.X+p.clip.W || ly >= p.clip.Y+p.clip.H {
				continue
			}
			out.Set(x, y, p.image.At(int(lx*scale), int(ly*scale)))
		}
	}
	return out
}

// Image paints an application-owned bitmap with clipping and rounded edges.
func (p *Painter) Image(r Rect, src image.Image, radius float64) {
	if src == nil || r.W <= 0 || r.H <= 0 || !finite(r.W) || !finite(r.H) || !finite(r.X) || !finite(r.Y) || !finite(radius) {
		return
	}
	b := src.Bounds()
	if b.Empty() {
		return
	}
	radius = max(0, min(radius, min(r.W, r.H)/2))
	p.rasterShade(r, func(x, y float64) float64 {
		qx := math.Abs(x-(r.X+r.W/2)) - (r.W/2 - radius)
		qy := math.Abs(y-(r.Y+r.H/2)) - (r.H/2 - radius)
		return math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - radius
	}, func(x, y float64) color.NRGBA {
		sx := b.Min.X + min(b.Dx()-1, max(0, int((x-r.X)/r.W*float64(b.Dx()))))
		sy := b.Min.Y + min(b.Dy()-1, max(0, int((y-r.Y)/r.H*float64(b.Dy()))))
		return color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA)
	})
}
