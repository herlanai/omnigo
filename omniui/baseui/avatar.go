package baseui

import (
	"image"
	"image/draw"
	"math"
	"strings"
	"unicode/utf8"
)

// Avatar takes an owned image snapshot. Nil or empty images use up to two
// initials. Images are cropped to a circle; no network or file IO is performed.
func Avatar(id, name string, source image.Image) *Element {
	var snapshot *image.RGBA
	if source != nil && !source.Bounds().Empty() {
		b := source.Bounds()
		if b.Dx() > 4096 || b.Dy() > 4096 {
			panic("baseui: avatar image exceeds 4096 pixels")
		}
		snapshot = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(snapshot, snapshot.Bounds(), source, b.Min, draw.Src)
	}
	initials := ""
	words := strings.Fields(name)
	for _, w := range words {
		r, _ := utf8.DecodeRuneInString(w)
		initials += strings.ToUpper(string(r))
		if utf8.RuneCountInString(initials) >= 2 {
			break
		}
	}
	return Canvas(id, name, func(p *Painter, r Rect) {
		side := min(r.W, r.H)
		if side <= 0 {
			return
		}
		r = Rect{r.X + (r.W-side)/2, r.Y + (r.H-side)/2, side, side}
		p.Fill(r, p.theme.Muted, side/2)
		if snapshot == nil {
			width := float64(len([]rune(initials))) * p.theme.FontSize * .6
			p.Text(r.X+(r.W-width)/2, r.Y+r.H/2+p.theme.FontSize*.35, initials, p.theme.Text)
			return
		}
		// Nearest-neighbor sampling preserves image alpha and logical clipping.
		scale := p.scale
		if scale == 0 {
			scale = 1
		}
		b := snapshot.Bounds()
		crop := float64(min(b.Dx(), b.Dy()))
		ox, oy := (float64(b.Dx())-crop)/2, (float64(b.Dy())-crop)/2
		area := intersect(r, p.clip)
		for y := max(0, int(math.Floor(area.Y*scale))); y < min(p.image.Bounds().Dy(), int(math.Ceil((area.Y+area.H)*scale))); y++ {
			for x := max(0, int(math.Floor(area.X*scale))); x < min(p.image.Bounds().Dx(), int(math.Ceil((area.X+area.W)*scale))); x++ {
				px, py := (float64(x)+.5)/scale, (float64(y)+.5)/scale
				coverage := max(0, min(1, .5-roundedDistance(r, side/2, px, py)*scale))
				coverage *= max(0, min(float64(x+1), (p.clip.X+p.clip.W)*scale)-max(float64(x), p.clip.X*scale)) * max(0, min(float64(y+1), (p.clip.Y+p.clip.H)*scale)-max(float64(y), p.clip.Y*scale))
				sx, sy := max(0, min(b.Dx()-1, int(ox+(px-r.X)/side*crop))), max(0, min(b.Dy()-1, int(oy+(py-r.Y)/side*crop)))
				c := snapshot.RGBAAt(sx, sy)
				alpha := float64(c.A) / 255 * coverage
				at := p.image.PixOffset(x, y)
				for i, v := range []uint8{c.R, c.G, c.B} {
					p.image.Pix[at+i] = uint8(math.Round(float64(v)*coverage + float64(p.image.Pix[at+i])*(1-alpha)))
				}
				p.image.Pix[at+3] = uint8(math.Round(255*alpha + float64(p.image.Pix[at+3])*(1-alpha)))
			}
		}
	}).Width(32).Height(32)
}
