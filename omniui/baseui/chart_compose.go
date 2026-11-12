package baseui

import (
	"fmt"
	"image/color"
	"sort"
)

// ChartTick positions an axis label in normalized plot coordinates. X grows
// left to right, Y bottom to top. Nonfinite and out-of-range ticks are omitted.
type ChartTick struct {
	At    float64
	Label string
}
type ChartAxes struct{ Top, Bottom, Left, Right []ChartTick }

func snapshotChartTicks(ticks []ChartTick) []ChartTick {
	out := make([]ChartTick, 0, len(ticks))
	for _, tick := range ticks {
		if finite(tick.At) && tick.At >= 0 && tick.At <= 1 {
			out = append(out, tick)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}
func chartAxesLayout(p *Painter, r Rect, a ChartAxes) Rect {
	top, bottom, left, right := 0., 0., 0., 0.
	line := p.theme.FontSize * 1.5
	if len(a.Top) > 0 {
		top = line + 10
	}
	if len(a.Bottom) > 0 {
		bottom = line + 10
	}
	for _, tick := range a.Left {
		left = max(left, chartTextWidth(p, tick.Label)+10)
	}
	for _, tick := range a.Right {
		right = max(right, chartTextWidth(p, tick.Label)+10)
	}
	// Labels never consume more than half the surface. A tiny surface still has
	// a nonnegative plot, with all drawing clipped to its actual visible bounds.
	left = min(left, r.W/4)
	right = min(right, r.W/4)
	top = min(top, r.H/4)
	bottom = min(bottom, r.H/4)
	return Rect{r.X + left, r.Y + top, max(0, r.W-left-right), max(0, r.H-top-bottom)}
}
func paintChartAxes(p *Painter, r, plot Rect, a ChartAxes) {
	horizontal := func(ticks []ChartTick, top bool) {
		area := Rect{r.X, plot.Y + plot.H, r.W, max(0, r.Y+r.H-plot.Y-plot.H)}
		if top {
			area = Rect{r.X, r.Y, r.W, max(0, plot.Y-r.Y)}
		}
		q := *p
		q.clip = intersect(q.clip, area)
		baseline := area.Y + p.theme.FontSize + 5
		for _, tick := range ticks {
			width := chartTextWidth(p, tick.Label)
			x := plot.X + tick.At*plot.W - width/2
			x = max(r.X, min(x, r.X+r.W-width))
			q.Text(x, baseline, tick.Label, p.theme.MutedText)
		}
	}
	vertical := func(ticks []ChartTick, left bool) {
		area := Rect{plot.X + plot.W, r.Y, max(0, r.X+r.W-plot.X-plot.W), r.H}
		if left {
			area = Rect{r.X, r.Y, max(0, plot.X-r.X), r.H}
		}
		q := *p
		q.clip = intersect(q.clip, area)
		for _, tick := range ticks {
			x := area.X + 10
			if left {
				x = area.X + area.W - 10 - chartTextWidth(p, tick.Label)
			}
			y := plot.Y + (1-tick.At)*plot.H + p.theme.FontSize*.35
			y = max(r.Y+p.theme.FontSize, min(y, r.Y+r.H-2))
			q.Text(x, y, tick.Label, p.theme.MutedText)
		}
	}
	horizontal(a.Top, true)
	horizontal(a.Bottom, false)
	vertical(a.Left, true)
	vertical(a.Right, false)
}

// ChartContainer is a 16:9 composition surface with four independently
// configurable axes. The callback receives the plot's logical rect and a
// Painter already clipped to it. It must not retain that transient Painter.
func ChartContainer(id, label string, axes ChartAxes, paint func(*Painter, Rect)) *Element {
	axes = ChartAxes{snapshotChartTicks(axes.Top), snapshotChartTicks(axes.Bottom), snapshotChartTicks(axes.Left), snapshotChartTicks(axes.Right)}
	canvas := Canvas(id+"/plot", label, func(p *Painter, r Rect) {
		plot := chartAxesLayout(p, r, axes)
		paintChartAxes(p, r, plot, axes)
		if paint != nil && plot.W > 0 && plot.H > 0 {
			q := *p
			q.clip = intersect(q.clip, plot)
			paint(&q, plot)
		}
	})
	return AspectRatio(id, 16./9, canvas).Role("group").Label(label)
}

// ChartLegend snapshots labels and colors. Missing colors use semantic theme
// colors; an explicitly supplied transparent color remains transparent.
func ChartLegend(id string, labels []string, colors []color.NRGBA) *Element {
	labels = append([]string(nil), labels...)
	colors = append([]color.NRGBA(nil), colors...)
	items := make([]*Element, 0, len(labels))
	for i, label := range labels {
		prefix := fmt.Sprintf("%s/%d", id, i)
		items = append(items, Row(prefix, Canvas(prefix+"/color", label, func(p *Painter, r Rect) {
			c := chartColor(p.theme, i, color.NRGBA{})
			if i < len(colors) {
				c = colors[i]
			}
			p.Fill(Rect{r.X, r.Y + 4, 8, 8}, c, 2)
		}).Width(8).Height(20), Text(prefix+"/label", label)).Gap(6))
	}
	return Row(id, items...).Gap(16).Role("group").Label("Chart legend")
}

// ChartTooltip is a keyboard-accessible anchored popup with caller-owned open
// state. It uses the same focus restoration, Escape and outside dismissal as
// other popovers; its content may include arbitrary Go controls.
func ChartTooltip(id, anchor, title string, open *bool, children ...*Element) *Element {
	if open == nil {
		panic("baseui: nil chart tooltip state")
	}
	body := append([]*Element{Text(id+"/title", title)}, children...)
	return Popover(id, anchor, *open, func() { *open = false }, body...).Width(160).Padding(10).Gap(6).Label(title)
}
