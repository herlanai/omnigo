package baseui

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Painter is a clipped CPU drawing context, also usable by Go-only extensions.
// Coordinates are logical pixels at scale 1. There is no GPU/platform adapter yet.
type Painter struct {
	image *image.RGBA
	clip  Rect
	face  font.Face
	now   time.Duration
	theme Theme
}

func (p *Painter) Now() time.Duration { return p.now }
func (p *Painter) Theme() Theme       { return p.theme }
func (p *Painter) Fill(r Rect, c color.NRGBA, radius float64) {
	if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || !finite(radius) || r.W <= 0 || r.H <= 0 {
		return
	}
	radius = max(0, min(radius, min(r.W, r.H)/2))
	area := intersect(r, p.clip)
	for y := max(0, int(math.Floor(area.Y))); y < min(p.image.Bounds().Dy(), int(math.Ceil(area.Y+area.H))); y++ {
		for x := max(0, int(math.Floor(area.X))); x < min(p.image.Bounds().Dx(), int(math.Ceil(area.X+area.W))); x++ {
			px, py := float64(x)+.5, float64(y)+.5
			if !p.clip.Contains(px, py) {
				continue
			}
			coverage := 1.
			if radius > 0 {
				cx := max(r.X+radius, min(r.X+r.W-radius, px))
				cy := max(r.Y+radius, min(r.Y+r.H-radius, py))
				coverage = max(0, min(1, radius+.5-math.Hypot(px-cx, py-cy)))
			}
			alpha := float64(c.A) / 255 * coverage
			if alpha == 0 {
				continue
			}
			at := p.image.PixOffset(x, y)
			for i, v := range []uint8{c.R, c.G, c.B} {
				p.image.Pix[at+i] = uint8(math.Round(float64(v)*alpha + float64(p.image.Pix[at+i])*(1-alpha)))
			}
			p.image.Pix[at+3] = 255
		}
	}
}
func (p *Painter) Line(x1, y1, x2, y2, width float64, c color.NRGBA) {
	if !finite(x1) || !finite(y1) || !finite(x2) || !finite(y2) || !finite(width) || width <= 0 {
		return
	}
	distance := math.Hypot(x2-x1, y2-y1)
	if !finite(distance) {
		return
	}
	// Bound raster work even for an extension passing extremely distant points.
	steps := max(1, int(math.Ceil(min(distance*2, 65536))))
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		p.Fill(Rect{x1 + (x2-x1)*t - width/2, y1 + (y2-y1)*t - width/2, width, width}, c, width/2)
	}
}
func (p *Painter) Text(x, y float64, text string, c color.NRGBA) {
	clip := image.Rect(int(math.Ceil(p.clip.X)), int(math.Ceil(p.clip.Y)), int(math.Floor(p.clip.X+p.clip.W)), int(math.Floor(p.clip.Y+p.clip.H))).Intersect(p.image.Bounds())
	if clip.Empty() {
		return
	}
	d := font.Drawer{Dst: p.image.SubImage(clip).(*image.RGBA), Src: image.NewUniform(c), Face: p.face, Dot: fixed.P(int(math.Round(x)), int(math.Round(y)))}
	d.DrawString(text)
}

func blend(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{uint8(float64(a.R)*(1-t) + float64(b.R)*t), uint8(float64(a.G)*(1-t) + float64(b.G)*t), uint8(float64(a.B)*(1-t) + float64(b.B)*t), 255}
}

// Render returns a newly owned image. Call only when content/input/animation
// changes; stable surfaces do not schedule timers or render themselves.
func (s *Surface) Render() (*image.RGBA, error) {
	if s.closed {
		return nil, errors.New("baseui: closed surface")
	}
	img := image.NewRGBA(image.Rect(0, 0, s.width, s.height))
	draw.Draw(img, img.Bounds(), image.NewUniform(s.theme.Background), image.Point{}, draw.Src)
	for _, entry := range s.entries {
		if entry.element.kind == ModalKind {
			p := Painter{img, Rect{W: float64(s.width), H: float64(s.height)}, s.face, s.now, s.theme}
			p.Fill(p.clip, color.NRGBA{0, 0, 0, 100}, 0)
		}
		p := Painter{img, entry.clip, s.face, s.now, s.theme}
		if err := guarded(func() { s.drawElement(&p, entry) }); err != nil {
			return nil, err
		}
	}
	return img, nil
}

func (s *Surface) drawElement(p *Painter, item entry) {
	e, r, t := item.element, item.bounds, s.theme
	fg, bg, border := t.Text, t.Surface, t.Border
	if item.disabled {
		fg = blend(fg, t.Background, .55)
	}
	if e.invalid {
		border = t.Danger
	}
	frame := func(fill color.NRGBA) { p.Fill(r, border, t.Radius); p.Fill(r.inset(1), fill, max(0, t.Radius-1)) }
	label := func(text string, x float64) {
		p.Text(x, r.Y+(r.H-float64(s.face.Metrics().Height.Ceil()))/2+float64(s.face.Metrics().Ascent.Ceil()), text, fg)
	}
	switch e.kind {
	case CardKind, ModalKind:
		frame(bg)
	case TextKind:
		for i, line := range strings.Split(e.text, "\n") {
			p.Text(r.X, r.Y+float64(s.face.Metrics().Ascent.Ceil())+float64(i)*s.lineHeight(), line, fg)
		}
	case SeparatorKind:
		p.Fill(r, t.Border, 0)
	case ButtonKind, BadgeKind:
		switch e.variant {
		case Primary:
			bg = t.Accent
			fg = t.AccentText
			border = bg
		case Secondary:
			bg = t.Muted
			border = bg
		case Ghost:
			bg = t.Background
			border = bg
		case Destructive:
			bg = t.Danger
			fg = color.NRGBA{255, 255, 255, 255}
			border = bg
		}
		if e.selected {
			bg = t.Selection
			fg = t.Text
			border = t.Accent
		}
		if s.hover == e.id && !item.disabled {
			bg = blend(bg, fg, .08)
		}
		if s.pressed == e.id && !item.disabled {
			bg = blend(bg, fg, .16)
		}
		if item.disabled {
			bg = blend(bg, t.Background, .55)
			fg = blend(fg, t.Background, .4)
			border = blend(border, t.Background, .55)
		}
		frame(bg)
		label(e.text, r.X+max(2, (r.W-s.textWidth(e.text))/2))
	case CheckboxKind, RadioKind, SwitchKind:
		box := Rect{r.X, r.Y + (r.H-18)/2, 18, 18}
		radius := 4.
		if e.kind == RadioKind {
			radius = 9
		}
		if e.kind == SwitchKind {
			box.W = 36
			radius = 9
		}
		fill := bg
		if e.checked || e.mixed {
			fill = t.Accent
		}
		if item.disabled {
			fill = blend(fill, t.Background, .55)
		}
		p.Fill(box, border, radius)
		p.Fill(box.inset(1), fill, max(0, radius-1))
		switch e.kind {
		case SwitchKind:
			x := box.X + 3
			if e.checked {
				x += 18
			}
			p.Fill(Rect{x, box.Y + 3, 12, 12}, t.AccentText, 6)
		case RadioKind:
			if e.checked {
				p.Fill(box.inset(5), t.AccentText, 4)
			}
		default:
			if e.mixed {
				p.Line(box.X+4, box.Y+9, box.X+14, box.Y+9, 2, t.AccentText)
			} else if e.checked {
				p.Line(box.X+4, box.Y+9, box.X+8, box.Y+13, 2, t.AccentText)
				p.Line(box.X+8, box.Y+13, box.X+14, box.Y+5, 2, t.AccentText)
			}
		}
		label(e.text, box.X+box.W+10)
	case InputKind, TextareaKind:
		frame(bg)
		p.clip = intersect(p.clip, r.inset(8))
		b := e.buffer
		lines := strings.Split(b.text, "\n")
		anchor, caret := b.Selection()
		offset := 0
		lineHeight := s.lineHeight()
		top := r.Y + 8
		if e.kind == InputKind {
			top = r.Y + (r.H-lineHeight)/2
		}
		x := r.X + 10
		scroll := 0.
		if e.kind == InputKind {
			scroll = max(0, s.textWidth(string([]rune(b.text)[:caret]))-(r.W-24))
		}
		for row, line := range lines {
			runes := []rune(line)
			start, end := max(0, min(anchor, caret)-offset), min(len(runes), max(anchor, caret)-offset)
			if end > start {
				left := x - scroll + s.textWidth(string(runes[:start]))
				right := x - scroll + s.textWidth(string(runes[:end]))
				p.Fill(Rect{left, top + float64(row)*lineHeight, right - left, lineHeight}, t.Selection, 0)
			}
			baseline := top + float64(row)*lineHeight + float64(s.face.Metrics().Ascent.Ceil())
			p.Text(x-scroll, baseline, line, fg)
			if s.focus == e.id && caret >= offset && caret <= offset+len(runes) {
				cx := x - scroll + s.textWidth(string(runes[:caret-offset]))
				p.Fill(Rect{cx, top + float64(row)*lineHeight, 1, lineHeight}, t.Focus, 0)
				if b.composition != "" {
					p.Text(cx, baseline, b.composition, fg)
					p.Line(cx, baseline+2, cx+s.textWidth(b.composition), baseline+2, 1, t.Focus)
				}
			}
			offset += len(runes) + 1
		}
	case SliderKind:
		track := Rect{r.X + 10, r.Y + r.H/2 - 2, max(0, r.W-20), 4}
		p.Fill(track, t.Muted, 2)
		fraction := 0.
		if e.max > e.min {
			fraction = (e.value - e.min) / (e.max - e.min)
		}
		fraction = max(0, min(1, fraction))
		p.Fill(Rect{track.X, track.Y, track.W * fraction, 4}, t.Accent, 2)
		p.Fill(Rect{track.X + track.W*fraction - 7, r.Y + r.H/2 - 7, 14, 14}, t.Accent, 7)
	case ProgressKind:
		p.Fill(r, t.Muted, r.H/2)
		p.Fill(Rect{r.X, r.Y, r.W * e.value, r.H}, t.Accent, r.H/2)
	case SpinnerKind:
		phase := 0.
		if !t.ReduceMotion {
			phase = float64(s.now%time.Second) / float64(time.Second) * 2 * math.Pi
		}
		radius := max(1, min(r.W, r.H)/2-3)
		for i := 0; i < 10; i++ {
			a := phase + float64(i)*2*math.Pi/12
			c := blend(t.Muted, t.Accent, float64(i+1)/10)
			p.Fill(Rect{r.X + r.W/2 + math.Cos(a)*radius - 2, r.Y + r.H/2 + math.Sin(a)*radius - 2, 4, 4}, c, 2)
		}
	case CustomKind:
		if e.paint != nil {
			e.paint(p, r)
		}
	}
	if s.focus == e.id && !item.disabled {
		p.clip = item.clip
		focus := t.Focus
		if e.invalid {
			focus = t.Danger
		}
		p.Line(r.X+2, r.Y+2, r.X+r.W-2, r.Y+2, 2, focus)
		p.Line(r.X+2, r.Y+r.H-2, r.X+r.W-2, r.Y+r.H-2, 2, focus)
		p.Line(r.X+2, r.Y+2, r.X+2, r.Y+r.H-2, 2, focus)
		p.Line(r.X+r.W-2, r.Y+2, r.X+r.W-2, r.Y+r.H-2, 2, focus)
	}
}
