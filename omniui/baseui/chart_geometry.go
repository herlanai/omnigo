package baseui

import "math"

// ChartCurve selects interpolation between adjacent finite data points.
type ChartCurve uint8

const (
	CurveLinear ChartCurve = iota
	CurveMonotone
	CurveStepAfter
	CurveStepBefore
	CurveStep
)

type chartPoint struct{ x, y float64 }
type chartScale struct {
	lo, hi float64
	ticks  []float64
}

// Choose round steps in normalized coordinates so subnormals and domains
// spanning both float64 extremes do not overflow during division/subtraction.
func chartNiceScale(lo, hi float64) chartScale {
	magnitude := max(math.Abs(lo), math.Abs(hi))
	if magnitude == 0 {
		return chartScale{0, 1, []float64{0, .2, .4, .6, .8, 1}}
	}
	a, b := lo/magnitude, hi/magnitude
	logStep := math.Log10(magnitude) + math.Log10((b-a)/4)
	exponent := math.Floor(logStep)
	power := math.Pow(10, exponent)
	if power == 0 {
		power = math.SmallestNonzeroFloat64
	}
	ratio := math.Pow(10, logStep-exponent)
	factor := 1.
	switch {
	case ratio >= math.Sqrt(50):
		factor = 10
	case ratio >= math.Sqrt(10):
		factor = 5
	case ratio >= math.Sqrt(2):
		factor = 2
	}
	step := factor * (power / magnitude)
	low, high := math.Floor(a/step), math.Ceil(b/step)
	niceLo, niceHi := low*step*magnitude, high*step*magnitude
	if !finite(niceLo) || niceLo > lo {
		niceLo = lo
	}
	if !finite(niceHi) || niceHi < hi {
		niceHi = hi
	}
	ticks := []float64{}
	for i := low; i <= high && len(ticks) < 32; i++ {
		v := i * step * magnitude
		if finite(v) && v >= niceLo && v <= niceHi && (len(ticks) == 0 || v > ticks[len(ticks)-1]) {
			if v == 0 {
				v = 0
			}
			ticks = append(ticks, v)
		}
	}
	if len(ticks) < 2 {
		ticks = []float64{niceLo, niceHi}
	}
	return chartScale{niceLo, niceHi, ticks}
}
func (s chartScale) at(v float64) float64 {
	m := max(math.Abs(s.lo), math.Abs(s.hi))
	if m == 0 || s.lo == s.hi {
		return .5
	}
	return (v/m - s.lo/m) / (s.hi/m - s.lo/m)
}
func chartX(kind ChartKind, index, count int) float64 {
	if kind == ChartBar {
		return (float64(index) + .5) / float64(count)
	}
	if count <= 1 {
		return .5
	}
	return float64(index) / float64(count-1)
}
func chartPick(kind ChartKind, count int, f float64) int {
	if count == 0 {
		return -1
	}
	position := f * float64(max(0, count-1))
	if kind == ChartBar {
		position = f*float64(count) - .5
	}
	lower := math.Floor(position)
	if position-lower > .5 {
		lower++
	}
	return max(0, min(count-1, int(lower)))
}

// Steffen's monotone cubic derivative uses adjacent secants and a bounded
// weighted estimate. It creates no extrema between monotone samples.
func chartSlopes(points []chartPoint) []float64 {
	n := len(points)
	out := make([]float64, n)
	if n < 2 {
		return out
	}
	sec := make([]float64, n-1)
	for i := range sec {
		dx := points[i+1].x - points[i].x
		if dx > 0 {
			sec[i] = (points[i+1].y - points[i].y) / dx
		}
	}
	if n == 2 {
		out[0], out[1] = sec[0], sec[0]
		return out
	}
	for i := 1; i < n-1; i++ {
		left, right := sec[i-1], sec[i]
		if left == 0 || right == 0 || math.Signbit(left) != math.Signbit(right) {
			continue
		}
		h0, h1 := points[i].x-points[i-1].x, points[i+1].x-points[i].x
		weighted := (left*h1 + right*h0) / (h0 + h1)
		out[i] = math.Copysign(min(2*math.Abs(left), 2*math.Abs(right), math.Abs(weighted)), left)
	}
	out[0] = (3*sec[0] - out[1]) / 2
	out[n-1] = (3*sec[n-2] - out[n-2]) / 2
	return out
}
func chartCurvePoints(points []chartPoint, curve ChartCurve, scale float64) []chartPoint {
	if len(points) < 2 || curve == CurveLinear {
		return points
	}
	out := make([]chartPoint, 0, len(points)*2)
	out = append(out, points[0])
	slopes := []float64(nil)
	if curve == CurveMonotone {
		slopes = chartSlopes(points)
	}
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		switch curve {
		case CurveStepAfter:
			out = append(out, chartPoint{b.x, a.y})
		case CurveStepBefore:
			out = append(out, chartPoint{a.x, b.y})
		case CurveStep:
			middle := (a.x + b.x) / 2
			out = append(out, chartPoint{middle, a.y}, chartPoint{middle, b.y})
		case CurveMonotone:
			dx := b.x - a.x
			segments := max(1, min(16384, int(math.Ceil(dx*max(1, scale)))))
			for j := 1; j < segments; j++ {
				t := float64(j) / float64(segments)
				t2, t3 := t*t, t*t*t
				y := (2*t3-3*t2+1)*a.y + (t3-2*t2+t)*dx*slopes[i-1] + (-2*t3+3*t2)*b.y + (t3-t2)*dx*slopes[i]
				out = append(out, chartPoint{a.x + t*dx, max(min(a.y, b.y), min(max(a.y, b.y), y))})
			}
		}
		out = append(out, b)
	}
	return out
}
