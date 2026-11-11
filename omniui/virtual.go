package omniui

import "math"

// VisibleRange returns [start,end) and the top spacer in logical pixels for a
// fixed-height virtual list. Work and memory are independent of total row count.
// Invalid geometry produces an empty range; negative scroll clamps to zero.
func VisibleRange(count int, rowHeight, scroll, viewport float64, overscan int) (start, end int, spacer float64) {
	if count <= 0 || rowHeight <= 0 || viewport <= 0 || math.IsNaN(rowHeight) || math.IsNaN(scroll) || math.IsNaN(viewport) || math.IsInf(rowHeight, 0) || math.IsInf(scroll, 0) || math.IsInf(viewport, 0) {
		return 0, 0, 0
	}
	if overscan < 0 {
		overscan = 0
	}
	scroll = math.Max(0, scroll)
	first := math.Min(float64(count), math.Floor(scroll/rowHeight))
	last := math.Min(float64(count), math.Ceil((scroll+viewport)/rowHeight))
	// Bound in floating point before converting, including very large scrolls.
	index := func(value float64) int {
		if value <= 0 {
			return 0
		}
		// float64(MaxInt) rounds up on 64-bit hosts; never convert that value.
		if value >= float64(count) {
			return count
		}
		return int(value)
	}
	start = index(first - float64(overscan))
	end = index(last + float64(overscan))
	return start, end, float64(start) * rowHeight
}
