package baseui

import (
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
)

// Go Mono is distributed by x/image with the Go font family license.
var codeFont = func() *opentype.Font {
	f, err := opentype.Parse(gomono.TTF)
	if err != nil {
		panic(err)
	}
	return f
}()

// codeText keeps display tab expansion separate from the original copy payload.
// It paints only visible rows and leaves selection/highlighting to future work.
func codeText(id, source string, danger bool) *Element {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	face, err := opentype.NewFace(codeFont, &opentype.FaceOptions{Size: 12, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	width := 1.0
	for i, line := range lines {
		var b strings.Builder
		column := 0
		for _, ch := range line {
			if ch == '\t' {
				n := 4 - column%4
				b.WriteString(strings.Repeat(" ", n))
				column += n
			} else {
				b.WriteRune(ch)
				column++
			}
		}
		lines[i] = b.String()
		width = max(width, float64(font.MeasureString(face, lines[i]).Ceil()))
	}
	face.Close()
	e := Canvas(id, "Command output", func(p *Painter, r Rect) {
		scale := p.scale
		if scale == 0 {
			scale = 1
		}
		f, err := opentype.NewFace(codeFont, &opentype.FaceOptions{Size: 12 * scale, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return
		}
		defer f.Close()
		q := *p
		q.face = f
		fg := p.theme.Text
		if danger {
			fg = p.theme.Danger
		}
		first := max(0, int(math.Floor((p.clip.Y-r.Y)/18)))
		last := min(len(lines), int(math.Ceil((p.clip.Y+p.clip.H-r.Y)/18)))
		for i := first; i < last; i++ {
			q.Text(r.X, r.Y+float64(i)*18+float64(f.Metrics().Ascent)/64/scale, lines[i], fg)
		}
	}).Width(width).Height(float64(len(lines)) * 18).Role("text")
	e.description = source
	return e
}
