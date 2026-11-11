// Package loading implements optional Go-rendered indicators on baseui.Painter.
// These are independent designs. Ring, Dots and Bars do not cover the reference
// plugin's entire 29-indicator catalog.
package loading

import (
	"image/color"
	"math"
	"time"

	"omnigo.dev/omnigo/omniui/baseui"
)

type Style uint8

const (
	Ring Style = iota
	Dots
	Bars
)

type Options struct {
	Size   float64
	Color  color.NRGBA
	Period time.Duration
	Paused bool
}
type Indicator struct {
	style               Style
	last, elapsed       time.Duration
	initialized, paused bool
}

func New(style Style) *Indicator {
	if style > Bars {
		panic("loading: invalid style")
	}
	return &Indicator{style: style}
}
func (i *Indicator) phase(now time.Duration, paused bool, period time.Duration) float64 {
	if !i.initialized {
		i.last = now
		i.initialized = true
	}
	if now > i.last && !i.paused {
		i.elapsed = (i.elapsed + (now-i.last)%period) % period
	}
	i.last = now
	i.paused = paused
	return float64(i.elapsed%period) / float64(period)
}
func (i *Indicator) View(id, label string, o Options) *baseui.Element {
	if o.Size == 0 {
		o.Size = 24
	}
	if o.Period <= 0 {
		o.Period = time.Second
	}
	if math.IsNaN(o.Size) || math.IsInf(o.Size, 0) || o.Size < 1 || o.Size > 1024 {
		panic("loading: invalid size")
	}
	width := o.Size
	if i.style != Ring {
		width = o.Size * 2
	}
	return baseui.Canvas(id, label, func(p *baseui.Painter, r baseui.Rect) {
		phase := i.phase(p.Now(), o.Paused || p.Theme().ReduceMotion, o.Period)
		if p.Theme().ReduceMotion {
			phase = 0
		}
		c := o.Color
		if c.A == 0 {
			c = p.Theme().Accent
		}
		switch i.style {
		case Ring:
			radius := math.Max(0, math.Min(r.W, r.H)/2-2)
			for n := 0; n < 24; n++ {
				a := phase*2*math.Pi + float64(n)*math.Pi/16
				b := a + math.Pi/16
				p.Line(r.X+r.W/2+math.Cos(a)*radius, r.Y+r.H/2+math.Sin(a)*radius, r.X+r.W/2+math.Cos(b)*radius, r.Y+r.H/2+math.Sin(b)*radius, 2, c)
			}
		case Dots:
			for n := 0; n < 3; n++ {
				lift := (math.Sin(phase*2*math.Pi-float64(n)*.8) + 1) / 2
				d := o.Size / 4
				p.Fill(baseui.Rect{X: r.X + float64(n)*r.W/3 + (r.W/3-d)/2, Y: r.Y + r.H*.65 - lift*r.H*.4, W: d, H: d}, c, d/2)
			}
		case Bars:
			for n := 0; n < 5; n++ {
				height := r.H * (.3 + .7*(math.Sin(phase*2*math.Pi-float64(n)*.6)+1)/2)
				w := r.W / 8
				p.Fill(baseui.Rect{X: r.X + float64(n)*r.W/5, Y: r.Y + (r.H-height)/2, W: w, H: height}, c, w/2)
			}
		}
	}).Width(width).Height(o.Size).Role("progressbar").Animate(!o.Paused)
}
