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

// FontWeight selects a real bundled Inter weight: regular, medium, semibold or bold.
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
	size   float64
	weight int
	mono   bool
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
	weight := textWeight(e)
	if size == s.theme.FontSize && weight == 0 && !e.monospace {
		return s.face
	}
	key := textFaceKey{size, weight, e.monospace}
	if s.textFaces == nil {
		s.textFaces = map[textFaceKey]font.Face{}
	}
	if face := s.textFaces[key]; face != nil {
		return face
	}
	face := weightedFace(weight, e.monospace, size)
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

// Keep only the current backing scale. DPI changes release the previous faces,
// avoiding per-frame font construction without accumulating arbitrary scales.
func (s *Surface) clearRasterFaces() {
	for _, face := range s.rasterFaces {
		_ = face.Close()
	}
	s.rasterFaces = nil
	s.rasterScale = 0
}
func (s *Surface) rasterFace(key textFaceKey, scale float64) font.Face {
	if s.rasterScale != scale {
		s.clearRasterFaces()
		s.rasterScale = scale
	}
	if s.rasterFaces == nil {
		s.rasterFaces = make(map[textFaceKey]font.Face)
	}
	if face := s.rasterFaces[key]; face != nil {
		return face
	}
	face := weightedFace(key.weight, key.mono, key.size*scale)
	s.rasterFaces[key] = face
	return face
}
