// Package glass implements an optional CPU backdrop material. Native AppKit
// materials, GPU shaders and merging pane groups are not yet implemented.
package glass

import (
	"image"
	"image/color"
	"math"
	ui "github.com/herlanai/omnigo/omniui/baseui"
)

type Options struct {
	Radius     float64
	BlurRadius int
	Tint       color.NRGBA
	Refraction float64
	Opaque     bool
}

func Box(id string, o Options, children ...*ui.Element) *ui.Element {
	if o.BlurRadius == 0 {
		o.BlurRadius = 5
	}
	o.BlurRadius = max(0, min(32, o.BlurRadius))
	if o.Radius == 0 {
		o.Radius = 16
	}
	if math.IsNaN(o.Radius) || math.IsInf(o.Radius, 0) || o.Radius < 0 || math.IsNaN(o.Refraction) || math.IsInf(o.Refraction, 0) {
		panic("glass: invalid geometry")
	}
	o.Refraction = max(-16, min(16, o.Refraction))
	return ui.Column(id, children...).Padding(16).PaintSurface(func(p *ui.Painter, r ui.Rect) {
		if o.Opaque {
			p.Fill(r, p.Theme().Surface, o.Radius)
			return
		}
		source := p.Snapshot(r)
		if source == nil {
			return
		}
		blurred := Blur(source, o.BlurRadius)
		b := blurred.Bounds()
		out := image.NewNRGBA(b)
		tint := o.Tint
		if tint.A == 0 {
			tint = p.Theme().Surface
			tint.A = 70
		}
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				nx, ny := (float64(x)+.5)/float64(b.Dx())*2-1, (float64(y)+.5)/float64(b.Dy())*2-1
				edge := math.Pow(max(math.Abs(nx), math.Abs(ny)), 8) * o.Refraction
				sx, sy := max(0, min(b.Dx()-1, x+int(nx*edge))), max(0, min(b.Dy()-1, y+int(ny*edge)))
				c := blurred.NRGBAAt(sx, sy)
				a := float64(tint.A) / 255
				c.R = uint8(float64(c.R)*(1-a) + float64(tint.R)*a)
				c.G = uint8(float64(c.G)*(1-a) + float64(tint.G)*a)
				c.B = uint8(float64(c.B)*(1-a) + float64(tint.B)*a)
				out.SetNRGBA(x, y, c)
			}
		}
		p.Image(r, out, o.Radius)
		rim := p.Theme().Text
		rim.A = 45
		p.Stroke(r, rim, o.Radius, 1)
	})
}

// Blur is a separable box blur in premultiplied color space. The result owns its pixels.
func Blur(src image.Image, radius int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 || w*h > 4194304 {
		return image.NewNRGBA(image.Rectangle{})
	}
	radius = max(0, min(32, radius))
	a := image.NewRGBA(image.Rect(0, 0, w, h))
	tmp := image.NewRGBA(a.Bounds())
	out := image.NewNRGBA(a.Bounds())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	pass := func(from, to *image.RGBA, horizontal bool) {
		outer, inner := h, w
		if !horizontal {
			outer, inner = w, h
		}
		for row := 0; row < outer; row++ {
			var sum [4]int
			at := func(i int) int {
				if horizontal {
					return from.PixOffset(i, row)
				}
				return from.PixOffset(row, i)
			}
			for i := 0; i <= min(radius, inner-1); i++ {
				p := at(i)
				for k := range sum {
					sum[k] += int(from.Pix[p+k])
				}
			}
			for i := 0; i < inner; i++ {
				n := min(inner-1, i+radius) - max(0, i-radius) + 1
				p := at(i)
				for k := range sum {
					to.Pix[p+k] = uint8(sum[k] / n)
				}
				if remove := i - radius; remove >= 0 {
					p = at(remove)
					for k := range sum {
						sum[k] -= int(from.Pix[p+k])
					}
				}
				if add := i + radius + 1; add < inner {
					p = at(add)
					for k := range sum {
						sum[k] += int(from.Pix[p+k])
					}
				}
			}
		}
	}
	pass(a, tmp, true)
	pass(tmp, a, false)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y, a.At(x, y))
		}
	}
	return out
}
