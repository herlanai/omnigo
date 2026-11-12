package baseui

import "math"

type Side uint8

const (
	Right Side = iota
	Left
	Top
	Bottom
)

// Sheet uses the same focus isolation and dismissal behavior as Dialog.
func Sheet(id, title string, open bool, side Side, close func(), children ...*Element) *Element {
	e := Dialog(id, title, open, close, children...)
	e.sheet = true
	e.side = side
	e.width = 360
	return e
}

// AspectRatio derives height from available width unless Height is explicit.
func AspectRatio(id string, ratio float64, child *Element) *Element {
	e := Column(id, child)
	e.kind = AspectKind
	e.ratio = ratio
	return e
}

type LayoutDirection uint8

const (
	InheritDirection LayoutDirection = iota
	LeftToRight
	RightToLeft
)

// Direction changes row order and navigation direction. It does not implement
// Unicode bidi shaping or mirrored paragraph layout.
func Direction(id string, direction LayoutDirection, children ...*Element) *Element {
	e := Column(id, children...)
	e.direction = direction
	return e
}

type SplitState struct {
	// Fraction is the first pane's share of usable space, in [0,1].
	Fraction            float64
	MinFirst, MinSecond float64
}

// Resizable has an 8 DIP draggable divider, keyboard arrows and Home/End.
// Cancellation restores the fraction from the start of the drag.
func Resizable(id string, state *SplitState, vertical bool, first, second *Element) *Element {
	grip := element(id+"/divider", CustomKind)
	grip.role = "separator"
	grip.label = "Resize panes"
	grip.split = state
	grip.vertical = vertical
	grip.onKey = func(KeyEvent) bool { return false }
	e := Column(id, first, grip, second)
	e.kind = SplitKind
	e.split = state
	e.vertical = vertical
	e.width = 480
	e.height = 240
	return e
}
func (s *Surface) splitRange(item entry) (usable, lo, hi float64) {
	extent := item.bounds.W
	if item.element.vertical {
		extent = item.bounds.H
	}
	usable = max(0, extent-8)
	state := item.element.split
	lo = min(usable, state.MinFirst)
	hi = max(lo, usable-state.MinSecond)
	return
}
func (s *Surface) layoutSplit(e *Element, r, clip Rect, disabled bool, modal string) {
	usable, lo, hi := s.splitRange(s.entries[s.index[e.id]])
	first := max(lo, min(hi, usable*e.split.Fraction))
	for i, c := range e.children {
		child := r
		start, length := 0., first
		if i == 1 {
			start = first
			length = min(8, max(0, func() float64 {
				if e.vertical {
					return r.H
				}
				return r.W
			}()-first))
		}
		if i == 2 {
			start = first + 8
			length = max(0, usable-first)
		}
		if e.vertical {
			child.Y += start
			child.H = length
		} else {
			child.X += start
			child.W = length
		}
		s.layout(c, child, intersect(r, clip), disabled, modal, e.id)
	}
}
func (s *Surface) dragSplit(x, y float64) {
	i, ok := s.index[s.splitDrag]
	if !ok {
		s.splitDrag = ""
		return
	}
	divider := s.entries[i]
	if divider.disabled || divider.element.readOnly {
		s.splitDrag = ""
		return
	}
	panel := s.entries[s.index[divider.parent]]
	usable, lo, hi := s.splitRange(panel)
	local := x - panel.bounds.X - 4
	if panel.element.vertical {
		local = y - panel.bounds.Y - 4
	}
	if usable > 0 {
		panel.element.split.Fraction = max(lo, min(hi, local)) / usable
	}
	s.relayout()
}
func (s *Surface) splitKey(item entry, ev KeyEvent) bool {
	if item.element.split == nil || item.element.kind != CustomKind || item.element.readOnly {
		return false
	}
	panel := s.entries[s.index[item.parent]]
	usable, lo, hi := s.splitRange(panel)
	if usable == 0 {
		return true
	}
	n := item.element.split.Fraction
	switch ev.Key {
	case "Left":
		if item.element.vertical {
			return false
		}
		n -= .05
	case "Right":
		if item.element.vertical {
			return false
		}
		n += .05
	case "Up":
		if !item.element.vertical {
			return false
		}
		n -= .05
	case "Down":
		if !item.element.vertical {
			return false
		}
		n += .05
	case "Home":
		n = lo / usable
	case "End":
		n = hi / usable
	default:
		return false
	}
	item.element.split.Fraction = max(lo/usable, min(hi/usable, n))
	s.relayout()
	return true
}

// Skeleton is decorative and schedules animation only while visible. The
// shared explicit clock and ReduceMotion policy control its pulse.
func Skeleton(id string, width, height float64) *Element {
	return Canvas(id, "", func(p *Painter, r Rect) {
		t := p.Theme()
		amount := 0.
		if !t.ReduceMotion {
			amount = .08 * (1 + math.Sin(float64(p.Now())/1e9*math.Pi))
		}
		p.Fill(r, blend(t.Muted, t.Text, amount), t.Radius*.75)
	}).Width(width).Height(height).Animate(true).Role("")
}
