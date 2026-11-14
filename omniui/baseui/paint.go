package baseui

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Painter is a clipped CPU drawing context, also usable by Go-only extensions.
// Coordinates are logical pixels; RenderScaled maps them to physical pixels.
type Painter struct {
	image *image.RGBA
	clip  Rect
	face  font.Face
	now   time.Duration
	theme Theme
	scale float64
}

func (p *Painter) Now() time.Duration { return p.now }
func (p *Painter) Theme() Theme       { return p.theme }

// raster evaluates coverage once per pixel, so translucent strokes do not
// darken at overlapping samples. Geometry and clipping remain in DIP.
func (p *Painter) raster(r Rect, c color.NRGBA, distance func(float64, float64) float64) {
	if c.A == 0 {
		return
	}
	p.rasterShade(r, distance, func(float64, float64) color.NRGBA { return c })
}
func (p *Painter) rasterShade(r Rect, distance func(float64, float64) float64, shade func(float64, float64) color.NRGBA) {
	p.rasterShadeOutside(r, distance, shade, image.Rectangle{})
}

// skip contains whole pixels already filled, or known to be outside a stroke.
// Edge pixels still use the same coverage/blending path at every DPI.
func (p *Painter) rasterShadeOutside(r Rect, distance func(float64, float64) float64, shade func(float64, float64) color.NRGBA, skip image.Rectangle) {
	if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || r.W <= 0 || r.H <= 0 {
		return
	}
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	area := intersect(r.inset(-1/scale), p.clip)
	xmin, xmax := max(0, int(math.Floor(area.X*scale))), min(p.image.Bounds().Dx(), int(math.Ceil((area.X+area.W)*scale)))
	ymin, ymax := max(0, int(math.Floor(area.Y*scale))), min(p.image.Bounds().Dy(), int(math.Ceil((area.Y+area.H)*scale)))
	clipLeft, clipRight := p.clip.X*scale, (p.clip.X+p.clip.W)*scale
	clipTop, clipBottom := p.clip.Y*scale, (p.clip.Y+p.clip.H)*scale
	for y := ymin; y < ymax; y++ {
		cy := max(0, min(float64(y+1), clipBottom)-max(float64(y), clipTop))
		for x := xmin; x < xmax; x++ {
			if y >= skip.Min.Y && y < skip.Max.Y && x >= skip.Min.X && x < skip.Max.X {
				x = skip.Max.X - 1
				continue
			}
			px, py := (float64(x)+.5)/scale, (float64(y)+.5)/scale
			// Fractional clip coverage prevents painting beyond an ancestor's edge.
			cx := max(0, min(float64(x+1), clipRight)-max(float64(x), clipLeft))
			coverage := max(0, min(1, .5-distance(px, py)*scale))
			if coverage == 0 || cx == 0 || cy == 0 {
				continue
			}
			c := shade(px, py)
			alpha := float64(c.A) / 255 * coverage * cx * cy
			if alpha == 0 {
				continue
			}
			at := p.image.PixOffset(x, y)
			if alpha == 1 {
				p.image.Pix[at], p.image.Pix[at+1], p.image.Pix[at+2], p.image.Pix[at+3] = c.R, c.G, c.B, 255
				continue
			}
			for i, v := range [3]uint8{c.R, c.G, c.B} {
				p.image.Pix[at+i] = uint8(math.Round(float64(v)*alpha + float64(p.image.Pix[at+i])*(1-alpha)))
			}
			p.image.Pix[at+3] = uint8(math.Round(255*alpha + float64(p.image.Pix[at+3])*(1-alpha)))
		}
	}
}
func roundedDistance(r Rect, radius, x, y float64) float64 {
	radius = max(0, min(radius, min(r.W, r.H)/2))
	dx := math.Abs(x-r.X-r.W/2) - r.W/2 + radius
	dy := math.Abs(y-r.Y-r.H/2) - r.H/2 + radius
	// Away from a corner the distance is axis-aligned: no square root needed.
	if dx <= 0 || dy <= 0 {
		return max(dx, dy) - radius
	}
	return math.Hypot(dx, dy) - radius
}
func (p *Painter) Fill(r Rect, c color.NRGBA, radius float64) {
	if !finite(radius) {
		return
	}
	if c.A == 0 {
		return
	}
	var skip image.Rectangle
	if c.A == 255 {
		inset := max(0, min(radius, min(r.W, r.H)/2))
		skip = p.wholePixels(r.inset(inset))
		if !skip.Empty() {
			fillOpaquePixels(p.image, skip, c)
		}
	}
	p.rasterShadeOutside(r, func(x, y float64) float64 { return roundedDistance(r, radius, x, y) }, func(float64, float64) color.NRGBA { return c }, skip)
}

// Stroke draws a rounded border inside r without filling its interior.
func (p *Painter) Stroke(r Rect, c color.NRGBA, radius, width float64) {
	if !finite(radius) || !finite(width) || width <= 0 {
		return
	}
	if c.A == 0 {
		return
	}
	inner := r.inset(width)
	skip := p.wholePixels(inner.inset(max(0, min(radius-width, min(inner.W, inner.H)/2))))
	p.rasterShadeOutside(r, func(x, y float64) float64 {
		outer := roundedDistance(r, radius, x, y)
		if inner.W == 0 || inner.H == 0 {
			return outer
		}
		return max(outer, -roundedDistance(inner, max(0, radius-width), x, y))
	}, func(float64, float64) color.NRGBA { return c }, skip)
}
func (p *Painter) Line(x1, y1, x2, y2, width float64, c color.NRGBA) {
	if !finite(x1) || !finite(y1) || !finite(x2) || !finite(y2) || !finite(width) || width <= 0 {
		return
	}
	dx, dy := x2-x1, y2-y1
	length := math.Hypot(dx, dy)
	if !finite(length) {
		return
	}
	r := Rect{min(x1, x2) - width/2, min(y1, y2) - width/2, math.Abs(dx) + width, math.Abs(dy) + width}
	p.raster(r, c, func(x, y float64) float64 {
		along := 0.
		if length > 0 {
			along = max(0, min(length, (x-x1)*(dx/length)+(y-y1)*(dy/length)))
		}
		if length == 0 {
			return math.Hypot(x-x1, y-y1) - width/2
		}
		return math.Hypot(x-x1-along*dx/length, y-y1-along*dy/length) - width/2
	})
}
func (p *Painter) Text(x, y float64, text string, c color.NRGBA) {
	scale := p.scale
	if scale == 0 {
		scale = 1
	}
	clip := image.Rect(int(math.Ceil(p.clip.X*scale)), int(math.Ceil(p.clip.Y*scale)), int(math.Floor((p.clip.X+p.clip.W)*scale)), int(math.Floor((p.clip.Y+p.clip.H)*scale))).Intersect(p.image.Bounds())
	if clip.Empty() {
		return
	}
	d := font.Drawer{Dst: p.image.SubImage(clip).(*image.RGBA), Src: image.NewUniform(c), Face: p.face, Dot: fixed.Point26_6{X: fixed.Int26_6(math.Round(x * scale * 64)), Y: fixed.Int26_6(math.Round(y * scale * 64))}}
	d.DrawString(text)
}

func blend(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{uint8(float64(a.R)*(1-t) + float64(b.R)*t), uint8(float64(a.G)*(1-t) + float64(b.G)*t), uint8(float64(a.B)*(1-t) + float64(b.B)*t), uint8(float64(a.A)*(1-t) + float64(b.A)*t)}
}

// Render returns a newly owned image. Call only when content/input/animation
// changes; stable surfaces do not schedule timers or render themselves.
func (s *Surface) Render() (*image.RGBA, error) { return s.RenderScaled(1, nil) }

// RenderScaled rasterizes in physical pixels without changing layout, hit
// testing or accessibility bounds. suppliedFace, if provided, must be sized at
// theme.FontSize*scale and remains caller-owned. Custom-font surfaces require
// this face for scales other than 1; the default font is scaled automatically.
func (s *Surface) RenderScaled(scale float64, suppliedFace font.Face) (*image.RGBA, error) {
	return s.renderScaled(scale, suppliedFace, nil)
}

// RenderScaledInto redraws into a caller-owned, tightly packed RGBA frame with
// origin (0,0) and dimensions ceil(surface size * scale). It does not retain dst.
// Call on the UI goroutine, after previous consumers finish reading the frame.
// On a painting error dst may be partially drawn. Render/RenderScaled continue
// to return independently owned images for consumers that need snapshots.
func (s *Surface) RenderScaledInto(dst *image.RGBA, scale float64, suppliedFace font.Face) error {
	if dst == nil {
		return errors.New("baseui: nil render target")
	}
	_, err := s.renderScaled(scale, suppliedFace, dst)
	return err
}

func (s *Surface) renderScaled(scale float64, suppliedFace font.Face, img *image.RGBA) (*image.RGBA, error) {
	if s.closed {
		return nil, errors.New("baseui: closed surface")
	}
	if !finite(scale) || scale <= 0 || scale > 8 {
		return nil, errors.New("baseui: invalid render scale")
	}
	w, h := math.Ceil(float64(s.width)*scale), math.Ceil(float64(s.height)*scale)
	if w > 16384 || h > 16384 || w*h > 64_000_000 {
		return nil, errors.New("baseui: scaled surface exceeds pixel budget")
	}
	if img != nil && (img.Rect != image.Rect(0, 0, int(w), int(h)) || img.Stride != int(w)*4 || len(img.Pix) < int(w)*int(h)*4) {
		return nil, errors.New("baseui: render target dimensions or storage mismatch")
	}
	face := suppliedFace
	if face == nil {
		face = s.face
		if scale != 1 {
			if !s.ownedFace {
				return nil, errors.New("baseui: scaled rendering requires a scaled custom font face")
			}
			face = s.rasterFace(textFaceKey{size: s.theme.FontSize}, scale)
		}
	}
	if img == nil {
		img = image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	}
	draw.Draw(img, img.Bounds(), image.NewUniform(s.theme.Background), image.Point{}, draw.Src)
	for _, entry := range s.entries {
		if entry.element.kind == ModalKind && !entry.element.passive {
			p := Painter{img, Rect{W: float64(s.width), H: float64(s.height)}, face, s.now, s.theme, scale}
			p.Fill(p.clip, color.NRGBA{0, 0, 0, 100}, 0)
		}
		entryFace := face
		if s.ownedFace && (entry.element.fontSize != 0 || entry.element.fontWeight >= 500 || entry.element.role == "note" || entry.element.monospace) {
			size := entry.element.fontSize
			if size == 0 {
				size = s.theme.FontSize
				if entry.element.kind == TextKind && entry.element.role == "note" {
					size = max(6, size-1)
				}
			}
			key := textFaceKey{size, textWeight(entry.element), entry.element.monospace}
			if scale == 1 {
				entryFace = s.elementFace(entry.element)
			} else {
				entryFace = s.rasterFace(key, scale)
			}
		}
		p := Painter{img, entry.clip, entryFace, s.now, s.theme, scale}
		if err := guarded(func() { s.drawElement(&p, entry) }); err != nil {
			return nil, err
		}
	}
	return img, nil
}

func (s *Surface) drawElement(p *Painter, item entry) {
	e, r, t := item.element, item.bounds, s.theme
	s = s.textSurface(e)
	if e.otpLength > 0 {
		s.drawOTP(p, item)
		return
	}
	fg, bg, border := t.Text, t.Surface, t.Border
	if e.mutedText {
		fg = t.MutedText
	}
	for ancestor := e.id; ancestor != ""; {
		a := s.entries[s.index[ancestor]]
		if a.element.hoverOptions != nil && a.element.hoverOptions.tooltip {
			fg, bg, border = t.Background, t.Text, t.Text
			break
		}
		ancestor = a.parent
	}
	if item.disabled && e.kind != ButtonKind && e.kind != BadgeKind {
		fg = blend(fg, t.Background, .5)
	}
	if e.kind == InputKind || e.kind == TextareaKind || e.inputGroup {
		bg = t.Border
		bg.A = 77
		if t.Style != StyleMaia && t.Background.R > 127 {
			bg = t.Surface
		}
	}
	if !item.disabled && (e.kind == InputKind || e.kind == TextareaKind || e.inputGroup) {
		if s.hover == e.id {
			border = blend(t.Border, t.Text, .2)
		}
		if s.focus == e.id || e.inputGroup && s.within(s.focus, e.id) {
			border = t.Focus
		}
	}
	if e.invalid {
		border = t.Danger
	}
	radius := controlRadius(t, e)
	focusRect, focusRadius := r, radius
	frame := func(fill color.NRGBA) { p.Fill(r, fill, radius); p.Stroke(r, border, radius, 1) }
	label := func(text string, x float64) {
		p.Text(x, r.Y+(r.H-float64(s.face.Metrics().Height.Ceil()))/2+float64(s.face.Metrics().Ascent.Ceil()), text, fg)
	}
	switch e.kind {
	case RowKind:
		if e.role == "tablist" {
			p.Fill(r, t.Muted, controlRadius(t, &Element{kind: ButtonKind}))
		}
		if e.role == "row" && e.selected {
			p.Fill(r, t.Selection, 4)
		}
		if e.role == "row" && e.variant == Secondary && !e.selected {
			p.Fill(r, blend(t.Surface, t.Muted, .35), 4)
		}
		if e.inputGroup {
			frame(bg)
		}
	case CardKind, ModalKind, PopupKind:
		if e.kind == ModalKind || e.kind == PopupKind {
			q := *p
			q.clip = item.paintClip
			q.Shadow(r, radius, 6, 3, color.NRGBA{0, 0, 0, 20})
		}
		if e.variant == Secondary {
			bg = t.Muted
		}
		if e.flat {
			p.Fill(r, bg, radius)
		} else {
			frame(bg)
		}
	case ScrollKind:
		for _, horizontal := range []bool{false, true} {
			if horizontal && !e.horizontal {
				continue
			}
			track, thumb := s.scrollbar(item, horizontal)
			if thumb.W == 0 || thumb.H == 0 {
				continue
			}
			p.Fill(track, t.Muted, 3)
			c := t.MutedText
			if item.disabled {
				c = blend(c, t.Background, .5)
			}
			p.Fill(thumb, c, 3)
		}
	case TextKind:
		if e.role == "alert" {
			fg = t.Danger
		}
		if e.role == "note" {
			fg = t.MutedText
		}
		lines := s.textLines(e, max(0, r.W-e.textIndent))
		top := r.Y
		if e.height > 0 {
			top += max(0, (r.H-s.lineHeight()*float64(len(lines)))/2)
			// Fixed-height text uses symmetric leading, matching button labels.
			top += (s.lineHeight() - float64(s.face.Metrics().Height.Ceil())) / 2
		}
		for i, line := range lines {
			if !e.wrap {
				line = fitLabel(line, max(0, r.W-e.textIndent), s.textWidth)
			}
			p.Text(r.X+e.textIndent, top+float64(s.face.Metrics().Ascent.Ceil())+float64(i)*s.lineHeight(), line, fg)
		}
	case SeparatorKind:
		p.Fill(r, t.Border, 0)
	case ButtonKind, BadgeKind:
		switch e.variant {
		case Primary:
			bg = t.Accent
			fg = t.AccentText
			border = bg
		case Outline:
			bg = t.Surface
			if t.Style == StyleMaia {
				bg = t.Border
				bg.A = 77
			}
		case Secondary:
			bg = t.Muted
			border = bg
		case Link:
			bg = color.NRGBA{}
			border = bg
			fg = t.Accent
		case Ghost:
			bg = color.NRGBA{}
			border = bg
		case Destructive:
			bg = t.Danger
			bg.A = 26
			if t.Background.R < 128 {
				bg.A = 51
			}
			fg = t.Danger
			border = color.NRGBA{}
		}
		if e.selected {
			bg = t.Selection
			fg = t.Text
			border = color.NRGBA{}
		}
		if e.selected && (e.role == "option" || strings.HasPrefix(e.role, "menuitem")) && t.BoldMenus {
			bg, fg = t.Accent, t.AccentText
		}
		if e.selected && strings.HasPrefix(e.role, "menuitem") && !t.BoldMenus {
			bg = t.Hover
			fg = t.Text
			border = color.NRGBA{}
		}
		if e.selected && e.role == "gridcell" {
			bg, fg, border = t.Accent, t.AccentText, color.NRGBA{}
		}
		if e.role == "tab" {
			border = color.NRGBA{}
			fg = t.MutedText
			if e.selected {
				bg = t.Surface
				fg = t.Text
			}
		}
		if s.hover == e.id && !item.disabled && e.variant != Link && !e.selected {
			switch e.variant {
			case Primary:
				bg = t.Accent
				bg.A = 204
			case Outline:
				bg = t.Hover
			case Secondary:
				bg = blend(t.Muted, t.Text, .05)
			case Ghost:
				bg = t.Hover
			case Destructive:
				bg = t.Danger
				bg.A = 51
				if t.Background.R < 128 {
					bg.A = 77
				}
			}
		}

		menuRow := e.role == "option" || strings.HasPrefix(e.role, "menuitem")
		if menuRow && !item.disabled && (e.selected || e.expanded || s.focus == e.id || s.hover == e.id) {
			bg, fg, border = t.Hover, t.Text, color.NRGBA{}
			if t.BoldMenus {
				bg, fg = t.Accent, t.AccentText
			}
		}

		if s.pressed == e.id && !item.disabled && e.variant != Link {
			bg = blend(bg, fg, .16)
		}
		if e.inlineLink {
			fg = t.MutedText
			if s.hover == e.id {
				fg = t.Text
			}
		}
		if item.disabled {
			bg.A = uint8(float64(bg.A) * .5)
			fg.A = uint8(float64(fg.A) * .5)
			border.A = uint8(float64(border.A) * .5)
		}

		if e.invalid {
			border = t.Danger
		}
		frame(bg)
		x := r.X + max(2, (r.W-s.textWidth(e.text))/2)
		if e.inlineLink {
			x = r.X
		}
		if e.alignStart {
			x = r.X + 12 + e.textIndent
			if item.rtl {
				x = r.X + r.W - 12 - e.textIndent - s.textWidth(e.text)
			}
		}
		if e.role == "menuitemcheckbox" && e.checked {
			cx, cy := r.X+16, r.Y+r.H/2
			if item.rtl {
				cx = r.X + r.W - 16
			}
			p.Line(cx-4, cy, cx-1, cy+3, 1.5, fg)
			p.Line(cx-1, cy+3, cx+5, cy-4, 1.5, fg)
		}
		if e.branch {
			cx := r.X + 12 + e.textIndent - 10
			if item.rtl {
				cx = r.X + r.W - 12 - e.textIndent + 10
			}
			paintChevron(p, cx, r.Y+r.H/2, e.expanded, item.rtl, fg)
		}
		if e.submenu {
			cx := r.X + r.W - 12
			if item.rtl {
				cx = r.X + 12
			}
			paintChevron(p, cx, r.Y+r.H/2, false, item.rtl, fg)
		}
		if e.detail != "" {
			p.Text(x, r.Y+18, fitLabel(e.text, max(0, r.X+r.W-x-12), s.textWidth), fg)
			detailColor := t.MutedText
			if item.disabled {
				detailColor = blend(detailColor, t.Background, .4)
			}
			p.Text(x, r.Y+39, fitLabel(e.detail, max(0, r.X+r.W-x-12), s.textWidth), detailColor)
		} else {
			if e.role == "combobox" {
				text := fitLabel(e.text, max(0, r.W-44), s.textWidth)
				x = r.X + 12
				if item.rtl {
					x = r.X + r.W - 12 - s.textWidth(text)
				}
				label(text, x)
				cx := r.X + r.W - 14
				if item.rtl {
					cx = r.X + 14
				}
				paintChevron(p, cx, r.Y+r.H/2, true, false, fg)
			} else {
				if e.inlineLink {
					label(fitLabel(e.text, r.W, s.textWidth), x)
				} else {
					label(e.text, x)
				}
			}
		}
	case CheckboxKind, RadioKind, SwitchKind:
		box := Rect{r.X, r.Y + (r.H-16)/2, 16, 16}
		corner := t.Radius * 3 / 8
		fill, mark := t.Background, t.AccentText
		if e.kind == RadioKind {
			corner = 8
		}
		if e.kind == SwitchKind {
			box = Rect{r.X, r.Y + (r.H-18.4)/2, 32, 18.4}
			corner = 9.2
			fill = t.Border
		}
		if e.checked || e.mixed {
			if e.kind != RadioKind {
				fill = t.Accent
				border = t.Accent
			}
		}
		if e.invalid {
			border = t.Danger
		}
		if item.disabled {
			fill = blend(fill, t.Background, .5)
			border = blend(border, t.Background, .5)
			mark = blend(mark, t.Background, .5)
		}
		p.Fill(box, fill, corner)
		if e.kind != SwitchKind || e.invalid {
			p.Stroke(box, border, corner, 1)
		}
		switch e.kind {
		case SwitchKind:
			x := box.X + 1.2
			if e.checked {
				x += 13.6
			}
			thumb := t.Text
			if e.checked {
				thumb = t.AccentText
			} else if t.Background.R > 127 {
				thumb = t.Background
			}
			if item.disabled {
				thumb = blend(thumb, t.Background, .5)
			}
			p.Fill(Rect{x, box.Y + 1.2, 16, 16}, thumb, 8)
		case RadioKind:
			if e.checked {
				dot := t.Accent
				if item.disabled {
					dot = blend(dot, t.Background, .5)
				}
				p.Fill(box.inset(4), dot, 4)
			}
		default:
			if e.mixed {
				p.Line(box.X+4, box.Y+8, box.X+12, box.Y+8, 1.5, mark)
			} else if e.checked {
				p.Line(box.X+3, box.Y+8, box.X+6.5, box.Y+11, 1.5, mark)
				p.Line(box.X+6.5, box.Y+11, box.X+12.5, box.Y+5, 1.5, mark)
			}
		}
		focusRect, focusRadius = box, corner
		label(e.text, box.X+box.W+8)
	case InputKind, TextareaKind:
		if item.disabled {
			bg.A /= 2
			border.A /= 2
		}
		if !e.groupedInput {
			frame(bg)
		}
		inputClip := r.inset(8)
		if e.groupedInput {
			inputClip = Rect{r.X, r.Y + 4, r.W, max(0, r.H-8)}
		}
		p.clip = intersect(p.clip, inputClip)
		b := e.buffer
		document := b.Snapshot()
		anchor, caret := b.Selection()
		lineHeight := s.lineHeight()
		top := r.Y + 8
		if e.kind == InputKind {
			top = r.Y + (r.H-lineHeight)/2
		}
		inset := inputTextInset(e)
		x := r.X + inset
		if b.text == "" && b.composition == "" && e.placeholder != "" {
			hint := t.MutedText
			if item.disabled {
				hint = blend(hint, t.Background, .5)
			}
			p.Text(x, top+float64(s.face.Metrics().Ascent.Ceil()), e.placeholder, hint)
		}
		scroll := 0.
		if e.kind == InputKind {
			scroll = max(0, s.textWidth(string([]rune(b.text)[:caret]))-(r.W-2*inset))
		}
		// Keep one adjacent line for glyph overhang and composition underlines.
		first := max(0, int(math.Floor((p.clip.Y-top)/lineHeight))-1)
		last := min(document.LineCount(), int(math.Ceil((p.clip.Y+p.clip.H-top)/lineHeight))+1)
		offset := document.RuneOffset(document.LineStart(first))
		for row := first; row < last; row++ {
			line := document.Line(row)
			runes := []rune(line)
			start, end := max(0, min(anchor, caret)-offset), min(len(runes), max(anchor, caret)-offset)
			if end > start {
				left := x - scroll + s.textWidth(string(runes[:start]))
				right := x - scroll + s.textWidth(string(runes[:end]))
				p.Fill(Rect{left, top + float64(row)*lineHeight, right - left, lineHeight}, t.Selection, 0)
			}
			baseline := top + float64(row)*lineHeight + float64(s.face.Metrics().Ascent.Ceil())
			if e.codeRows != nil && row < len(e.codeRows) {
				p.Text(r.X+8-scroll, baseline, strconv.Itoa(row+1), t.MutedText)
				tx := x - scroll
				for _, run := range e.codeRows[row] {
					color := fg
					if run.style > 0 {
						color = t.Chart[(run.style-1)%len(t.Chart)]
					}
					p.Text(tx, baseline, run.text, color)
					tx += s.textWidth(run.text)
				}
			} else {
				p.Text(x-scroll, baseline, line, fg)
			}
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
		trackHeight, thumbSize := 4., 12.
		if t.Style == StyleMaia {
			trackHeight, thumbSize = 12, 16
		}
		trackRadius, thumbRadius := trackHeight/2, thumbSize/2
		if t.Style == StyleLyra {
			trackRadius, thumbRadius = 0, 0
		}
		track := Rect{r.X + 10, r.Y + r.H/2 - trackHeight/2, max(0, r.W-20), trackHeight}
		p.Fill(track, t.Muted, trackRadius)
		fraction := 0.
		if e.max > e.min {
			fraction = (e.value - e.min) / (e.max - e.min)
		}
		fraction = max(0, min(1, fraction))
		accent := t.Accent
		if item.disabled {
			accent = blend(accent, t.Background, .5)
		}
		p.Fill(Rect{track.X, track.Y, track.W * fraction, trackHeight}, accent, trackRadius)
		thumb := Rect{track.X + track.W*fraction - thumbSize/2, r.Y + r.H/2 - thumbSize/2, thumbSize, thumbSize}
		p.Fill(thumb, color.NRGBA{255, 255, 255, 255}, thumbRadius)
		p.Stroke(thumb, accent, thumbRadius, 1)
		focusRect, focusRadius = thumb, thumbRadius
	case ProgressKind:
		p.Fill(r, blend(t.Muted, t.Border, .45), r.H/2)
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
		if e.split != nil {
			p.Fill(r, t.Muted, 0)
			if e.vertical {
				p.Fill(Rect{r.X + r.W/2 - 12, r.Y + 2, 24, 4}, t.MutedText, 2)
			} else {
				p.Fill(Rect{r.X + 2, r.Y + r.H/2 - 12, 4, 24}, t.MutedText, 2)
			}
		}
		if e.paint != nil {
			e.paint(p, r)
		}
	}
	if e.iconSource != nil {
		box := r
		if e.kind == ButtonKind {
			size := min(e.iconOptions.Size, r.W, r.H)
			box = Rect{r.X + (r.W-size)/2, r.Y + (r.H-size)/2, size, size}
		}
		c := fg
		if e.iconOptions.Color.A != 0 {
			c = e.iconOptions.Color
			if item.disabled {
				c = blend(c, t.Background, .5)
			}
		}
		stroke := e.iconOptions.StrokeWidth
		if e.iconOptions.AbsoluteStrokeWidth && stroke > 0 {
			_, h := e.iconSource.Size()
			stroke = stroke * h / max(1, box.H)
		}
		if err := p.Icon(e.iconSource, box, c, stroke); err != nil {
			panic(err)
		}
	}
	if e.kind != CustomKind && e.paint != nil {
		q := *p
		if e.decorationOutside {
			q.clip = item.paintClip
		}
		e.paint(&q, r)
	}
	// Composite menu/list rows express keyboard focus through their active fill.
	// A button ring here duplicates that cue and is clipped by the scroll viewport.
	rowFocus := e.role == "option" || strings.HasPrefix(e.role, "menuitem")
	if !rowFocus && s.focusVisible && (s.focus == e.id && !e.groupedInput || e.inputGroup && s.within(s.focus, e.id)) && !item.disabled {
		p.clip = item.paintClip
		focus := t.Focus
		if e.invalid {
			focus = t.Danger
		}
		// Keyboard focus sits outside the silhouette, clipped by the parent viewport.
		ring := focus
		ring.A = 128
		outer := Rect{focusRect.X - 3, focusRect.Y - 3, focusRect.W + 6, focusRect.H + 6}
		p.Stroke(outer, ring, focusRadius+3, 3)
		p.Stroke(focusRect, focus, focusRadius, 1)
	}
}

// Small control marks are geometry, independent of font glyph coverage.
func paintChevron(p *Painter, x, y float64, down, rtl bool, c color.NRGBA) {
	if down {
		p.Line(x-3, y-1.5, x, y+1.5, 1.5, c)
		p.Line(x, y+1.5, x+3, y-1.5, 1.5, c)
		return
	}
	sign := 1.
	if rtl {
		sign = -1
	}
	p.Line(x-sign*1.5, y-3, x+sign*1.5, y, 1.5, c)
	p.Line(x+sign*1.5, y, x-sign*1.5, y+3, 1.5, c)
}
