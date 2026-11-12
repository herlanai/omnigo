// Package sfsymbols obtains artwork from AppKit at runtime; no Apple artwork is
// bundled. Render must be called on the native application's main UI thread.
package sfsymbols

import (
	"errors"
	"image"
	"math"
)

var ErrUnsupported = errors.New("sfsymbols: requires macOS AppKit")

type Options struct{ Size, Weight float64 }

func Render(name string, o Options) (image.Image, error) {
	if name == "" {
		return nil, errors.New("sfsymbols: empty name")
	}
	if o.Size == 0 {
		o.Size = 20
	}
	if math.IsNaN(o.Size) || math.IsInf(o.Size, 0) || o.Size < 1 || o.Size > 256 || math.IsNaN(o.Weight) || math.IsInf(o.Weight, 0) || o.Weight < -1 || o.Weight > 1 {
		return nil, errors.New("sfsymbols: invalid options")
	}
	return render(name, o)
}
