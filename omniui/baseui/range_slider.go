package baseui

import "math"

type RangeSliderState struct {
	Low, High                 float64
	dragging                  bool
	originalLow, originalHigh float64
}
type RangeSliderOptions struct {
	Min, Max, Step float64
	OnChange       func(float64, float64)
}

// RangeSlider paints one track with two separately focusable handles. Track hit
// regions are divided at the handles' midpoint so a press chooses the nearest.
func RangeSlider(id, label string, state *RangeSliderState, o RangeSliderOptions) *Element {
	if state == nil || !finite(o.Min) || !finite(o.Max) || !finite(o.Max-o.Min) || !finite(o.Step) || o.Min >= o.Max || o.Step <= 0 || !finite(state.Low) || !finite(state.High) || state.Low < o.Min || state.High > o.Max || state.Low > state.High {
		panic("baseui: invalid range slider")
	}
	span := o.Max - o.Min
	low, high := state.Low, state.High
	midpoint := max(.001, min(.999, ((low-o.Min)/span+(high-o.Min)/span)/2))
	emit := func(lo, hi float64) {
		if lo == state.Low && hi == state.High {
			return
		}
		state.Low, state.High = lo, hi
		if o.OnChange != nil {
			o.OnChange(lo, hi)
		}
	}
	set := func(index int, value float64) {
		value = max(o.Min, min(o.Max, value))
		steps := (value - o.Min) / o.Step
		if finite(steps) {
			value = o.Min + math.Round(steps)*o.Step
		}
		value = max(o.Min, min(o.Max, value))
		if index == 0 {
			emit(min(value, state.High), state.High)
		} else {
			emit(state.Low, max(state.Low, value))
		}
	}
	children := []*Element{}
	for index := 0; index < 2; index++ {
		index := index
		fraction := midpoint
		name := "minimum"
		value := low
		if index == 1 {
			fraction = 1 - midpoint
			name = "maximum"
			value = high
		}
		e := Canvas(id+"/"+name, label+" "+name, nil).Height(32).Grow(fraction).Role("slider")
		e.value, e.min, e.max, e.step = value, o.Min, o.Max, o.Step
		e.onPointer = func(ev PointerEvent, r, viewport Rect) {
			switch ev.Kind {
			case "down":
				state.originalLow, state.originalHigh = state.Low, state.High
				state.dragging = true
			case "cancel":
				if state.dragging {
					emit(state.originalLow, state.originalHigh)
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
			width := r.W / fraction
			left := r.X
			if index == 1 {
				left -= width * midpoint
			}
			set(index, o.Min+max(0, min(1, (ev.X-left)/max(1, width)))*span)
			if ev.Kind == "up" {
				state.dragging = false
			}
		}
		e.OnKey(func(ev KeyEvent) bool {
			value := state.Low
			if index == 1 {
				value = state.High
			}
			switch ev.Key {
			case "Left", "Down":
				value -= o.Step
			case "Right", "Up":
				value += o.Step
			case "Home":
				value = o.Min
			case "End":
				value = o.Max
			default:
				return false
			}
			set(index, value)
			return true
		})
		children = append(children, e)
	}
	root := Row(id, children...).Gap(0).Height(32).Role("group").Label(label)
	root.paint = func(p *Painter, r Rect) {
		y := r.Y + r.H/2
		x1 := r.X + (low-o.Min)/span*r.W
		x2 := r.X + (high-o.Min)/span*r.W
		p.Line(r.X, y, r.X+r.W, y, 4, p.theme.Muted)
		p.Line(x1, y, x2, y, 4, p.theme.Accent)
		for _, x := range []float64{x1, x2} {
			p.Fill(Rect{x - 6, y - 6, 12, 12}, p.theme.Surface, 6)
			p.Stroke(Rect{x - 6, y - 6, 12, 12}, p.theme.Accent, 6, 2)
		}
	}
	return root
}
