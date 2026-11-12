package baseui

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

func TestChartNiceScaleRoundTicksAndExtremes(t *testing.T) {
	for _, input := range [][2]float64{{0, 87}, {-5, 17}, {-.013, .009}, {0, math.SmallestNonzeroFloat64}, {-math.MaxFloat64, math.MaxFloat64}} {
		d := chartNiceScale(input[0], input[1])
		if !finite(d.lo) || !finite(d.hi) || d.lo > input[0] || d.hi < input[1] || d.lo >= d.hi {
			t.Fatal(input, d)
		}
		if d.at(input[0]) < 0 || d.at(input[1]) > 1 || !finite(d.at(0)) {
			t.Fatal("invalid normalization", input, d)
		}
		for i, v := range d.ticks {
			if !finite(v) || v < d.lo || v > d.hi || (i > 0 && v <= d.ticks[i-1]) {
				t.Fatal("invalid ticks", d)
			}
		}
	}
	d := chartNiceScale(0, 87)
	if d.hi != 100 || len(d.ticks) != 6 {
		t.Fatal("expected 20-unit nice ticks", d)
	}
	for i, v := range d.ticks {
		if math.Abs(v-float64(i)*20) > 1e-10 {
			t.Fatal(d)
		}
	}
}
func TestChartOnlyCategoricalDataAffectsDomain(t *testing.T) {
	render := func(values []float64) *image.RGBA {
		s := surface(t)
		mount(t, s, Chart("chart", []string{"A", "B"}, []ChartSeries{{Label: "Data", Values: values}}, &ChartState{Selected: -1}, ChartOptions{ShowYAxis: true}))
		img, err := s.Render()
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	a, b := render([]float64{2, 3}), render([]float64{2, 3, 1e100})
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("unlabelled value changed scale")
	}
}
func TestChartCurvesStepPositionsAndMonotoneBounds(t *testing.T) {
	points := []chartPoint{{0, 0}, {10, 10}, {20, 10}, {30, -5}, {40, 0}}
	for _, curve := range []ChartCurve{CurveLinear, CurveMonotone, CurveStepAfter, CurveStepBefore, CurveStep} {
		output := chartCurvePoints(points, curve, 2)
		if output[0] != points[0] || output[len(output)-1] != points[len(points)-1] {
			t.Fatal(curve, "endpoints")
		}
		previous := -math.MaxFloat64
		for _, p := range output {
			if !finite(p.x) || !finite(p.y) || p.x < previous {
				t.Fatal(curve, p)
			}
			previous = p.x
			if curve == CurveMonotone {
				i := min(len(points)-2, int(p.x/10))
				if p.y < min(points[i].y, points[i+1].y)-1e-9 || p.y > max(points[i].y, points[i+1].y)+1e-9 {
					t.Fatal("overshoot", p)
				}
			}
		}
	}
	for curve, want := range map[ChartCurve]chartPoint{CurveStepAfter: {10, 0}, CurveStepBefore: {0, 10}, CurveStep: {5, 0}} {
		got := chartCurvePoints(points[:2], curve, 1)
		if got[1] != want {
			t.Fatal(curve, got)
		}
	}
}
func TestChartEmptyAllKindsAndShortSeries(t *testing.T) {
	for _, kind := range []ChartKind{ChartBar, ChartLine, ChartArea} {
		for _, emptyLabels := range []bool{false, true} {
			s := surface(t)
			state := &ChartState{Selected: 1}
			labels := []string{"A", "B"}
			series := []ChartSeries(nil)
			if emptyLabels {
				labels = nil
				series = []ChartSeries{{Values: []float64{1}}}
			}
			mount(t, s, Chart("chart", labels, series, state, ChartOptions{Kind: kind, OnSelect: func(int) { t.Fatal("empty data fired callback") }}))
			if state.Selected != -1 || s.Focus("chart/plot") {
				t.Fatal(kind, "empty selection")
			}
			if s.entries[0].element.text != "No chart data" {
				t.Fatal("empty message")
			}
		}
	}
}
func TestChartSelectionTooltipFormatAndReadOnly(t *testing.T) {
	s := surface(t)
	state := &ChartState{Selected: -1}
	calls := 0
	format := func(v float64) string { return "$" + strings.Repeat("x", int(v)) }
	view := func() *Element {
		return Chart("chart", []string{"A", "B", "C"}, []ChartSeries{{Label: "Value", Values: []float64{2, math.NaN(), 3}}}, state, ChartOptions{Kind: ChartLine, Label: "Revenue", Height: 180, Curve: CurveMonotone, ShowYAxis: true, FormatValue: format, OnSelect: func(int) { calls++ }}).Width(400)
	}
	mount(t, s, view())
	s.Focus("chart/plot")
	s.Key(KeyEvent{Key: "End"})
	mount(t, s, view())
	item := s.entries[s.index["chart/plot"]]
	if state.Selected != 2 || item.bounds.H != 180 || !strings.Contains(item.element.description, "$xxx") {
		t.Fatal(state, item)
	}
	img, err := s.RenderScaled(1.25, nil)
	if err != nil || img == nil {
		t.Fatal(err)
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	if state.Selected != -1 || calls != 2 {
		t.Fatal(state, calls)
	}
	mount(t, s, view().ReadOnly(true))
	s.Focus("chart/plot")
	s.Key(KeyEvent{Key: "Home"})
	click(t, s, "chart/plot")
	if calls != 2 {
		t.Fatal("read-only changed selection")
	}
	mount(t, s, view().Disabled(true))
	if s.Focus("chart/plot") {
		t.Fatal("disabled focused")
	}
}
func TestChartCoordinatesAndNearestSelection(t *testing.T) {
	if chartX(ChartLine, 0, 3) != 0 || chartX(ChartArea, 2, 3) != 1 || chartX(ChartLine, 0, 1) != .5 || chartX(ChartBar, 0, 2) != .25 {
		t.Fatal("category positions")
	}
	for _, test := range []struct {
		k    ChartKind
		f    float64
		want int
	}{{ChartLine, .25, 0}, {ChartBar, 1. / 3, 0}, {ChartLine, .24, 0}, {ChartLine, .26, 1}, {ChartBar, .32, 0}, {ChartBar, .34, 1}, {ChartLine, .99, 2}} {
		if got := chartPick(test.k, 3, test.f); got != test.want {
			t.Fatal(test, got)
		}
	}
}
func TestChartMissingValuesProduceActualGaps(t *testing.T) {
	for _, kind := range []ChartKind{ChartLine, ChartArea} {
		for curve := CurveLinear; curve <= CurveStep; curve++ {
			s := surface(t)
			mount(t, s, Chart("chart", []string{"A", "B", "C", "D", "E"}, []ChartSeries{{Color: color.NRGBA{R: 255, A: 255}, Values: []float64{1, 2, math.NaN(), 2, 1}}}, &ChartState{Selected: -1}, ChartOptions{Kind: kind, Curve: curve, HideGrid: true, HideLegend: true}).Width(400))
			img, err := s.Render()
			if err != nil {
				t.Fatal(err)
			}
			plot := chartPlot(s.entries[s.index["chart/plot"]].bounds, false)
			for y := int(plot.Y + 2); y < int(plot.Y+plot.H-2); y++ {
				c := img.RGBAAt(int(plot.X+plot.W/2), y)
				if int(c.R) > int(c.G)+10 && int(c.R) > int(c.B)+10 {
					t.Fatal("curve bridged gap", kind, curve, y)
				}
			}
		}
	}
}
func TestChartPaintClipGradientAndTransparentJoins(t *testing.T) {
	for _, scale := range []float64{1, 1.25, 2} {
		img := image.NewRGBA(image.Rect(0, 0, 100, 100))
		p := &Painter{image: img, clip: Rect{10, 10, 30, 30}, scale: scale}
		points := []chartPoint{{0, 0}, {20, 0}, {50, 0}}
		p.chartArea(points, 50, color.NRGBA{R: 255, A: 255})
		for y := 0; y < 100; y++ {
			for x := 0; x < 100; x++ {
				if float64(x+1) <= 10*scale || float64(x) >= 40*scale || float64(y+1) <= 10*scale || float64(y) >= 40*scale {
					if img.RGBAAt(x, y).A != 0 {
						t.Fatal("paint escaped plot", scale, x, y)
					}
				}
			}
		}
		upper, lower := img.RGBAAt(int(20*scale), int(12*scale)), img.RGBAAt(int(20*scale), int(35*scale))
		if upper.A <= lower.A {
			t.Fatal("gradient missing", upper, lower)
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, 80, 40))
	p := &Painter{image: img, clip: Rect{0, 0, 80, 40}, scale: 1}
	p.chartStroke([]chartPoint{{5, 20}, {40, 20}, {70, 20}}, 4, color.NRGBA{R: 255, A: 100})
	if img.RGBAAt(39, 20).A != img.RGBAAt(20, 20).A {
		t.Fatal("join accumulated alpha")
	}
}
func BenchmarkChartCurvedAreaRender(b *testing.B) {
	labels := make([]string, 200)
	values := make([]float64, 200)
	for i := range labels {
		labels[i] = "sample"
		values[i] = math.Sin(float64(i) * .1)
	}
	s, err := NewSurface(640, 320, NewTheme(false))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	if err = s.Mount(Chart("chart", labels, []ChartSeries{{Values: values}}, &ChartState{Selected: -1}, ChartOptions{Kind: ChartArea, Curve: CurveMonotone})); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err = s.Render(); err != nil {
			b.Fatal(err)
		}
	}
}

func TestChartHoverKeyboardPrecedenceAndLeave(t *testing.T) {
	s := surface(t)
	state := &ChartState{Selected: -1}
	calls := []int{}
	view := func() *Element {
		return Chart("chart", []string{"A", "B", "C"}, []ChartSeries{{Values: []float64{1, 2, 3}}}, state, ChartOptions{Kind: ChartLine, OnSelect: func(v int) { calls = append(calls, v) }}).Width(400)
	}
	mount(t, s, view())
	s.Focus("chart/plot")
	s.Key(KeyEvent{Key: "Left"})
	if state.Selected != 2 {
		t.Fatal("left from no selection must select last")
	}
	s.Key(KeyEvent{Key: "Escape"})
	r, _ := s.Bounds("chart/plot")
	plot := chartPlot(r, false)
	move := PointerEvent{Kind: "move", X: plot.X + 10, Y: plot.Y + 30}
	s.Pointer(move)
	if state.Selected != 0 {
		t.Fatal("hover not selected")
	}
	mount(t, s, view())
	s.Key(KeyEvent{Key: "End"})
	if state.Selected != 2 {
		t.Fatal("keyboard did not override hover")
	}
	before := len(calls)
	s.Pointer(move)
	if state.Selected != 2 || len(calls) != before {
		t.Fatal("stationary pointer stole keyboard selection")
	}
	move.X += plot.W / 2
	s.Pointer(move)
	if state.Selected != 1 {
		t.Fatal("moved pointer did not select nearest")
	}
	s.Pointer(PointerEvent{Kind: "leave"})
	if state.Selected != -1 {
		t.Fatal("leave did not clear")
	}
}
func TestChartHoverUnmountAndDisableDoNotEmitUserActions(t *testing.T) {
	s := surface(t)
	state := &ChartState{Selected: -1}
	calls := 0
	view := func() *Element {
		return Chart("chart", []string{"A", "B"}, []ChartSeries{{Values: []float64{1, 2}}}, state, ChartOptions{OnSelect: func(int) { calls++ }})
	}
	mount(t, s, view())
	s.Pointer(PointerEvent{Kind: "move", X: 100, Y: 40})
	if !state.hovering || calls != 1 {
		t.Fatal(state, calls)
	}
	mount(t, s, view().Disabled(true))
	if state.hovering || calls != 1 {
		t.Fatal("disable emitted action or retained hover")
	}
	mount(t, s, view())
	s.Pointer(PointerEvent{Kind: "move", X: 500, Y: 40})
	before := calls
	mount(t, s, Text("other", "Other"))
	if state.hovering || calls != before || s.hoverListener != "" {
		t.Fatal("unmount cleanup")
	}
}
