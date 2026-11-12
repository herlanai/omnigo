package baseui

import (
	"golang.org/x/image/font"
	"math"
)

type ControlSize uint8

const (
	ControlNormal ControlSize = iota
	ControlXS
	ControlSmall
	ControlLarge
)

// FontSize applies to this element. Half-DIP quantization bounds the surface's
// font cache. Custom caller-owned faces retain their supplied metrics.
func (e *Element) FontSize(size float64) *Element {
	if !finite(size) || size < 6 || size > 72 {
		panic("baseui: font size outside 6..72")
	}
	e.fontSize = math.Round(size*2) / 2
	return e
}

// FontWeight selects regular (400/500) or bold (600/700) in the bundled Go fonts.
func (e *Element) FontWeight(weight int) *Element {
	if weight != 400 && weight != 500 && weight != 600 && weight != 700 {
		panic("baseui: unsupported font weight")
	}
	e.fontWeight = weight
	return e
}
func (e *Element) Muted(value bool) *Element { e.mutedText = value; return e }
func (e *Element) CornerRadius(radius float64) *Element {
	if !finite(radius) || radius < 0 || radius > 128 {
		panic("baseui: invalid radius")
	}
	e.radius = &radius
	return e
}
func (e *Element) Size(size ControlSize) *Element {
	switch size {
	case ControlNormal:
		e.height = 36
		e.fontSize = 14
		e.controlPadding = 12
	case ControlXS:
		e.height = 24
		e.fontSize = 12
		e.controlPadding = 10
	case ControlSmall:
		e.height = 32
		e.fontSize = 14
		e.controlPadding = 12
	case ControlLarge:
		e.height = 40
		e.fontSize = 14
		e.controlPadding = 16
	default:
		panic("baseui: invalid control size")
	}
	if e.iconSource != nil && e.kind == ButtonKind {
		e.width = e.height
		e.iconOptions.Size = min(20, e.height-10)
	}
	return e
}
func Heading(id, text string, level int) *Element {
	if level < 1 || level > 6 {
		panic("baseui: invalid heading level")
	}
	sizes := []float64{28, 24, 20, 18, 16, 14}
	return Text(id, text).Role("heading").FontSize(sizes[level-1]).FontWeight(600).Wrap(true)
}
func Label(id, text string) *Element { return Text(id, text).FontSize(14).FontWeight(500) }

type textFaceKey struct {
	size float64
	bold bool
	mono bool
}

func (s *Surface) elementFace(e *Element) font.Face {
	if !s.ownedFace {
		return s.face
	}
	size := e.fontSize
	if size == 0 {
		size = s.theme.FontSize
		if e.kind == TextKind && e.role == "note" {
			size = max(6, size-1)
		}
	}
	bold := e.fontWeight >= 600
	if size == s.theme.FontSize && !bold && !e.monospace {
		return s.face
	}
	key := textFaceKey{size, bold, e.monospace}
	if s.textFaces == nil {
		s.textFaces = map[textFaceKey]font.Face{}
	}
	if face := s.textFaces[key]; face != nil {
		return face
	}
	style := 0
	if bold {
		style = 1
	}
	if e.monospace {
		style += 4
	}
	face := richFace(style, size)
	s.textFaces[key] = face
	return face
}
func (s *Surface) textSurface(e *Element) *Surface {
	face := s.elementFace(e)
	if face == s.face {
		return s
	}
	local := *s
	local.face = face
	return &local
}
