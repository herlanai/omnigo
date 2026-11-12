package baseui

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
)

type ColorPickerState struct {
	Value    color.NRGBA
	Open     bool
	hex      *TextBuffer
	last     color.NRGBA
	hue      float64
	dragging bool
	original color.NRGBA
}
type ColorPickerOptions struct{ OnChange func(color.NRGBA) }

func colorHSV(c color.NRGBA) (h, s, v float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	v = max(r, g, b)
	low := min(r, g, b)
	delta := v - low
	if v > 0 {
		s = delta / v
	}
	if delta == 0 {
		return
	}
	switch v {
	case r:
		h = math.Mod((g-b)/delta, 6)
	case g:
		h = (b-r)/delta + 2
	default:
		h = (r-g)/delta + 4
	}
	h /= 6
	if h < 0 {
		h++
	}
	return
}
func hsvColor(h, s, v, a float64) color.NRGBA {
	h = math.Mod(h, 1)
	if h < 0 {
		h++
	}
	s = max(0, min(1, s))
	v = max(0, min(1, v))
	a = max(0, min(1, a))
	sector := h * 6
	x := v * s * (1 - math.Abs(math.Mod(sector, 2)-1))
	c := v * s
	m := v - c
	r, g, b := 0., 0., 0.
	switch int(sector) {
	case 0:
		r, g = c, x
	case 1:
		r, g = x, c
	case 2:
		g, b = c, x
	case 3:
		g, b = x, c
	case 4:
		r, b = x, c
	default:
		r, b = c, x
	}
	return color.NRGBA{uint8(math.Round((r + m) * 255)), uint8(math.Round((g + m) * 255)), uint8(math.Round((b + m) * 255)), uint8(math.Round(a * 255))}
}
func colorHex(c color.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A) }
func parseColorHex(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 && len(s) != 8 {
		return color.NRGBA{}, false
	}
	v, e := strconv.ParseUint(s, 16, 32)
	if e != nil {
		return color.NRGBA{}, false
	}
	if len(s) == 6 {
		v = v<<8 | 255
	}
	return color.NRGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, true
}
func ColorPicker(id, label string, state *ColorPickerState, o ColorPickerOptions) *Element {
	if state == nil {
		panic("baseui: nil color picker")
	}
	if state.hex == nil || state.Value != state.last {
		state.hex = NewTextBuffer(colorHex(state.Value))
		h, s, _ := colorHSV(state.Value)
		if s > 0 {
			state.hue = h
		}
		state.last = state.Value
	}
	set := func(c color.NRGBA, normalize bool) {
		changed := c != state.Value
		state.Value = c
		state.last = c
		if normalize {
			state.hex.Set(colorHex(c))
		}
		if changed && o.OnChange != nil {
			o.OnChange(c)
		}
	}
	_, sat, val := colorHSV(state.Value)
	hue, alpha := state.hue, float64(state.Value.A)/255
	square := Canvas(id+"/square", label+" saturation and brightness", func(p *Painter, r Rect) {
		p.rasterShade(r, func(x, y float64) float64 { return -1 }, func(x, y float64) color.NRGBA { return hsvColor(hue, (x-r.X)/max(1, r.W), 1-(y-r.Y)/max(1, r.H), 1) })
		x, y := r.X+sat*r.W, r.Y+(1-val)*r.H
		p.Stroke(Rect{x - 5, y - 5, 10, 10}, color.NRGBA{0, 0, 0, 255}, 5, 3)
		p.Stroke(Rect{x - 5, y - 5, 10, 10}, color.NRGBA{255, 255, 255, 255}, 5, 1)
	}).Height(140).Role("slider").Description(fmt.Sprintf("Saturation %.0f%%; brightness %.0f%%", sat*100, val*100))
	square.onPointer = func(ev PointerEvent, r, viewport Rect) {
		switch ev.Kind {
		case "down":
			state.original = state.Value
			state.dragging = true
		case "cancel":
			if state.dragging {
				set(state.original, true)
			}
			state.dragging = false
			return
		case "move", "up":
			if !state.dragging {
				return
			}
		default:
			return
		}
		set(hsvColor(state.hue, (ev.X-r.X)/max(1, r.W), 1-(ev.Y-r.Y)/max(1, r.H), float64(state.Value.A)/255), true)
		if ev.Kind == "up" {
			state.dragging = false
		}
	}
	square.OnKey(func(ev KeyEvent) bool {
		_, s, v := colorHSV(state.Value)
		switch ev.Key {
		case "Left":
			s -= .01
		case "Right":
			s += .01
		case "Up":
			v += .01
		case "Down":
			v -= .01
		case "Home":
			s = 0
		case "End":
			s = 1
		default:
			return false
		}
		set(hsvColor(state.hue, s, v, float64(state.Value.A)/255), true)
		return true
	})
	hueSlider := Slider(id+"/hue", label+" hue", hue*360, 0, 360, 1, func(v float64) {
		state.hue = math.Mod(v/360, 1)
		_, s, b := colorHSV(state.Value)
		set(hsvColor(state.hue, s, b, float64(state.Value.A)/255), true)
	})
	opacity := Slider(id+"/opacity", label+" opacity", alpha*100, 0, 100, 1, func(v float64) { c := state.Value; c.A = uint8(math.Round(v * 255 / 100)); set(c, true) })

	hueSlider.paint = func(p *Painter, r Rect) {
		track := Rect{r.X + 10, r.Y + r.H/2 - 3, max(0, r.W-20), 6}
		p.rasterShade(track, func(x, y float64) float64 { return -1 }, func(x, y float64) color.NRGBA { return hsvColor((x-track.X)/max(1, track.W), 1, 1, 1) })
		x := track.X + hue*track.W
		p.Fill(Rect{x - 7, r.Y + r.H/2 - 7, 14, 14}, p.theme.Surface, 7)
		p.Stroke(Rect{x - 7, r.Y + r.H/2 - 7, 14, 14}, p.theme.Text, 7, 1)
	}
	opacityColor := state.Value
	opacity.paint = func(p *Painter, r Rect) {
		track := Rect{r.X + 10, r.Y + r.H/2 - 3, max(0, r.W-20), 6}
		c := opacityColor
		p.rasterShade(track, func(x, y float64) float64 { return -1 }, func(x, y float64) color.NRGBA {
			bg := p.theme.Surface
			if int((x-track.X)/5)%2 == 0 {
				bg = p.theme.Border
			}
			c.A = 255
			return blend(bg, c, (x-track.X)/max(1, track.W))
		})
		x := track.X + alpha*track.W
		p.Fill(Rect{x - 7, r.Y + r.H/2 - 7, 14, 14}, p.theme.Surface, 7)
		p.Stroke(Rect{x - 7, r.Y + r.H/2 - 7, 14, 14}, p.theme.Text, 7, 1)
	}
	hex := Input(id+"/hex", label+" hex RGBA", state.hex)
	_, valid := parseColorHex(state.hex.String())
	hex.Invalid(!valid)
	hex.onChange = func(s string) {
		if c, ok := parseColorHex(s); ok {
			h, s, _ := colorHSV(c)
			if s > 0 {
				state.hue = h
			}
			set(c, false)
		}
	}
	hex.OnSubmit(func() {
		if state.hex.Composition() == "" {
			state.hex.Set(colorHex(state.Value))
		}
	})
	palette := []color.NRGBA{{0, 0, 0, 255}, {255, 255, 255, 255}, {225, 50, 50, 255}, {30, 160, 80, 255}, {40, 100, 230, 255}, {230, 175, 20, 255}}
	swatches := []*Element{}
	for i, c := range palette {
		c := c
		b := Button(fmt.Sprintf("%s/swatch/%d", id, i), "", func() { state.hue, _, _ = colorHSV(c); set(c, true) }).Width(28).Height(28).Label(colorHex(c))
		b.paint = func(p *Painter, r Rect) { p.Fill(Rect{r.X + 5, r.Y + 5, r.W - 10, r.H - 10}, c, 3) }
		swatches = append(swatches, b)
	}
	return Column(id, square, Text(id+"/hue-label", "Hue"), hueSlider, Text(id+"/opacity-label", "Opacity"), opacity, hex, Row(id+"/swatches", swatches...).Gap(6)).Width(260).Role("group").Label(label)
}
func ColorWell(id, label string, state *ColorPickerState, o ColorPickerOptions) *Element {
	if state == nil {
		panic("baseui: nil color well")
	}
	c := state.Value
	trigger := Button(id+"/trigger", colorHex(c), func() { state.Open = !state.Open }).Label(label).Variant(Outline)
	trigger.paint = func(p *Painter, r Rect) { p.Fill(Rect{r.X + 8, r.Y + 8, 12, max(0, r.H-16)}, c, 3) }
	return Column(id, trigger, Popover(id+"/popup", id+"/trigger", state.Open, func() { state.Open = false }, ColorPicker(id+"/picker", label, state, o))).Gap(0)
}
