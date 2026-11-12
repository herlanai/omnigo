package baseui

import (
	"image/color"
	"math"
)

// Shadow draws a bounded, soft CPU shadow behind a rounded surface. It obeys
// the caller's current clip and never retains the Painter or allocates a bitmap.
func (p *Painter) Shadow(r Rect, radius, blur, offsetY float64, c color.NRGBA) {
	if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || !finite(blur) || !finite(offsetY) || !finite(radius) || blur <= 0 || blur > 32 || math.Abs(offsetY) > 64 || r.W <= 0 || r.H <= 0 {
		return
	}
	shape := r
	shape.Y += offsetY
	spread := blur * 3
	area := Rect{shape.X - spread, shape.Y - spread, shape.W + 2*spread, shape.H + 2*spread}
	p.rasterShade(area, func(x, y float64) float64 { return -1 }, func(x, y float64) color.NRGBA {
		distance := max(0, roundedDistance(shape, radius, x, y))
		out := c
		out.A = uint8(math.Round(float64(c.A) * math.Exp(-distance*distance/(2*blur*blur))))
		return out
	})
}
