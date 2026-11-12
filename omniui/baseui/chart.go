package baseui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

type ChartKind uint8

const (
	ChartBar ChartKind = iota
	ChartLine
	ChartArea
)

type ChartSeries struct {
	Label  string
	Values []float64
	Color  color.NRGBA
}
type ChartState struct {
	Selected int
	hovering bool
	hoverX   float64
}
type ChartOptions struct {
	Label  string
	Kind   ChartKind
	Height float64
	Curve  ChartCurve
	// Grid and Legend are retained for source compatibility. Both are enabled
	// by default; use HideGrid and HideLegend to opt out explicitly.
	Grid, Legend                    bool
	HideGrid, HideLegend, ShowYAxis bool
	FormatValue                     func(float64) string
	OnSelect                        func(int)
}

func chartDomain(series []ChartSeries) (float64, float64) {
	lo, hi := 0., 0.
	for _, s := range series {
		for _, v := range s.Values {
			if finite(v) {
				lo = min(lo, v)
				hi = max(hi, v)
			}
		}
	}
	if lo == hi {
		hi = lo + 1
	}
	return lo, hi
}
func chartDescription(label string, series []ChartSeries, index int, format func(float64) string) string {
	var text strings.Builder
	text.WriteString(label)
	for _, s := range series {
		if index < len(s.Values) && finite(s.Values[index]) {
			text.WriteString(" · ")
			text.WriteString(s.Label)
			text.WriteString(": ")
			text.WriteString(format(s.Values[index]))
		}
	}
	return text.String()
}

// Chart renders categorical bars, lines and gradient areas in Go. Only values
// with a corresponding category contribute to the zero-inclusive nice domain.
// Missing/nonfinite values split runs; empty data clears selection without a
// user callback. State belongs to the UI goroutine and Selected=-1 means none.
func Chart(id string, labels []string, series []ChartSeries, state *ChartState, o ChartOptions) *Element {
	if state == nil || o.Kind > ChartArea || o.Curve > CurveStep || !finite(o.Height) {
		panic("baseui: invalid chart options")
	}
	labels = append([]string(nil), labels...)
	series = append([]ChartSeries(nil), series...)
	for i := range series {
		series[i].Values = append([]float64(nil), series[i].Values[:min(len(labels), len(series[i].Values))]...)
	}
	if len(labels) == 0 || len(series) == 0 {
		state.Selected = -1
		return Text(id, "No chart data").Role("note")
	}
	if state.Selected < -1 || state.Selected >= len(labels) {
		state.Selected = -1
	}
	lo, hi := chartDomain(series)
	domain := chartNiceScale(lo, hi)
	format := o.FormatValue
	if format == nil {
		format = func(v float64) string { return fmt.Sprintf("%g", v) }
	}
	height := o.Height
	if height <= 0 {
		height = 260
	}
	label := o.Label
	if label == "" {
		label = "Chart plot"
	}
	selectIndex := func(n int) {
		if n >= -1 && n < len(labels) && n != state.Selected {
			state.Selected = n
			if o.OnSelect != nil {
				o.OnSelect(n)
			}
		}
	}
	canvas := Canvas(id+"/plot", label, func(p *Painter, r Rect) {
		plot := chartPlot(r, o.ShowYAxis)
		if plot.W <= 0 || plot.H <= 0 {
			return
		}
		t := p.Theme()
		y := func(v float64) float64 { return plot.Y + plot.H*(1-domain.at(v)) }
		q := *p
		q.clip = intersect(p.clip, plot)
		if !o.HideGrid {
			for _, v := range domain.ticks {
				q.Line(plot.X, y(v), plot.X+plot.W, y(v), 1, t.Border)
			}
		}
		if o.ShowYAxis {
			axis := *p
			axis.clip = intersect(p.clip, Rect{r.X, r.Y, max(0, plot.X-r.X-6), r.H})
			for _, v := range domain.ticks {
				text := format(v)
				axis.Text(max(r.X, plot.X-8-chartTextWidth(p, text)), y(v)+4, text, t.MutedText)
			}
		}
		stride := max(1, (len(labels)+6)/7)
		lastRight := math.Inf(-1)
		for i := 0; i < len(labels); i += stride {
			text := labels[i]
			width := chartTextWidth(p, text)
			x := plot.X + chartX(o.Kind, i, len(labels))*plot.W
			left := max(plot.X, min(x-width/2, plot.X+plot.W-width))
			if left < lastRight+6 {
				continue
			}
			axis := *p
			axis.clip = intersect(p.clip, Rect{plot.X, plot.Y + plot.H, plot.W, max(0, r.Y+r.H-plot.Y-plot.H)})
			axis.Text(left, plot.Y+plot.H+20, text, t.MutedText)
			lastRight = left + width
		}
		zero := y(0)
		for si, s := range series {
			c := chartColor(t, si, s.Color)
			if o.Kind == ChartBar {
				group := plot.W * .8 / float64(len(labels))
				width := group / float64(len(series))
				for i, v := range s.Values {
					if !finite(v) {
						continue
					}
					x := plot.X + chartX(o.Kind, i, len(labels))*plot.W - group/2 + float64(si)*width
					py := y(v)
					if py == zero {
						continue
					}
					q.Fill(Rect{x + width*.04, min(py, zero), width * .92, math.Abs(py - zero)}, c, 4)
				}
				continue
			}
			run := []chartPoint{}
			flush := func() {
				if len(run) == 0 {
					return
				}
				points := chartCurvePoints(run, o.Curve, p.scale)
				if o.Kind == ChartArea {
					q.chartArea(points, zero, c)
				}
				q.chartStroke(points, 1.5, c)
				run = run[:0]
			}
			for i := range labels {
				if i >= len(s.Values) || !finite(s.Values[i]) {
					flush()
					continue
				}
				run = append(run, chartPoint{plot.X + chartX(o.Kind, i, len(labels))*plot.W, y(s.Values[i])})
			}
			flush()
		}
		i := state.Selected
		if i >= 0 && i < len(labels) {
			x := plot.X + chartX(o.Kind, i, len(labels))*plot.W
			q.Line(x, plot.Y, x, plot.Y+plot.H, 1, t.MutedText)
			rows := []string{}
			colors := []color.NRGBA{}
			for si, s := range series {
				if i >= len(s.Values) || !finite(s.Values[i]) {
					continue
				}
				c := chartColor(t, si, s.Color)
				py := y(s.Values[i])
				q.Fill(Rect{x - 3.5, py - 3.5, 7, 7}, c, 4)
				q.Stroke(Rect{x - 3.5, py - 3.5, 7, 7}, t.Surface, 4, 1)
				rows = append(rows, s.Label+": "+format(s.Values[i]))
				colors = append(colors, c)
			}
			chartTooltip(p, plot, x, labels[i], rows, colors)
		}
	}).Height(height).Role("img").Description("Arrow keys select data; Escape clears selection")
	if state.Selected >= 0 {
		canvas.Description(chartDescription(labels[state.Selected], series, state.Selected, format))
	}
	canvas.OnKey(func(ev KeyEvent) bool {
		key := ev.Key
		if ev.RTL {
			if key == "Left" {
				key = "Right"
			} else if key == "Right" {
				key = "Left"
			}
		}
		n := state.Selected
		switch key {
		case "Right", "Down":
			n = min(len(labels)-1, n+1)
		case "Left", "Up":
			if n < 0 {
				n = len(labels) - 1
			} else {
				n = max(0, n-1)
			}
		case "Home":
			n = 0
		case "End":
			n = len(labels) - 1
		case "Escape":
			n = -1
		default:
			return false
		}
		selectIndex(n)
		return true
	})
	canvas.onPointer = func(ev PointerEvent, r, _ Rect) {
		if ev.Kind != "down" {
			return
		}
		plot := chartPlot(r, o.ShowYAxis)
		if plot.W > 0 && plot.H > 0 && plot.Contains(ev.X, ev.Y) {
			selectIndex(chartPick(o.Kind, len(labels), (ev.X-plot.X)/plot.W))
		}
	}
	canvas.onHover = func(ev PointerEvent, r Rect) {
		if ev.Kind == "cancel" {
			state.hovering = false
			return
		}
		plot := chartPlot(r, o.ShowYAxis)
		over := ev.Kind == "move" && plot.W > 0 && plot.H > 0 && plot.Contains(ev.X, ev.Y)
		if !over {
			if state.hovering {
				state.hovering = false
				selectIndex(-1)
			}
			return
		}
		if !state.hovering || ev.X != state.hoverX {
			selectIndex(chartPick(o.Kind, len(labels), (ev.X-plot.X)/plot.W))
		}
		state.hovering = true
		state.hoverX = ev.X
	}
	children := []*Element{canvas}
	if !o.HideLegend {
		names := make([]string, len(series))
		// Resolve missing colors at paint time in ChartLegend; per-series
		// defaults still use their original indices.
		for i, s := range series {
			names[i] = s.Label
		}
		legend := ChartLegend(id+"/legend", names, nil)
		for i, s := range series {
			if s.Color != (color.NRGBA{}) {
				swatch := legend.children[i].children[0]
				c := s.Color
				swatch.paint = func(p *Painter, r Rect) { p.Fill(Rect{r.X, r.Y + 4, 8, 8}, c, 2) }
			}
		}
		children = append(children, legend)
	}

	if state.Selected >= 0 {
		children = append(children, paragraph(id+"/selection", chartDescription(labels[state.Selected], series, state.Selected, format)).Role("status"))
	}
	return Column(id, children...).Role("group").Label(label)
}
