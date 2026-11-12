package baseui

import (
	"image"
	"image/color"
	"math"
)

// wholePixels returns only pixels entirely inside the rectangle and the clip.
// Conservative rounding keeps fractional boundary coverage on the scalar path.
func (p *Painter) wholePixels(r Rect) image.Rectangle {
	if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || r.W <= 0 || r.H <= 0 {
		return image.Rectangle{}
	}
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	r = intersect(r, p.clip)
	r = intersect(r, Rect{W: float64(p.image.Bounds().Dx()) / scale, H: float64(p.image.Bounds().Dy()) / scale})
	if r.W <= 0 || r.H <= 0 {
		return image.Rectangle{}
	}
	left, top := int(math.Ceil(r.X*scale)), int(math.Ceil(r.Y*scale))
	right, bottom := int(math.Floor((r.X+r.W)*scale)), int(math.Floor((r.Y+r.H)*scale))
	if right <= left || bottom <= top {
		return image.Rectangle{}
	}
	return image.Rect(left, top, right, bottom)
}

// The caller supplies a clipped, nonempty whole-pixel rectangle. Seed one row
// and copy it, avoiding a color interface and allocation for every solid fill.
func fillOpaquePixels(dst *image.RGBA, r image.Rectangle, c color.NRGBA) {
	start := dst.PixOffset(r.Min.X, r.Min.Y)
	row := dst.Pix[start : start+r.Dx()*4]
	for x := 0; x < len(row); x += 4 {
		row[x], row[x+1], row[x+2], row[x+3] = c.R, c.G, c.B, 255
	}
	for y := r.Min.Y + 1; y < r.Max.Y; y++ {
		at := dst.PixOffset(r.Min.X, y)
		copy(dst.Pix[at:at+len(row)], row)
	}
}
