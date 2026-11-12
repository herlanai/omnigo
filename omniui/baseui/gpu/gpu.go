// Package gpu is an opt-in native GPU renderer. It is not imported by baseui.
package gpu

import (
	"errors"
	"image"
	"image/color"
	"math"
)

var ErrUnavailable = errors.New("gpu: native GPU backend unavailable")

type Rect struct {
	X, Y, Width, Height float32
	Color               color.NRGBA
}
type renderer interface {
	render(int, int, color.NRGBA, []Rect) (*image.RGBA, error)
	close()
}

// Device is confined to its owning UI goroutine. Close releases native resources.
type Device struct {
	backend renderer
	closed  bool
}

func Open() (*Device, error) {
	b, err := openRenderer()
	if err != nil {
		return nil, err
	}
	return &Device{backend: b}, nil
}

// Render batches ordered, alpha-composited axis-aligned rectangle fills on the
// GPU. Text, paths, antialiasing and filter effects are not yet implemented.
func (d *Device) Render(width, height int, background color.NRGBA, rects []Rect) (*image.RGBA, error) {
	if d.closed {
		return nil, errors.New("gpu: device closed")
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > 16_000_000 || len(rects) > 4096 || int64(width)*int64(height)*int64(max(1, len(rects))) > 64_000_000 {
		return nil, errors.New("gpu: frame work budget exceeded")
	}
	for _, r := range rects {
		for _, v := range []float32{r.X, r.Y, r.Width, r.Height} {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, errors.New("gpu: non-finite geometry")
			}
		}
		if r.Width < 0 || r.Height < 0 {
			return nil, errors.New("gpu: negative rectangle size")
		}
	}
	return d.backend.render(width, height, background, rects)
}
func (d *Device) Close() {
	if !d.closed {
		d.closed = true
		d.backend.close()
	}
}
