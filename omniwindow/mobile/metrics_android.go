//go:build android

package mobile

// x/mobile reports physical 72-dpi points; Android controls use 160-dpi dp.
func nativeMetrics(pixelsPerPoint float64, _ bool) metrics {
	return metrics{scale: pixelsPerPoint * 72 / 160}
}
