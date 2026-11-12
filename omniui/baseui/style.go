package baseui

import (
	"errors"
	"image/color"
	"strings"
)

// VisualStyle selects shared shape tokens, independently of light/dark colors.
// These are native approximations of the corresponding shadcn style families.
type VisualStyle uint8

const (
	StyleMaia VisualStyle = iota
	StyleNova
	StyleLyra
)

func (s VisualStyle) String() string {
	switch s {
	case StyleMaia:
		return "Maia"
	case StyleNova:
		return "Nova"
	case StyleLyra:
		return "Lyra"
	}
	return "Unknown"
}

type AccentPalette uint8

const (
	AccentNeutral AccentPalette = iota
	AccentBlue
	AccentViolet
)

// ThemePreset separates geometry, palette and selected menu emphasis.
func ThemePreset(dark bool, style VisualStyle, accent AccentPalette, boldMenus bool) Theme {
	if style > StyleLyra || accent > AccentViolet {
		panic("baseui: invalid theme preset")
	}
	t := NewTheme(dark)
	t.Style = style
	t.BoldMenus = boldMenus
	switch style {
	case StyleNova:
		t.Radius = 12
		t.Height = 32
	case StyleLyra:
		t.Radius = 0
		t.Height = 32
		t.FontSize = 12
	}
	switch accent {
	case AccentBlue:
		t.Accent = color.NRGBA{37, 99, 235, 255}
		t.AccentText = color.NRGBA{255, 255, 255, 255}
		t.Focus = t.Accent
	case AccentViolet:
		t.Accent = color.NRGBA{109, 40, 217, 255}
		t.AccentText = color.NRGBA{255, 255, 255, 255}
		t.Focus = t.Accent
	}
	if dark && accent != AccentNeutral {
		if accent == AccentBlue {
			t.Accent = color.NRGBA{96, 165, 250, 255}
		} else {
			t.Accent = color.NRGBA{167, 139, 250, 255}
		}
		t.AccentText = color.NRGBA{17, 17, 17, 255}
		t.Focus = t.Accent
	}
	return t
}

// SetTheme updates a live surface on its UI goroutine. Application buffers,
// selection, focus and scroll survive; font caches and pressed state do not.
func (s *Surface) SetTheme(t Theme) error {
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	var next *Surface
	var err error
	if s.ownedFace {
		next, err = NewSurface(s.width, s.height, t)
	} else {
		next, err = NewSurfaceWithFont(s.width, s.height, t, s.face)
	}
	if err != nil {
		return err
	}
	if s.pressed != "" {
		_ = s.Pointer(PointerEvent{Kind: "cancel"})
	}
	for _, face := range s.textFaces {
		_ = face.Close()
	}
	s.textFaces = nil
	s.clearRasterFaces()
	if s.ownedFace {
		_ = s.face.Close()
	}
	s.theme = t
	s.face = next.face
	var update func(*Element)
	update = func(e *Element) {
		if e == nil {
			return
		}
		e.applyStyle(t)
		for _, child := range e.children {
			update(child)
		}
	}
	update(s.root)
	s.relayout()
	return nil
}
func controlRadius(t Theme, e *Element) float64 {
	radius := t.Radius
	if e.kind == ButtonKind || e.kind == BadgeKind || e.kind == InputKind || e.inputGroup {
		switch t.Style {
		case StyleMaia:
			radius = t.Radius * 2
		case StyleNova:
			radius = max(0, t.Radius-4)
		case StyleLyra:
			radius = 0
		}
	}
	if e.kind == BadgeKind && t.Style != StyleLyra {
		radius = 32
	}
	if e.kind == TextareaKind || e.alignStart {
		radius = t.Radius * .75
	}
	if t.Style == StyleNova && (e.role == "option" || strings.HasPrefix(e.role, "menuitem")) {
		radius = max(0, t.Radius-6)
	}
	if e.role == "tab" && t.Style == StyleMaia {
		radius = 12
	}
	if e.sheet {
		radius = 0
	}
	if e.radius != nil {
		radius = *e.radius
	}
	return radius
}

func (e *Element) applyStyle(t Theme) {
	if e.kind == CardKind {
		spacing := 24.
		if t.Style != StyleMaia {
			spacing = 16
		}
		if !e.paddingExplicit {
			e.padding = spacing
		}
		if !e.gapExplicit {
			e.gap = spacing
		}
	}
}
