package render

import (
	"fmt"
	"golang.org/x/net/html"
	"math"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func animationSeconds(v string) (float64, bool) {
	scale := 1.0
	if strings.HasSuffix(v, "ms") {
		scale = .001
		v = strings.TrimSuffix(v, "ms")
	} else if strings.HasSuffix(v, "s") {
		v = strings.TrimSuffix(v, "s")
	} else {
		return 0, false
	}
	n, e := strconv.ParseFloat(v, 64)
	return n * scale, e == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}
func sampleAnimations(sheet css.Sheet, styles map[*html.Node]map[string]string, elapsed time.Duration) {
	for n, s := range styles {
		if n.Type != html.ElementNode || s["display"] == "none" {
			continue
		}
		name, timing, direction, fill, play := "", "ease", "normal", "none", "running"
		duration, delay, count := 0.0, 0.0, 1.0
		times := 0
		for _, part := range css.Split(strings.Split(s["animation"], ",")[0], ' ') {
			if part == "" {
				continue
			}
			if v, ok := animationSeconds(part); ok {
				if times == 0 {
					duration = v
				} else {
					delay = v
				}
				times++
				continue
			}
			switch part {
			case "linear", "ease", "ease-in", "ease-out", "ease-in-out":
				timing = part
			case "normal", "reverse", "alternate", "alternate-reverse":
				direction = part
			case "both", "forwards", "backwards", "none":
				fill = part
			case "paused", "running":
				play = part
			case "infinite":
				count = math.Inf(1)
			default:
				if v, e := strconv.ParseFloat(part, 64); e == nil {
					count = v
				} else {
					name = part
				}
			}
		}
		first := func(k string) string { return strings.TrimSpace(strings.Split(s[k], ",")[0]) }
		if v := first("animation-name"); v != "" {
			name = v
		}
		if v, ok := animationSeconds(first("animation-duration")); ok {
			duration = v
		}
		if v, ok := animationSeconds(first("animation-delay")); ok {
			delay = v
		}
		for key, dst := range map[string]*string{"animation-timing-function": &timing, "animation-direction": &direction, "animation-fill-mode": &fill, "animation-play-state": &play} {
			if v := first(key); v != "" {
				*dst = v
			}
		}
		if v := first("animation-iteration-count"); v != "" {
			if v == "infinite" {
				count = math.Inf(1)
			} else if n, e := strconv.ParseFloat(v, 64); e == nil {
				count = n
			}
		}
		frames := sheet.Keyframes[name]
		if len(frames) == 0 || duration <= 0 || count <= 0 {
			continue
		}
		local := elapsed.Seconds()
		if play == "paused" {
			local = 0
		}
		local -= delay
		if local < 0 && fill != "backwards" && fill != "both" {
			continue
		}
		if local >= duration*count && fill != "forwards" && fill != "both" {
			continue
		}
		cycle := math.Floor(max(0, local) / duration)
		fraction := math.Mod(max(0, local), duration) / duration
		if local >= duration*count {
			cycle = math.Ceil(count) - 1
			fraction = count - cycle
		}
		if direction == "reverse" || direction == "alternate" && int64(cycle)%2 == 1 || direction == "alternate-reverse" && int64(cycle)%2 == 0 {
			fraction = 1 - fraction
		}
		type point struct {
			at    float64
			value string
		}
		tracks := map[string][]point{}
		for _, frame := range frames {
			for _, d := range frame.Declarations {
				if d.Name != "transform" {
					continue
				}
				tracks[d.Name] = append(tracks[d.Name], point{frame.Offset, d.Value})
			}
		}
		for property, points := range tracks {
			if sheet.Important(n, property) {
				continue
			}
			base := s[property]
			if base == "" {
				base = "none"
			}
			if points[0].at > 0 {
				points = append([]point{{0, base}}, points...)
			}
			if points[len(points)-1].at < 1 {
				points = append(points, point{1, base})
			}
			for i := 1; i < len(points); i++ {
				a, b := points[i-1], points[i]
				if fraction > b.at {
					continue
				}
				u := 0.0
				if b.at > a.at {
					u = (fraction - a.at) / (b.at - a.at)
				}
				if v, ok := interpolateTransform(a.value, b.value, easeFraction(timing, u)); ok {
					s[property] = v
				}
				break
			}
		}
	}
}

var numericCSS = regexp.MustCompile(`[-+]?(?:\d*\.\d+|\d+)(?:[eE][-+]?\d+)?`)

func interpolateTransform(a, b string, u float64) (string, bool) {
	normalize := func(v string) string {
		return transformFunctions.ReplaceAllStringFunc(v, func(f string) string {
			if strings.HasPrefix(f, "rotate(") && !strings.Contains(f, "deg") {
				return strings.TrimSuffix(f, ")") + "deg)"
			}
			return f
		})
	}
	a, b = normalize(a), normalize(b)
	if a == "none" {
		a = identityTransform(b)
	}
	if b == "none" {
		b = identityTransform(a)
	}
	if numericCSS.ReplaceAllString(a, "#") != numericCSS.ReplaceAllString(b, "#") {
		return "", false
	}
	av, bv := numericCSS.FindAllString(a, -1), numericCSS.FindAllString(b, -1)
	if len(av) != len(bv) || len(av) > 64 {
		return "", false
	}
	i := 0
	value := numericCSS.ReplaceAllStringFunc(a, func(v string) string {
		x, _ := strconv.ParseFloat(v, 64)
		y, _ := strconv.ParseFloat(bv[i], 64)
		i++
		return fmt.Sprintf("%.6g", x+(y-x)*max(0, min(1, u)))
	})
	return value, true
}
func identityTransform(v string) string {
	return transformFunctions.ReplaceAllStringFunc(v, func(f string) string {
		identity := "0"
		if strings.HasPrefix(f, "scale") {
			identity = "1"
		}
		return numericCSS.ReplaceAllString(f, identity)
	})
}
func easeFraction(timing string, u float64) float64 {
	u = max(0, min(1, u))
	if timing == "linear" {
		return u
	}
	x1, y1, x2, y2 := .25, .1, .25, 1.0
	switch timing {
	case "ease-in":
		x1, y1, x2, y2 = .42, 0, 1, 1
	case "ease-out":
		x1, y1, x2, y2 = 0, 0, .58, 1
	case "ease-in-out":
		x1, y1, x2, y2 = .42, 0, .58, 1
	case "ease":
	default:
		return u
	}
	bezier := func(t, a, b float64) float64 { return 3*(1-t)*(1-t)*t*a + 3*(1-t)*t*t*b + t*t*t }
	lo, hi := 0.0, 1.0
	for range 24 {
		mid := (lo + hi) / 2
		if bezier(mid, x1, x2) < u {
			lo = mid
		} else {
			hi = mid
		}
	}
	return bezier((lo+hi)/2, y1, y2)
}
