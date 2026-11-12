package baseui

import (
	"golang.org/x/image/font"
	"image/color"
)

// StyledText paints text at a baseline without retaining a font or Painter.
func (p *Painter) StyledText(x, y float64, text string, style TextSpan, size float64, c color.NRGBA) {
	if !finite(size) || size < 6 || size > 96 {
		return
	}
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	face := richFace(richStyle(style), size*scale)
	defer face.Close()
	q := *p
	q.face = face
	q.Text(x, y, text, c)
}
func MonospaceMetrics(size float64) (advance, height, baseline float64) {
	if !finite(size) || size < 6 || size > 96 {
		return 0, 0, 0
	}
	face := richFace(4, size)
	defer face.Close()
	return float64(font.MeasureString(face, "M")) / 64, size * 1.5, float64(face.Metrics().Ascent) / 64
}
