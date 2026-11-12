// Package borderbeam provides optional CPU-rendered light fields around Go controls.
// Its renderer is independently implemented; no reference framework is linked.
package borderbeam

import (
	"image"
	"image/color"
	"math"
	ui "github.com/herlanai/omnigo/omniui/baseui"
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

type Palette uint8

const (
	Theme Palette = iota
	Colorful
	Ocean
	Sunset
	Mono
)

type Options struct {
	Preset                        Preset
	Palette                       Palette
	Colors                        []color.NRGBA
	Duration                      time.Duration
	Radius, Strength, Glow, Width float64
	Background                    color.NRGBA
	Tint                          color.NRGBA
	Paused, Disabled              bool
}
type State struct {
	last, elapsed    time.Duration
	started, stopped bool
	image            *image.NRGBA
	key              renderKey
	cached           bool
}
type renderKey struct {
	width, height               int
	phase                       float64
	preset                      Preset
	radius, strength, glow, rim float64
	palette                     string
}

func Duration(p Preset) time.Duration {
	if p == Line {
		return 3100 * time.Millisecond
	}
	if p >= PulseInner {
		return 2300 * time.Millisecond
	}
	return 1960 * time.Millisecond
}
func (s *State) Box(id string, o Options, children ...*ui.Element) *ui.Element {
	if o.Preset > PulseOutside || o.Palette > Mono {
		panic("borderbeam: invalid preset or palette")
	}
	if o.Duration <= 0 {
		o.Duration = Duration(o.Preset)
	}
	if o.Radius == 0 {
		o.Radius = 16
		if o.Preset == Small {
			o.Radius = 32
		}
	}
	if o.Strength == 0 {
		o.Strength = .7
	}
	if o.Glow == 0 {
		o.Glow = 28
	}
	if o.Width == 0 {
		o.Width = 1
	}
	for _, v := range []float64{o.Radius, o.Strength, o.Glow, o.Width} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			panic("borderbeam: invalid geometry")
		}
	}
	o.Strength = min(1, o.Strength)
	o.Glow = min(48, o.Glow)
	o.Width = min(8, o.Width)
	o.Colors = append([]color.NRGBA(nil), o.Colors...)
	return ui.Column(id, children...).Padding(16).Animate(!o.Paused && !o.Disabled).PaintDecoration(func(p *ui.Painter, r ui.Rect) {
		stop := o.Paused || o.Disabled || p.Theme().ReduceMotion
		if !s.started {
			s.last = p.Now()
			s.started = true
		}
		if !s.stopped && p.Now() > s.last {
			s.elapsed += (p.Now() - s.last)
		}
		s.last = p.Now()
		s.stopped = stop
		bg := o.Background
		if bg.A == 0 {
			bg = p.Theme().Surface
		}
		if o.Tint.A != 0 {
			amount := .035
			if p.Theme().Background.R < 128 {
				amount = .07
			}
			bg.R = uint8(float64(bg.R)*(1-amount) + float64(o.Tint.R)*amount)
			bg.G = uint8(float64(bg.G)*(1-amount) + float64(o.Tint.G)*amount)
			bg.B = uint8(float64(bg.B)*(1-amount) + float64(o.Tint.B)*amount)
		}
		p.Fill(r, bg, o.Radius)
		if o.Disabled {
			s.image = nil
			s.cached = false
			return
		}
		phase := float64(s.elapsed%o.Duration) / float64(o.Duration)
		if p.Theme().ReduceMotion {
			phase = 0
		}
		colors := palette(o, p.Theme())
		pad := 0.
		if o.Preset == PulseOutside {
			pad = 60
		}
		w, h := max(1, int(math.Ceil(r.W))), max(1, int(math.Ceil(r.H)))
		// Bound effect allocation; large surfaces use a lower-resolution field.
		scale := min(2., math.Sqrt(1200000/(float64(w+int(2*pad))*float64(h+int(2*pad)))))
		pw, ph := max(1, int((float64(w)+2*pad)*scale)), max(1, int((float64(h)+2*pad)*scale))
		key := renderKey{pw, ph, phase, o.Preset, o.Radius * scale, o.Strength, o.Glow * scale, o.Width * scale, string(colorBytes(colors))}
		if !s.cached || s.key != key {
			if s.image == nil || s.image.Bounds().Dx() != pw || s.image.Bounds().Dy() != ph {
				s.image = image.NewNRGBA(image.Rect(0, 0, pw, ph))
			}
			raster(s.image, float64(w)*scale, float64(h)*scale, pad*scale, key, colors)
			s.key = key
			s.cached = true
		}
		p.Image(ui.Rect{X: r.X - pad, Y: r.Y - pad, W: r.W + 2*pad, H: r.H + 2*pad}, s.image, 0)
	})
}
func colorBytes(cs []color.NRGBA) []byte {
	b := make([]byte, 0, len(cs)*4)
	for _, c := range cs {
		b = append(b, c.R, c.G, c.B, c.A)
	}
	return b
}
func palette(o Options, t ui.Theme) []color.NRGBA {
	if len(o.Colors) > 0 {
		return o.Colors
	}
	switch o.Palette {
	case Colorful:
		return []color.NRGBA{{238, 76, 138, 255}, {96, 126, 255, 255}, {49, 211, 163, 255}, {178, 108, 247, 255}}
	case Ocean:
		return []color.NRGBA{{34, 211, 238, 255}, {56, 130, 246, 255}, {31, 56, 160, 255}}
	case Sunset:
		return []color.NRGBA{{251, 113, 133, 255}, {251, 146, 60, 255}, {216, 110, 225, 255}}
	case Mono:
		return []color.NRGBA{t.Text}
	default:
		return []color.NRGBA{t.Accent, {uint8((int(t.Accent.R) + 255) / 2), uint8((int(t.Accent.G) + 255) / 2), uint8((int(t.Accent.B) + 255) / 2), 255}}
	}
}
func clamp(v float64) float64 { return max(0, min(1, v)) }
func raster(img *image.NRGBA, w, h, pad float64, k renderKey, colors []color.NRGBA) {
	radius := min(k.radius, min(w, h)/2)
	// Moving elliptical light sources create a continuous field, not segmented lines.
	type spot struct {
		x, y, rx, ry float64
		c            color.NRGBA
	}
	spots := make([]spot, len(colors))
	phase := k.phase * 2 * math.Pi
	for i, c := range colors {
		a := float64(i) * 2 * math.Pi / float64(len(colors))
		spots[i] = spot{w * (.5 + .46*math.Cos(a+phase)), h * (.5 + .46*math.Sin(a-phase*.7)), max(16, w*.42), max(16, h*.65), c}
	}
	pulse := .35 + .65*(.5-.5*math.Cos(phase))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			px, py := float64(x)+.5-pad, float64(y)+.5-pad
			dx, dy := math.Abs(px-w/2)-w/2+radius, math.Abs(py-h/2)-h/2+radius
			d := math.Hypot(max(0, dx), max(0, dy)) + min(0, max(dx, dy)) - radius
			inside := clamp(.5 - d)
			rim := inside * clamp(d+k.rim+.5)
			inward := math.Exp(-math.Pow(max(0, -d)/max(1, k.glow*.55), 2)) * inside
			angle := math.Atan2(py-h/2, px-w/2)
			sweep := math.Pow(max(0, .5+.5*math.Cos(angle-phase)), 5)
			alpha := rim*sweep*.95 + inward*sweep*.25
			switch k.preset {
			case Small:
				alpha = rim*sweep + inward*.13*math.Pow(max(0, .5+.5*math.Sin(angle+phase)), 3)
			case Line:
				location := w * (.5 - .6*math.Cos(phase))
				beam := math.Exp(-math.Pow((px-location)/max(20, w*.13), 2) - math.Pow((py-h)/max(16, k.glow*1.8), 2))
				alpha = (rim + inside*.48) * beam * math.Pow(math.Sin(math.Pi*k.phase), 2)
			case PulseInner:
				alpha = (rim*.85 + inward*.55) * pulse * (.65 + .35*math.Sin(angle*2+phase))
			case PulseOutside:
				alpha = rim * pulse
				if d > 0 {
					alpha += pulse * (.38*math.Exp(-d/9) + .18*math.Exp(-d/24)) * (1 - inside)
				}
			}
			if alpha*k.strength < 1./255 {
				img.Pix[img.PixOffset(x, y)+3] = 0
				continue
			}
			var red, green, blue, weight float64
			for _, s := range spots {
				v := math.Exp(-math.Pow((px-s.x)/s.rx, 2)-math.Pow((py-s.y)/s.ry, 2)) * float64(s.c.A) / 255
				red += float64(s.c.R) * v
				green += float64(s.c.G) * v
				blue += float64(s.c.B) * v
				weight += v
			}
			at := img.PixOffset(x, y)
			if weight > 0 {
				img.Pix[at] = uint8(red / weight)
				img.Pix[at+1] = uint8(green / weight)
				img.Pix[at+2] = uint8(blue / weight)
			}
			img.Pix[at+3] = uint8(255 * clamp(alpha*k.strength*min(1, weight*1.4)))
		}
	}
}
