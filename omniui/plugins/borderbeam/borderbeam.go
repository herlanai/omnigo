// Package borderbeam draws optional native animated border decoration.
// Geometry and animation are independent designs, not pixel-identical ports.
package borderbeam

import (
	"image/color"
	"math"
	ui "omnigo.dev/omnigo/omniui/baseui"
	"time"
)

type Preset uint8

const (
	Medium Preset = iota
	Small
	Line
	PulseInner
	PulseOutside
)

type Options struct {
	Preset           Preset
	Colors           []color.NRGBA
	Duration         time.Duration
	Radius, Strength float64
	Paused, Disabled bool
}
type State struct {
	last, elapsed   time.Duration
	started, paused bool
}

func (s *State) Box(id string, o Options, children ...*ui.Element) *ui.Element {
	if o.Preset > PulseOutside {
		panic("borderbeam: invalid preset")
	}
	if o.Duration <= 0 {
		o.Duration = 2 * time.Second
	}
	if o.Radius == 0 {
		o.Radius = 16
	}
	if o.Strength == 0 {
		o.Strength = 1
	}
	if math.IsNaN(o.Radius) || math.IsInf(o.Radius, 0) || o.Radius < 0 || math.IsNaN(o.Strength) || math.IsInf(o.Strength, 0) || o.Strength < 0 {
		panic("borderbeam: invalid geometry")
	}
	colors := append([]color.NRGBA(nil), o.Colors...)
	return ui.Column(id, children...).Padding(16).Animate(!o.Paused && !o.Disabled).PaintSurface(func(p *ui.Painter, r ui.Rect) {
		stopped := o.Paused || o.Disabled || p.Theme().ReduceMotion
		if !s.started {
			s.last = p.Now()
			s.started = true
		}
		if !s.paused && p.Now() > s.last {
			s.elapsed = (s.elapsed + (p.Now()-s.last)%o.Duration) % o.Duration
		}
		s.last = p.Now()
		s.paused = stopped
		if o.Disabled {
			return
		}
		phase := float64(s.elapsed%o.Duration) / float64(o.Duration)
		if p.Theme().ReduceMotion {
			phase = 0
		}
		palette := colors
		if len(palette) == 0 {
			palette = []color.NRGBA{p.Theme().Accent, p.Theme().Focus, p.Theme().Text}
		}
		p.Stroke(r, p.Theme().Border, o.Radius, 1)
		if o.Preset == Line {
			for n := 0; n < 80; n++ {
				x := float64(n) / 79
				distance := math.Abs(x - phase)
				a := math.Max(0, 1-distance/.22)
				c := palette[n%len(palette)]
				c.A = uint8(float64(c.A) * a * min(1, o.Strength))
				p.Line(r.X+x*r.W, r.Y+r.H-1, r.X+(x+1./80)*r.W, r.Y+r.H-1, 2, c)
			}
			return
		}
		radius := min(o.Radius, min(r.W, r.H)/2)
		straightX, straightY := max(0, r.W-2*radius), max(0, r.H-2*radius)
		perimeter := 2*(straightX+straightY) + 2*math.Pi*radius
		if perimeter <= 0 {
			return
		}
		point := func(t float64) (float64, float64) {
			d := t * perimeter
			lengths := []float64{straightX, math.Pi * radius / 2, straightY, math.Pi * radius / 2, straightX, math.Pi * radius / 2, straightY, math.Pi * radius / 2}
			for side, length := range lengths {
				if d <= length || side == 7 {
					switch side {
					case 0:
						return r.X + radius + d, r.Y
					case 2:
						return r.X + r.W, r.Y + radius + d
					case 4:
						return r.X + r.W - radius - d, r.Y + r.H
					case 6:
						return r.X, r.Y + r.H - radius - d
					}
					if radius == 0 {
						return r.X, r.Y
					}
					angle := -math.Pi/2 + float64(side/2)*math.Pi/2 + d/radius
					cx, cy := r.X+r.W-radius, r.Y+radius
					if side == 3 {
						cy = r.Y + r.H - radius
					}
					if side == 5 {
						cx = r.X + radius
						cy = r.Y + r.H - radius
					}
					if side == 7 {
						cx = r.X + radius
					}
					return cx + math.Cos(angle)*radius, cy + math.Sin(angle)*radius
				}
				d -= length
			}
			return r.X, r.Y
		}
		for n := 0; n < 180; n++ {
			t := float64(n) / 180
			trail := math.Mod(t-phase+1, 1)
			a := max(0, 1-trail/.28)
			if o.Preset == Small {
				a = max(0, 1-trail/.15)
			}
			if o.Preset == PulseInner || o.Preset == PulseOutside {
				a = .35 + .65*(math.Sin(phase*2*math.Pi)+1)/2
			}
			c := palette[(n*len(palette)/180)%len(palette)]
			c.A = uint8(float64(c.A) * a * min(1, o.Strength))
			x, y := point(t)
			xx, yy := point(float64(n+1) / 180)
			p.Line(x, y, xx, yy, 2, c)
		}
	})
}
