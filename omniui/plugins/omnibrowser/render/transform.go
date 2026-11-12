package render

import (
	"golang.org/x/image/math/f64"
	"image"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var transformFunctions = regexp.MustCompile(`(rotate|scaleX|scaleY|scale|translateX|translateY|translate)\(([^)]+)\)`)

func multiply(a, b f64.Aff3) f64.Aff3 {
	return f64.Aff3{a[0]*b[0] + a[1]*b[3], a[0]*b[1] + a[1]*b[4], a[0]*b[2] + a[1]*b[5] + a[2], a[3]*b[0] + a[4]*b[3], a[3]*b[1] + a[4]*b[4], a[3]*b[2] + a[4]*b[5] + a[5]}
}
func cssTransform(value string, r image.Rectangle) f64.Aff3 {
	m := f64.Aff3{1, 0, 0, 0, 1, 0}
	for _, match := range transformFunctions.FindAllStringSubmatch(value, -1) {
		parts := strings.Fields(strings.ReplaceAll(match[2], ",", " "))
		if len(parts) == 0 {
			continue
		}
		parse := func(s string) float64 {
			n, _ := strconv.ParseFloat(s, 64)
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return 0
			}
			return max(-10000, min(10000, n))
		}
		t := f64.Aff3{1, 0, 0, 0, 1, 0}
		switch match[1] {
		case "rotate":
			a := parse(strings.TrimSuffix(parts[0], "deg")) * math.Pi / 180
			t = f64.Aff3{math.Cos(a), -math.Sin(a), 0, math.Sin(a), math.Cos(a), 0}
		case "scale", "scaleX", "scaleY":
			a := parse(parts[0])
			b := a
			if len(parts) > 1 {
				b = parse(parts[1])
			}
			if match[1] == "scaleX" {
				b = 1
			}
			if match[1] == "scaleY" {
				a = 1
			}
			t[0], t[4] = a, b
		default:
			x, y := translation(match[0], r.Dx(), r.Dy())
			t[2], t[5] = float64(x), float64(y)
		}
		m = multiply(m, t)
	}
	cx, cy := float64(r.Min.X+r.Max.X)/2, float64(r.Min.Y+r.Max.Y)/2
	return multiply(multiply(f64.Aff3{1, 0, cx, 0, 1, cy}, m), f64.Aff3{1, 0, -cx, 0, 1, -cy})
}
