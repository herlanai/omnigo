package render

import (
	"bytes"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/net/html"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
)

func rasterSVG(n *html.Node, w, h int, fg color.NRGBA) (img image.Image) {
	if w <= 0 || h <= 0 || w > 2048 || h > 2048 || w*h > 1<<20 {
		return nil
	}
	defer func() {
		if recover() != nil {
			img = nil
		}
	}()
	var b bytes.Buffer
	html.Render(&b, n)
	if b.Len() > 256<<10 {
		return nil
	}
	source := strings.ReplaceAll(b.String(), "currentColor", colorHex(fg))
	icon, err := oksvg.ReadIconStream(strings.NewReader(source), oksvg.WarnErrorMode)
	if err != nil {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	if icon.ViewBox.W <= 0 || icon.ViewBox.H <= 0 {
		return nil
	}
	if nodeAttr(n, "preserveAspectRatio") == "none" {
		icon.SetTarget(0, 0, float64(w), float64(h))
	} else {
		scale := math.Min(float64(w)/icon.ViewBox.W, float64(h)/icon.ViewBox.H)
		tw, th := icon.ViewBox.W*scale, icon.ViewBox.H*scale
		icon.SetTarget((float64(w)-tw)/2, (float64(h)-th)/2, tw, th)
	}
	scanner := rasterx.NewScannerGV(w, h, out, out.Bounds())
	icon.Draw(rasterx.NewDasher(w, h, scanner), 1)
	return out
}
func colorHex(c color.NRGBA) string {
	const hex = "0123456789abcdef"
	return string([]byte{'#', hex[c.R>>4], hex[c.R&15], hex[c.G>>4], hex[c.G&15], hex[c.B>>4], hex[c.B&15]})
}
func roundedCoverage(rect image.Rectangle, r float64, x, y int) float64 {
	if rect.Empty() {
		return 0
	}
	r = max(0, min(r, float64(min(rect.Dx(), rect.Dy()))/2))
	cx, cy := float64(rect.Min.X+rect.Max.X)/2, float64(rect.Min.Y+rect.Max.Y)/2
	qx := math.Abs(float64(x)+.5-cx) - float64(rect.Dx())/2 + r
	qy := math.Abs(float64(y)+.5-cy) - float64(rect.Dy())/2 + r
	distance := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - r
	return max(0, min(1, .5-distance))
}
func paintDecoration(dst *image.RGBA, box paintBox) {
	area := box.rect.Intersect(dst.Bounds())
	if area.Empty() {
		return
	}
	if box.radius == 0 && box.corners == ([4]float64{}) && box.border == 0 && box.gradient == nil {
		if box.bg.A != 0 {
			draw.Draw(dst, area, &image.Uniform{box.bg}, image.Point{}, draw.Over)
		}
		return
	}
	fill := image.NewAlpha(area)
	ring := image.NewAlpha(area)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			radius := box.radius
			index := 0
			if x >= (box.rect.Min.X+box.rect.Max.X)/2 {
				index = 1
			}
			if y >= (box.rect.Min.Y+box.rect.Max.Y)/2 {
				index = 3 - index
			}
			if box.corners != ([4]float64{}) {
				radius = box.corners[index]
			}
			outer := roundedCoverage(box.rect, radius, x, y)
			if box.ellipse {
				outer = ellipseCoverage(box.rect, x, y)
			}
			fill.SetAlpha(x, y, color.Alpha{uint8(outer * 255)})
			if box.border > 0 {
				inner := roundedCoverage(box.rect.Inset(box.border), max(0, radius-float64(box.border)), x, y)
				if box.ellipse {
					inner = ellipseCoverage(box.rect.Inset(box.border), x, y)
				}
				ring.SetAlpha(x, y, color.Alpha{uint8(max(0, outer-inner) * 255)})
			}
		}
	}
	draw.DrawMask(dst, area, &image.Uniform{box.bg}, image.Point{}, fill, area.Min, draw.Over)
	if box.gradient != nil {
		layer := image.NewNRGBA(area)
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				layer.SetNRGBA(x, y, box.gradient.at(box.rect, x, y))
			}
		}
		draw.DrawMask(dst, area, layer, area.Min, fill, area.Min, draw.Over)
	}
	if box.border > 0 {
		draw.DrawMask(dst, area, &image.Uniform{box.borderColor}, image.Point{}, ring, area.Min, draw.Over)
	}
}

func ellipseCoverage(r image.Rectangle, x, y int) float64 {
	if r.Empty() {
		return 0
	}
	rx, ry := float64(r.Dx())/2, float64(r.Dy())/2
	dx, dy := (float64(x)+.5-float64(r.Min.X)-rx)/rx, (float64(y)+.5-float64(r.Min.Y)-ry)/ry
	return max(0, min(1, .5+(1-math.Hypot(dx, dy))*min(rx, ry)))
}

func invertImage(src image.Image) image.Image {
	if src == nil {
		return nil
	}
	dst := image.NewNRGBA(src.Bounds())
	for y := dst.Bounds().Min.Y; y < dst.Bounds().Max.Y; y++ {
		for x := dst.Bounds().Min.X; x < dst.Bounds().Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			dst.SetNRGBA(x, y, color.NRGBA{255 - c.R, 255 - c.G, 255 - c.B, c.A})
		}
	}
	return dst
}
