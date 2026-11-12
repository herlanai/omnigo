package baseui

import (
	"errors"
	"math"
)

// ScrollState belongs to the UI goroutine. Offsets are in logical pixels.
// Layout clamps offsets when content or viewport dimensions change.
type ScrollState struct {
	X, Y       float64
	maxX, maxY float64
}

func (s *ScrollState) Limits() (float64, float64) { return s.maxX, s.maxY }

// ScrollArea clips content and provides wheel, keyboard and draggable scrollbars.
// Use Width/Height to choose the viewport. Horizontal scrolling is opt-in.
func ScrollArea(id string, state *ScrollState, children ...*Element) *Element {
	e := Column(id, children...)
	e.kind = ScrollKind
	e.scroll = state
	e.height = 240
	e.role = "region"
	e.label = "Scrollable content"
	return e
}
func (e *Element) HorizontalScroll(enabled bool) *Element { e.horizontal = enabled; return e }

type WheelEvent struct{ X, Y, DeltaX, DeltaY float64 }

func (s *Surface) scrollViewport(item entry) Rect {
	r := item.bounds.inset(item.element.padding)
	r.W = max(0, r.W-10)
	if item.element.horizontal {
		r.H = max(0, r.H-10)
	}
	return r
}
func (s *Surface) layoutScroll(e *Element, r, clip Rect, disabled bool, modal string) {
	item := s.entries[s.index[e.id]]
	viewport := s.scrollViewport(item)
	var contentW, contentH float64
	count := 0
	for _, c := range e.children {
		if c.hidden || overlay(c) {
			continue
		}
		w, h := s.measure(c, viewport.W)
		contentW = max(contentW, w)
		contentH += h
		count++
	}
	contentH += float64(max(0, count-1)) * e.gap
	state := e.scroll
	atTail := state.Y >= state.maxY-1
	state.maxY = max(0, contentH-viewport.H)
	if e.followTail && atTail {
		state.Y = state.maxY
	}
	state.maxX = 0
	if e.horizontal {
		state.maxX = max(0, contentW-viewport.W)
	}
	state.X = max(0, min(state.maxX, state.X))
	state.Y = max(0, min(state.maxY, state.Y))
	y := viewport.Y - state.Y
	for _, c := range e.children {
		if c.hidden {
			continue
		}
		if overlay(c) {
			s.deferOverlay(c, disabled, modal, e.id)
			continue
		}
		w, h := s.measure(c, viewport.W)
		if c.width == 0 {
			w = viewport.W
			if e.horizontal {
				w = max(w, contentW)
			}
		}
		s.layout(c, Rect{viewport.X - state.X, y, w, h}, intersect(viewport, clip), disabled, modal, e.id)
		y += h + e.gap
	}
}

// ScrollTo updates one mounted viewport and immediately refreshes geometry.
func (s *Surface) ScrollTo(id string, x, y float64) error {
	defer s.syncHover()
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if !finite(x) || !finite(y) {
		return errors.New("baseui: invalid scroll offset")
	}
	i, ok := s.index[id]
	if !ok || s.entries[i].element.scroll == nil {
		return errors.New("baseui: unknown scroll area")
	}
	item := s.entries[i]
	if item.disabled || item.element.readOnly {
		return nil
	}
	state := item.element.scroll
	state.X = max(0, min(state.maxX, x))
	state.Y = max(0, min(state.maxY, y))
	s.relayout()
	return nil
}
func (s *Surface) Wheel(ev WheelEvent) error {
	if s.closed {
		return errors.New("baseui: closed surface")
	}
	if !finite(ev.X) || !finite(ev.Y) || !finite(ev.DeltaX) || !finite(ev.DeltaY) {
		return errors.New("baseui: invalid wheel event")
	}
	// Deepest visible viewport consumes scrolling. At an edge, bubble to its
	// ancestor only; never scroll a geometrically overlapping sibling.
	target := ""
	for i := len(s.entries) - 1; i >= 0; i-- {
		item := s.entries[i]
		if item.clip.Contains(ev.X, ev.Y) && s.inScope(item) {
			target = item.element.id
			break
		}
	}
	for target != "" {
		item := s.entries[s.index[target]]
		e := item.element
		if e.scroll != nil {
			if item.disabled || e.readOnly {
				return nil
			}
			state := e.scroll
			x, y := max(0, min(state.maxX, state.X+ev.DeltaX)), max(0, min(state.maxY, state.Y+ev.DeltaY))
			if x != state.X || y != state.Y {
				return s.ScrollTo(e.id, x, y)
			}
		}
		target = item.parent
	}
	return nil
}
func (s *Surface) scrollbar(item entry, horizontal bool) (track, thumb Rect) {
	viewport := s.scrollViewport(item)
	state := item.element.scroll
	if horizontal {
		track = Rect{viewport.X, viewport.Y + viewport.H + 2, viewport.W, 6}
		if state.maxX <= 0 {
			return track, Rect{}
		}
		length := min(track.W, max(20, track.W*viewport.W/(viewport.W+state.maxX)))
		thumb = Rect{track.X + (track.W-length)*state.X/state.maxX, track.Y, length, track.H}
	} else {
		track = Rect{viewport.X + viewport.W + 2, viewport.Y, 6, viewport.H}
		if state.maxY <= 0 {
			return track, Rect{}
		}
		length := min(track.H, max(20, track.H*viewport.H/(viewport.H+state.maxY)))
		thumb = Rect{track.X, track.Y + (track.H-length)*state.Y/state.maxY, track.W, length}
	}
	return
}
func (s *Surface) startScrollDrag(item entry, x, y float64) bool {
	if item.disabled || item.element.readOnly {
		return false
	}
	for _, horizontal := range []bool{false, true} {
		if horizontal && !item.element.horizontal {
			continue
		}
		track, thumb := s.scrollbar(item, horizontal)
		if thumb.W == 0 || thumb.H == 0 || !track.inset(-2).Contains(x, y) {
			continue
		}
		s.scrollDrag = item.element.id
		s.scrollHorizontal = horizontal
		s.scrollGrab = y - thumb.Y
		if horizontal {
			s.scrollGrab = x - thumb.X
		}
		if !thumb.Contains(x, y) {
			s.scrollGrab = thumb.H / 2
			if horizontal {
				s.scrollGrab = thumb.W / 2
			}
		}
		s.dragScroll(x, y)
		return true
	}
	return false
}
func (s *Surface) dragScroll(x, y float64) {
	i, ok := s.index[s.scrollDrag]
	if !ok {
		s.scrollDrag = ""
		return
	}
	item := s.entries[i]
	if item.disabled || item.element.readOnly {
		s.scrollDrag = ""
		return
	}
	track, thumb := s.scrollbar(item, s.scrollHorizontal)
	state := item.element.scroll
	if s.scrollHorizontal {
		state.X = max(0, min(state.maxX, (x-track.X-s.scrollGrab)/math.Max(1, track.W-thumb.W)*state.maxX))
	} else {
		state.Y = max(0, min(state.maxY, (y-track.Y-s.scrollGrab)/math.Max(1, track.H-thumb.H)*state.maxY))
	}
	s.relayout()
}
func (s *Surface) scrollKey(item entry, ev KeyEvent) bool {
	if item.element.scroll == nil || item.element.readOnly {
		return false
	}
	state := item.element.scroll
	x, y := state.X, state.Y
	page := s.scrollViewport(item).H * .9
	switch ev.Key {
	case "Down":
		y += 32
	case "Up":
		y -= 32
	case "Right":
		x += 32
	case "Left":
		x -= 32
	case "PageDown":
		y += page
	case "PageUp":
		y -= page
	case "Home":
		y = 0
	case "End":
		y = state.maxY
	default:
		return false
	}
	_ = s.ScrollTo(item.element.id, x, y)
	return true
}

// focusEligible includes offscreen descendants of a visible scroll viewport.
func (s *Surface) focusEligible(id string) bool {
	i, ok := s.index[id]
	if !ok {
		return false
	}
	item := s.entries[i]
	if item.disabled || !s.inScope(item) || item.bounds.W <= 0 || item.bounds.H <= 0 {
		return false
	}
	if item.clip.W > 0 && item.clip.H > 0 {
		return true
	}
	for parent := item.parent; parent != ""; {
		ancestor := s.entries[s.index[parent]]
		if ancestor.element.scroll != nil && ancestor.clip.W > 0 && ancestor.clip.H > 0 {
			return true
		}
		parent = ancestor.parent
	}
	return false
}
func (s *Surface) reveal(id string) {
	i, ok := s.index[id]
	if !ok {
		return
	}
	for parent := s.entries[i].parent; parent != ""; {
		ancestor := s.entries[s.index[parent]]
		if state := ancestor.element.scroll; state != nil && !ancestor.disabled {
			target := s.entries[s.index[id]].bounds
			v := s.scrollViewport(ancestor)
			if target.Y < v.Y {
				state.Y -= v.Y - target.Y
			} else if target.Y+target.H > v.Y+v.H {
				state.Y += target.Y + min(target.H, v.H) - v.Y - v.H
			}
			if ancestor.element.horizontal {
				if target.X < v.X {
					state.X -= v.X - target.X
				} else if target.X+target.W > v.X+v.W {
					state.X += target.X + min(target.W, v.W) - v.X - v.W
				}
			}
			s.relayout()
		}
		parent = ancestor.parent
	}
}
