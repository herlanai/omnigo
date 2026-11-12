package baseui

import (
	"math"
	"strings"
)

// InputOTP presents one logical text input as digit cells. The shared buffer
// retains selection/undo/IME semantics; only ASCII digits are accepted on input.
// Initial content must contain at most length digits. Length is 1..32.
func InputOTP(id, label string, buffer *TextBuffer, length int) *Element {
	if length < 1 || length > 32 {
		panic("baseui: OTP length must be 1..32")
	}
	e := Input(id, label, buffer)
	e.otpLength = length
	e.width = float64(length) * 40
	e.height = 40
	return e
}
func digits(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func (s *Surface) replaceInput(e *Element, text string) {
	if e.kind == InputKind {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\n", ""), "\r", "")
	}
	limit := e.buffer.Limit
	if e.otpLength > 0 {
		filtered := digits(text)
		if text != "" && filtered == "" {
			return
		}
		text = filtered
		if limit <= 0 || limit > e.otpLength {
			limit = e.otpLength
		}
	}
	before := e.buffer.String()
	e.buffer.replace(text, limit)
	if before != e.buffer.String() && e.onChange != nil {
		e.onChange(e.buffer.String())
	}
}
func (s *Surface) drawOTP(p *Painter, item entry) {
	e, r, t := item.element, item.bounds, s.theme
	b := e.buffer
	anchor, caret := b.Selection()
	cellW := r.W / float64(e.otpLength)
	fg, border := t.Text, t.Border
	if item.disabled {
		fg = blend(fg, t.Background, .5)
	}
	if e.invalid {
		border = t.Danger
	}
	baseline := r.Y + (r.H-float64(s.face.Metrics().Height.Ceil()))/2 + float64(s.face.Metrics().Ascent.Ceil())
	for i := range e.otpLength {
		cell := Rect{r.X + float64(i)*cellW, r.Y, max(0, cellW-4), r.H}
		p.Fill(cell, t.Surface, 8)
		p.Stroke(cell, border, 8, 1)
		if i >= min(anchor, caret) && i < max(anchor, caret) {
			p.Fill(cell.inset(2), t.Selection, 6)
		}
		if i < len(b.text) {
			text := string(b.text[i])
			p.Text(cell.X+(cell.W-s.textWidth(text))/2, baseline, text, fg)
		}
		if s.focus == e.id && !item.disabled && i == min(caret, e.otpLength-1) {
			focus := t.Focus
			if e.invalid {
				focus = t.Danger
			}
			p.Stroke(cell, focus, 8, 2)
			if b.composition != "" {
				old := p.clip
				p.clip = intersect(p.clip, cell.inset(3))
				p.Text(cell.X+4, baseline, b.composition, fg)
				p.clip = old
				p.Line(cell.X+4, baseline+2, cell.X+cell.W-4, baseline+2, 1, focus)
			} else if !e.readOnly && caret < len(b.text)+1 && caret < e.otpLength {
				x := cell.X + cell.W/2
				if caret < len(b.text) {
					x = cell.X + 4
				}
				p.Fill(Rect{x, r.Y + 10, 1, r.H - 20}, focus, 0)
			}
		}
	}
}
func (s *Surface) selectOTP(item entry, x float64, extend bool) {
	b := item.element.buffer
	cellW := item.bounds.W / float64(item.element.otpLength)
	caret := int(math.Floor((x - item.bounds.X) / max(1, cellW)))
	caret = max(0, min(len(b.text), caret))
	anchor := caret
	if extend {
		anchor = b.anchor
	}
	b.Select(anchor, caret)
}
