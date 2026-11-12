package baseui

import (
	"image/color"
	"math"
	"testing"
)

func TestChartContainerAxesSnapshotAndClippedCallback(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(320, 240, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		axes := ChartAxes{Bottom: []ChartTick{{At: 1, Label: "End"}, {At: 0, Label: "Start"}, {At: math.NaN(), Label: "Invalid"}}, Left: []ChartTick{{At: 0, Label: "0"}, {At: 1, Label: "100"}}, Top: []ChartTick{{At: .5, Label: "Top"}}, Right: []ChartTick{{At: .5, Label: "Right"}}}
		var plot Rect
		e := ChartContainer("chart", "Custom data", axes, func(p *Painter, r Rect) {
			plot = r
			p.Fill(Rect{-100, -100, 1000, 1000}, color.NRGBA{R: 255, A: 255}, 0)
		}).Width(300)
		axes.Left[1].Label = "Changed after build"
		mount(t, s, e)
		img, err := s.RenderScaled(1.25, nil)
		if err != nil {
			t.Fatal(err)
		}
		root, _ := s.Bounds("chart")
		if root.H != 300*9./16 || plot.W <= 0 || plot.H <= 0 {
			t.Fatal(root, plot)
		}
		c := img.RGBAAt(int((plot.X+plot.W/2)*1.25), int((plot.Y+plot.H/2)*1.25))
		if c.R != 255 || c.G != 0 {
			t.Fatal("plot not painted", c)
		}
		outside := img.RGBAAt(2, 2)
		if outside.R == 255 && outside.G == 0 {
			t.Fatal("paint escaped plot")
		}
		s.Close()
	}
}
func TestChartTickOrderingAndInvalidPositions(t *testing.T) {
	input := []ChartTick{{1, "last"}, {.5, "middle1"}, {math.Inf(1), "bad"}, {-1, "bad"}, {.5, "middle2"}, {0, "first"}}
	out := snapshotChartTicks(input)
	if len(out) != 4 || out[0].Label != "first" || out[1].Label != "middle1" || out[2].Label != "middle2" || input[0].Label != "last" {
		t.Fatal(out, input)
	}
}
func TestChartLegendColorOwnershipAndEmpty(t *testing.T) {
	s := surface(t)
	colors := []color.NRGBA{{R: 255, A: 255}}
	labels := []string{"First", "Fallback"}
	e := ChartLegend("legend", labels, colors)
	colors[0] = color.NRGBA{B: 255, A: 255}
	labels[0] = "Changed"
	mount(t, s, e)
	img, err := s.Render()
	if err != nil {
		t.Fatal(err)
	}
	r, _ := s.Bounds("legend/0/color")
	c := img.RGBAAt(int(r.X+4), int(r.Y+8))
	if c.R != 255 || c.B != 0 {
		t.Fatal("colors not snapshotted", c)
	}
	if s.entries[s.index["legend/0/label"]].element.text != "First" {
		t.Fatal("labels not snapshotted")
	}
	mount(t, s, ChartLegend("empty", nil, nil))
	if _, err = s.Render(); err != nil {
		t.Fatal(err)
	}
}
func TestChartTooltipDismissAndRestoreFocus(t *testing.T) {
	s := surface(t)
	open := false
	view := func() *Element {
		return Column("root", Button("trigger", "Inspect", func() { open = true }), ChartTooltip("tooltip", "trigger", "Data point", &open, Button("action", "Inspect details", nil)))
	}
	mount(t, s, view())
	click(t, s, "trigger")
	mount(t, s, view())
	if s.modal != "tooltip" || s.focus != "action" {
		t.Fatal(s.modal, s.focus)
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	if open || s.focus != "trigger" {
		t.Fatal(open, s.focus)
	}
	click(t, s, "trigger")
	mount(t, s, view())
	s.Pointer(PointerEvent{Kind: "down", X: 600, Y: 400})
	if open {
		t.Fatal("outside did not dismiss")
	}
}
