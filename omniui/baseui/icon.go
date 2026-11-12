package baseui

import (
	"image"
	"image/color"
	"math"
)

// IconSource keeps optional SVG parsing and icon catalogs out of applications
// which only import baseui. omniui/icon.SVG implements this interface.
type IconSource interface {
	Size() (float64, float64)
	Rasterize(width, height int, current color.NRGBA, strokeWidth float64) (image.Image, error)
}
type IconOptions struct {
	Size                float64
	Color               color.NRGBA // zero alpha uses the current theme's text color
	StrokeWidth         float64     // source viewBox units; zero preserves source width
	AbsoluteStrokeWidth bool        // interpret StrokeWidth as logical pixels
}

// Icon draws a monochrome vector icon. Empty labels mark decorative images;
// provide a label for standalone meaningful images. This is not a focus stop.
func Icon(id, label string, source IconSource, o IconOptions) *Element {
	if source == nil {
		panic("baseui: nil icon")
	}
	w, h := source.Size()
	if !finite(w) || !finite(h) || w <= 0 || h <= 0 {
		panic("baseui: invalid icon source")
	}
	if o.Size == 0 {
		o.Size = 24
	}
	if !finite(o.Size) || o.Size <= 0 || o.Size > 1024 || !finite(o.StrokeWidth) || o.StrokeWidth < 0 || o.StrokeWidth > 64 {
		panic("baseui: invalid icon options")
	}
	height := o.Size
	width := height * w / h
	if width > 1024 {
		panic("baseui: icon width exceeds limit")
	}
	e := Canvas(id, label, nil).Width(width).Height(height)
	e.iconSource, e.iconOptions = source, o
	if label == "" {
		e.Role("presentation")
	} else {
		e.Role("img")
	}
	return e
}

// Icon rasterizes vectors at the actual output scale, then composites into the
// clipped surface. Neither the source nor the Painter is retained by the other.
func (p *Painter) Icon(source IconSource, r Rect, c color.NRGBA, stroke float64) error {
	if r.W <= 0 || r.H <= 0 {
		return nil
	}
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	pixels, e := source.Rasterize(int(math.Ceil(r.W*scale)), int(math.Ceil(r.H*scale)), c, stroke)
	if e != nil {
		return e
	}
	p.Image(r, pixels, 0)
	return nil
}

// IconButton combines the existing button input/focus semantics with an icon.
// A nonempty label is mandatory because the button has no visible text.
func IconButton(id, label string, source IconSource, action func()) *Element {
	if label == "" || source == nil {
		panic("baseui: icon button needs a label and source")
	}
	e := Button(id, "", action).Label(label).Width(36).Height(36)
	e.iconSource = source
	e.iconOptions = IconOptions{Size: 20}
	return e
}
