package baseui

// Placement chooses the preferred anchor edge; panels flip and clamp at the
// viewport boundary. All positions are logical pixels.
type Placement uint8

const (
	Below Placement = iota
	Above
	Beside
)

type deferredOverlay struct {
	element       *Element
	disabled      bool
	modal, parent string
}
type focusLayer struct{ id, returnTo string }

func overlay(e *Element) bool { return e.kind == ModalKind || e.kind == PopupKind }

// Popover is a Go-painted, anchored focus scope. Outside presses and Escape
// dismiss it without activating controls behind it. Anchor IDs must exist.
func Popover(id, anchor string, open bool, close func(), children ...*Element) *Element {
	e := Card(id, children...).Width(288).Padding(12).Gap(8).Hidden(!open)
	e.kind = PopupKind
	e.anchor = anchor
	e.activate = close
	e.role = "dialog"
	e.dismissOutside = true
	return e
}
func (e *Element) Placement(p Placement) *Element { e.placement = p; return e }
func (e *Element) AutoFocus(id string) *Element   { e.autoFocus = id; return e }
func (s *Surface) deferOverlay(e *Element, disabled bool, modal, parent string) {
	if !e.hidden && !disabled && !e.disabled {
		s.overlays = append(s.overlays, deferredOverlay{e, disabled, modal, parent})
	}
}
func (s *Surface) relayout() {
	s.entries = s.entries[:0]
	clear(s.index)
	s.overlays = nil
	s.hoverCandidates = nil
	s.layers = nil
	s.modal = ""
	if s.root == nil {
		return
	}
	bounds := Rect{W: float64(s.width), H: float64(s.height)}
	if s.root.width > 0 {
		bounds.W = min(bounds.W, s.root.width)
	}
	if s.root.height > 0 {
		bounds.H = min(bounds.H, s.root.height)
	} else if s.root.kind == AspectKind {
		bounds.H = min(bounds.H, bounds.W/s.root.ratio)
	}
	s.layout(s.root, bounds, bounds, false, "", "")
	pending := s.overlays
	hoverSeen := map[string]bool{}
	for len(pending) > 0 {
		s.overlays = nil
		remaining := []deferredOverlay{}
		progressed := false
		for _, o := range pending {
			e := o.element
			if e.hoverOptions != nil {
				if !hoverSeen[e.id] {
					s.hoverCandidates = append(s.hoverCandidates, e)
					hoverSeen[e.id] = true
				}
				state := s.hoverStates[e.id]
				if state == nil || !state.open {
					continue
				}
			}
			if e.kind == PopupKind {
				if _, ready := s.index[e.anchor]; !ready {
					remaining = append(remaining, o)
					continue
				}
			}
			s.placing = e.id
			s.layout(e, Rect{}, Rect{W: float64(s.width), H: float64(s.height)}, o.disabled, o.modal, o.parent)
			progressed = true
		}
		pending = append(remaining, s.overlays...)
		if !progressed {
			break
		}
	}
	s.placing = ""
}
func (s *Surface) placeOverlay(e *Element) (Rect, bool) {
	screen := Rect{W: float64(s.width), H: float64(s.height)}.inset(8)
	w, h := s.measure(e, screen.W)
	w = min(w, screen.W)
	h = min(h, screen.H)
	if e.notifications != nil {
		return Rect{screen.X + screen.W - w, screen.Y + screen.H - h, w, h}, true
	}
	if e.kind == ModalKind && e.sheet {
		full := Rect{W: float64(s.width), H: float64(s.height)}
		switch e.side {
		case Left:
			full.W = min(w, full.W*.8)
		case Right:
			full.W = min(w, full.W*.8)
			full.X = float64(s.width) - full.W
		case Top:
			full.H = min(h, full.H*.8)
		case Bottom:
			full.H = min(h, full.H*.8)
			full.Y = float64(s.height) - full.H
		}
		if e.drawer != nil {
			full.Y += e.drawer.offset
		}
		return full, true
	}
	if e.kind == ModalKind {
		return Rect{screen.X + (screen.W-w)/2, screen.Y + (screen.H-h)/2, w, h}, true
	}
	i, ok := s.index[e.anchor]
	if !ok {
		return Rect{}, false
	}
	a := s.entries[i]
	if a.clip.W <= 0 || a.clip.H <= 0 || a.disabled {
		return Rect{}, false
	}
	x, y := a.bounds.X, a.bounds.Y+a.bounds.H+4
	if e.pointAnchor {
		x = a.bounds.X + e.pointX
		y = a.bounds.Y + e.pointY
		return Rect{max(screen.X, min(x, screen.X+screen.W-w)), max(screen.Y, min(y, screen.Y+screen.H-h)), w, h}, true
	}
	switch e.placement {
	case Above:
		y = a.bounds.Y - h - 4
		if y < screen.Y {
			y = a.bounds.Y + a.bounds.H + 4
		}
	case Beside:
		x = a.bounds.X + a.bounds.W + 4
		y = a.bounds.Y
		if x+w > screen.X+screen.W {
			x = a.bounds.X - w - 4
		}
	default:
		if y+h > screen.Y+screen.H && a.bounds.Y-h-4 >= screen.Y {
			y = a.bounds.Y - h - 4
		}
	}
	return Rect{max(screen.X, min(x, screen.X+screen.W-w)), max(screen.Y, min(y, screen.Y+screen.H-h)), w, h}, true
}
func (s *Surface) updateLayerFocus() {
	shared := 0
	for shared < len(s.focusLayers) && shared < len(s.layers) && s.focusLayers[shared].id == s.layers[shared] {
		shared++
	}
	for len(s.focusLayers) > shared {
		last := s.focusLayers[len(s.focusLayers)-1]
		s.focus = last.returnTo
		s.focusLayers = s.focusLayers[:len(s.focusLayers)-1]
	}
	for _, id := range s.layers[shared:] {
		s.focusLayers = append(s.focusLayers, focusLayer{id, s.focus})
		s.focus = ""
		e := s.entries[s.index[id]].element
		// Only the top layer is eligible for focus.
		if id == s.modal {
			if !s.Focus(e.autoFocus) {
				s.focusNext(false)
			}
		}
	}
}

func (s *Surface) inScope(item entry) bool {
	if s.modal == "" || item.modal == s.modal {
		return true
	}
	if i, ok := s.index[s.modal]; ok {
		panel := s.entries[i].element
		return panel.allowAnchorFocus && panel.anchor == item.element.id
	}
	return false
}
