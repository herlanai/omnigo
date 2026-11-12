// Package loading implements optional Go-rendered indicators on baseui.Painter.
// These are independent designs. The available styles do not cover the reference
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
	Clock
	Ripple
	Orbit
	Pulse
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
	if style > Pulse {
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
	if i.style == Dots || i.style == Bars {
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
		case Clock:
			cx, cy, radius := r.X+r.W/2, r.Y+r.H/2, math.Min(r.W, r.H)/2-2
			p.Stroke(baseui.Rect{X: cx - radius, Y: cy - radius, W: radius * 2, H: radius * 2}, c, radius, 1)
			for _, hand := range []struct{ angle, length float64 }{{phase*2*math.Pi - math.Pi/2, .8}, {phase*math.Pi/6 - math.Pi/2, .5}} {
				p.Line(cx, cy, cx+math.Cos(hand.angle)*radius*hand.length, cy+math.Sin(hand.angle)*radius*hand.length, 2, c)
			}
		case Ripple:
			for n := 0; n < 2; n++ {
				f := math.Mod(phase+float64(n)/2, 1)
				d := math.Max(2, math.Min(r.W, r.H)*f)
				cc := c
				cc.A = uint8(float64(c.A) * (1 - f))
				p.Stroke(baseui.Rect{X: r.X + (r.W-d)/2, Y: r.Y + (r.H-d)/2, W: d, H: d}, cc, d/2, 2)
			}
		case Orbit:
			d := math.Min(r.W, r.H) / 5
			p.Fill(baseui.Rect{X: r.X + (r.W-d)/2, Y: r.Y + (r.H-d)/2, W: d, H: d}, c, d/2)
			for n := 0; n < 3; n++ {
				a := phase*2*math.Pi + float64(n)*2*math.Pi/3
				p.Fill(baseui.Rect{X: r.X + (r.W-d)/2 + math.Cos(a)*(r.W-d)/2, Y: r.Y + (r.H-d)/2 + math.Sin(a)*(r.H-d)/2, W: d, H: d}, c, d/2)
			}
		case Pulse:
			f := (1 - math.Cos(phase*2*math.Pi)) / 2
			d := math.Min(r.W, r.H) * (.35 + .65*f)
			c.A = uint8(float64(c.A) * (.4 + .6*(1-f)))
			p.Fill(baseui.Rect{X: r.X + (r.W-d)/2, Y: r.Y + (r.H-d)/2, W: d, H: d}, c, d/2)

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
