// Package mobile hosts Go controls on mobile platforms. Rendering and control
// behavior remain in baseui; the optional native driver only presents pixels.
package mobile

import (
	"errors"
	"image"
	"math"
	"time"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

type Content struct {
	View  func() *ui.Element
	Close func()
}

type Options struct {
	// Create runs once on the UI event goroutine. Keep application state here.
	Create func() Content
	Dark   bool
	// OnReady runs after the first frame is presented, on the UI event goroutine.
	OnReady func()
	// OnError receives native/render errors on the UI event goroutine.
	OnError func(error)
}

// canvas is owned by the native event goroutine. It survives GPU context loss;
// platform textures do not. Every state mutation passes through this owner.
type canvas struct {
	content        Content
	surface        *ui.Surface
	dark           bool
	scale          float64
	width, height  int
	active, closed bool
	pointer        int64
	touching       bool
}

func newCanvas(o Options) (*canvas, error) {
	if o.Create == nil {
		return nil, errors.New("mobile: Create is required")
	}
	c := o.Create()
	if c.View == nil {
		if c.Close != nil {
			c.Close()
		}
		return nil, errors.New("mobile: View is required")
	}
	return &canvas{content: c, dark: o.Dark, active: true}, nil
}
func (c *canvas) resize(w, h int, scale float64) error {
	if c.closed {
		return errors.New("mobile: closed")
	}
	if w < 1 || h < 1 || w > 8192 || h > 8192 || math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 || scale > 8 || math.Ceil(float64(w)*scale)*math.Ceil(float64(h)*scale) > 16_000_000 {
		return errors.New("mobile: invalid viewport or pixel budget exceeded")
	}
	if c.surface == nil {
		s, err := ui.NewSurface(w, h, ui.NewTheme(c.dark))
		if err != nil {
			return err
		}
		c.surface = s
	} else if err := c.surface.Resize(w, h); err != nil {
		return err
	}
	c.width, c.height, c.scale = w, h, scale
	return c.mount()
}
func (c *canvas) mount() error { return c.surface.Mount(c.content.View()) }
func (c *canvas) touch(id int64, kind string, x, y float64) error {
	if c.closed {
		return errors.New("mobile: closed")
	}
	if !c.active || c.surface == nil {
		return nil
	}
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return errors.New("mobile: non-finite touch")
	}
	switch kind {
	case "down":
		if c.touching {
			return nil
		}
		c.pointer = id
		c.touching = true
	case "move", "up", "cancel":
		if !c.touching || id != c.pointer {
			return nil
		}
	default:
		return errors.New("mobile: invalid touch phase")
	}
	if kind == "up" || kind == "cancel" {
		c.touching = false
	}
	if err := c.surface.Pointer(ui.PointerEvent{Kind: kind, X: x / c.scale, Y: y / c.scale}); err != nil {
		return err
	}
	return c.mount()
}
func (c *canvas) activate(active bool) error {
	if c.closed {
		return errors.New("mobile: closed")
	}
	c.active = active
	if !active && c.surface != nil {
		c.touching = false
		c.surface.CancelComposition()
		if err := c.surface.Pointer(ui.PointerEvent{Kind: "cancel"}); err != nil {
			return err
		}
		return c.mount()
	}
	return nil
}
func (c *canvas) render(now time.Duration) (*image.RGBA, error) {
	if c.closed || c.surface == nil || !c.active {
		return nil, errors.New("mobile: canvas not active")
	}
	if err := c.surface.Advance(now); err != nil {
		return nil, err
	}
	if err := c.mount(); err != nil {
		return nil, err
	}
	return c.surface.RenderScaled(c.scale, nil)
}
func (c *canvas) close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if c.content.Close != nil {
		defer c.content.Close()
	}
	if c.surface != nil {
		return c.surface.Close()
	}
	return nil
}
