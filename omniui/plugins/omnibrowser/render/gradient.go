package render

import (
	"image"
	"image/color"
	"math"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strconv"
	"strings"
)

type gradientStop struct {
	color     color.NRGBA
	at        float64
	specified bool
}
type gradient struct {
	radial bool
	angle  float64
	stops  []gradientStop
}

func parseGradient(value string) *gradient {
	value = strings.TrimSpace(value)
	radial := strings.HasPrefix(value, "radial-gradient(")
	if !radial && !strings.HasPrefix(value, "linear-gradient(") {
		return nil
	}
	if !strings.HasSuffix(value, ")") {
		return nil
	}
	parts := css.Split(value[strings.IndexByte(value, '(')+1:len(value)-1], ',')
	if len(parts) < 2 || len(parts) > 64 {
		return nil
	}
	g := &gradient{radial: radial, angle: math.Pi}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if i == 0 && (strings.HasSuffix(part, "deg") || strings.HasPrefix(part, "to ") || strings.HasPrefix(part, "circle") || strings.HasPrefix(part, "ellipse")) {
			if strings.HasSuffix(part, "deg") {
				n, e := strconv.ParseFloat(strings.TrimSuffix(part, "deg"), 64)
				if e != nil {
					return nil
				}
				g.angle = n * math.Pi / 180
			}
			switch part {
			case "to right":
				g.angle = math.Pi / 2
			case "to left":
				g.angle = -math.Pi / 2
			case "to top":
				g.angle = 0
			}
			continue
		}
		fields := css.Split(part, ' ')
		stop := gradientStop{}
		if len(fields) > 1 && strings.HasSuffix(fields[len(fields)-1], "%") {
			n, e := strconv.ParseFloat(strings.TrimSuffix(fields[len(fields)-1], "%"), 64)
			if e != nil {
				return nil
			}
			stop.at = n / 100
			stop.specified = true
			fields = fields[:len(fields)-1]
		}
		stop.color = paintColor(strings.Join(fields, " "), color.NRGBA{})
		g.stops = append(g.stops, stop)
	}
	if len(g.stops) < 2 {
		return nil
	}
	if !g.stops[0].specified {
		g.stops[0].at = 0
		g.stops[0].specified = true
	}
	last := len(g.stops) - 1
	if !g.stops[last].specified {
		g.stops[last].at = 1
		g.stops[last].specified = true
	}
	for a := 0; a < last; {
		b := a + 1
		for b < last && !g.stops[b].specified {
			b++
		}
		g.stops[b].at = max(g.stops[a].at, g.stops[b].at)
		for i := a + 1; i < b; i++ {
			g.stops[i].at = g.stops[a].at + (g.stops[b].at-g.stops[a].at)*float64(i-a)/float64(b-a)
		}
		a = b
	}
	return g
}
func (g *gradient) at(r image.Rectangle, x, y int) color.NRGBA {
	dx, dy := float64(x-r.Min.X)+.5-float64(r.Dx())/2, float64(y-r.Min.Y)+.5-float64(r.Dy())/2
	t := 0.0
	if g.radial {
		t = math.Hypot(dx, dy) / max(1, math.Hypot(float64(r.Dx()), float64(r.Dy()))/2)
	} else {
		sx, sy := math.Sin(g.angle), -math.Cos(g.angle)
		t = .5 + (dx*sx+dy*sy)/max(1, math.Abs(float64(r.Dx())*sx)+math.Abs(float64(r.Dy())*sy))
	}
	a := g.stops[0]
	if t <= a.at {
		return a.color
	}
	for _, b := range g.stops[1:] {
		if t <= b.at {
			u := (t - a.at) / max(.000001, b.at-a.at)
			alpha := float64(a.color.A)*(1-u) + float64(b.color.A)*u
			mix := func(c, d uint8) uint8 {
				if alpha == 0 {
					return 0
				}
				return uint8(max(0, min(255, (float64(c)*float64(a.color.A)*(1-u)+float64(d)*float64(b.color.A)*u)/alpha)))
			}
			return color.NRGBA{mix(a.color.R, b.color.R), mix(a.color.G, b.color.G), mix(a.color.B, b.color.B), uint8(alpha)}
		}
		a = b
	}
	return a.color
}

func backgroundGradient(s map[string]string) *gradient {
	if v := s["background-image"]; v != "" {
		return parseGradient(v)
	}
	return parseGradient(s["background"])
}
