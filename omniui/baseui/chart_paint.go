package baseui

import (
	"golang.org/x/image/font"
	"image/color"
	"math"
	"sort"
)

func chartColor(theme Theme, index int, custom color.NRGBA) color.NRGBA {
	if custom != (color.NRGBA{}) {
		return custom
	}
	c := theme.Chart[index%len(theme.Chart)]
	if c == (color.NRGBA{}) {
		c = theme.Accent
	}
	return c
}
func chartPlot(r Rect, axis bool) Rect {
	left := 24.
	if axis {
		left = 56
	}
	return Rect{r.X + left, r.Y + 16, max(0, r.W-left-24), max(0, r.H-44)}
}
func chartTextWidth(p *Painter, text string) float64 {
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	return float64(font.MeasureString(p.face, text)) / 64 / scale
}
func (p *Painter) chartArea(points []chartPoint, zero float64, c color.NRGBA) {
	if len(points) < 2 {
		return
	}
	x0, x1 := points[0].x, points[len(points)-1].x
	if x1 <= x0 {
		return
	}
	top, bottom := zero, zero
	for _, v := range points {
		top = min(top, v.y)
		bottom = max(bottom, v.y)
	}
	if top == bottom {
		return
	}
	height := bottom - top
	// Curve points are monotone in X. Duplicate X coordinates represent steps.
	at := func(x float64) float64 {
		i := sort.Search(len(points), func(i int) bool { return points[i].x > x })
		if i == 0 {
			return points[0].y
		}
		if i == len(points) {
			return points[i-1].y
		}
		a, b := points[i-1], points[i]
		fraction := (x - a.x) / (b.x - a.x)
		return a.y*(1-fraction) + b.y*fraction
	}
	p.rasterShade(Rect{x0, top, x1 - x0, height}, func(x, y float64) float64 {
		edge := at(x)
		return max(x0-x, x-x1, min(edge, zero)-y, y-max(edge, zero))
	}, func(_, y float64) color.NRGBA {
		v := c
		v.A = uint8(math.Round(float64(c.A) * (.45 - .41*max(0, min(1, (y-top)/height)))))
		return v
	})
}
func chartTooltip(p *Painter, plot Rect, x float64, title string, rows []string, colors []color.NRGBA) {
	lineHeight := p.theme.FontSize * 1.5
	width := chartTextWidth(p, title)
	for _, row := range rows {
		width = max(width, chartTextWidth(p, row)+16)
	}
	width = min(plot.W, width+24)
	height := min(plot.H, 24+lineHeight*float64(len(rows)+1))
	if width < 24 || height < lineHeight {
		return
	}
	left := x + 12
	if left+width > plot.X+plot.W {
		left = x - width - 12
	}
	left = max(plot.X, min(left, plot.X+plot.W-width))
	box := Rect{left, plot.Y + 4, width, height}
	q := *p
	q.clip = intersect(q.clip, plot)
	q.Fill(box, q.theme.Surface, 12)
	q.Stroke(box, q.theme.Border, 12, 1)
	q.clip = intersect(q.clip, box.inset(8))
	baseline := box.Y + 12 + p.theme.FontSize
	q.Text(box.X+12, baseline, title, q.theme.Text)
	for i, row := range rows {
		baseline += lineHeight
		q.Fill(Rect{box.X + 12, baseline - 8, 8, 8}, colors[i], 2)
		q.Text(box.X+28, baseline, row, q.theme.Text)
	}
}

// Rasterize a whole run once, so translucent joins never accumulate alpha.
func (p *Painter) chartStroke(points []chartPoint, width float64, c color.NRGBA) {
	if len(points) < 2 {
		return
	}
	lo, hi := points[0].y, points[0].y
	for _, v := range points {
		lo = min(lo, v.y)
		hi = max(hi, v.y)
	}
	scale := max(1, p.scale)
	margin := width/2 + 1/scale
	bounds := Rect{points[0].x - width/2, lo - width/2, points[len(points)-1].x - points[0].x + width, hi - lo + width}
	p.raster(bounds, c, func(x, y float64) float64 {
		end := sort.Search(len(points), func(i int) bool { return points[i].x > x+margin })
		start := max(1, sort.Search(len(points), func(i int) bool { return points[i].x >= x-margin }))
		distance := math.Inf(1)
		for i := start; i <= min(len(points)-1, end); i++ {
			a, b := points[i-1], points[i]
			dx, dy := b.x-a.x, b.y-a.y
			squared := dx*dx + dy*dy
			along := 0.
			if squared > 0 {
				along = max(0, min(1, ((x-a.x)*dx+(y-a.y)*dy)/squared))
			}
			distance = min(distance, math.Hypot(x-a.x-along*dx, y-a.y-along*dy))
		}
		return distance - width/2
	})
}
