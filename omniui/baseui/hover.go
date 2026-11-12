package baseui

import "time"

type hoverOptions struct {
	show, hide time.Duration
	tooltip    bool
}
type hoverRuntime struct {
	open, waiting, want, dismissed bool
	deadline                       time.Duration
}

// Tooltip is a noninteractive descriptive popup. Pointer entry waits 500ms;
// keyboard focus opens immediately. Escape dismisses until the anchor is left.
func Tooltip(id, anchor, text string) *Element {
	e := Popover(id, anchor, true, nil, Text(id+"/text", text)).Width(0).Padding(10).Role("tooltip")
	e.hoverOptions = &hoverOptions{show: 500 * time.Millisecond, hide: 100 * time.Millisecond, tooltip: true}
	e.passive = true
	return e
}

// HoverCard remains open while its anchor or panel contains pointer/focus.
// It does not steal focus or trap Tab. The closing delay bridges their gap.
func HoverCard(id, anchor string, children ...*Element) *Element {
	e := Popover(id, anchor, true, nil, children...)
	e.hoverOptions = &hoverOptions{show: 250 * time.Millisecond, hide: 200 * time.Millisecond}
	e.passive = true
	return e
}

// HoverDelay overrides a Tooltip/HoverCard's pointer delays. Zero is immediate.
func (e *Element) HoverDelay(show, hide time.Duration) *Element {
	if e.hoverOptions == nil {
		panic("baseui: HoverDelay requires Tooltip or HoverCard")
	}
	options := *e.hoverOptions
	options.show = show
	options.hide = hide
	e.hoverOptions = &options
	return e
}
func (s *Surface) within(child, parent string) bool {
	for child != "" {
		if child == parent {
			return true
		}
		i, ok := s.index[child]
		if !ok {
			return false
		}
		child = s.entries[i].parent
	}
	return false
}
func (s *Surface) pointerWithin(id string) bool {
	if !s.pointerKnown {
		return false
	}
	for i := len(s.entries) - 1; i >= 0; i-- {
		item := s.entries[i]
		if item.clip.Contains(s.pointerX, s.pointerY) && s.inScope(item) {
			// Tooltips do not intercept the pointer, including when flipped over
			// their own anchor in a small viewport.
			skip := false
			for p := item.element.id; p != ""; {
				ancestor := s.entries[s.index[p]]
				if ancestor.element.hoverOptions != nil && ancestor.element.hoverOptions.tooltip {
					skip = true
					break
				}
				p = ancestor.parent
			}
			if skip {
				continue
			}
			return s.within(item.element.id, id)
		}
	}
	return false
}
func (s *Surface) syncHover() {
	if s.closed || s.syncingHover {
		return
	}
	s.syncNotifications()
	s.syncingHover = true
	defer func() { s.syncingHover = false }()
	if s.hoverStates == nil {
		s.hoverStates = map[string]*hoverRuntime{}
	}
	seen := map[string]bool{}
	changed := false
	for _, e := range s.hoverCandidates {
		seen[e.id] = true
		state := s.hoverStates[e.id]
		if state == nil {
			state = &hoverRuntime{}
			s.hoverStates[e.id] = state
		}
		anchor, exists := s.index[e.anchor]
		eligible := exists && !s.entries[anchor].disabled && s.inScope(s.entries[anchor]) && s.entries[anchor].clip.W > 0 && s.entries[anchor].clip.H > 0
		focused := eligible && s.within(s.focus, e.anchor)
		desired := eligible && (focused || s.pointerWithin(e.anchor))
		if !e.hoverOptions.tooltip && eligible {
			focused = focused || s.within(s.focus, e.id)
			desired = desired || focused || s.pointerWithin(e.id)
		}
		if !eligible {
			if s.within(s.focus, e.id) {
				s.focus = ""
				if exists && !s.entries[anchor].disabled {
					s.focus = e.anchor
				}
			}
			changed = changed || state.open
			*state = hoverRuntime{}
			continue
		}
		if !desired {
			state.dismissed = false
		}
		if state.dismissed {
			desired = false
		}
		if desired == state.open {
			state.waiting = false
			continue
		}
		if !state.waiting || state.want != desired {
			delay := e.hoverOptions.hide
			if desired {
				delay = e.hoverOptions.show
				if focused {
					delay = 0
				}
			}
			state.waiting = true
			state.want = desired
			state.deadline = s.now + delay
		}
		if focused && desired {
			state.deadline = s.now
		}
		if s.now >= state.deadline {
			state.open = desired
			state.waiting = false
			changed = true
		}
	}
	for id := range s.hoverStates {
		if !seen[id] {
			delete(s.hoverStates, id)
		}
	}
	if changed {
		s.relayout()
	}
}
func (s *Surface) dismissHover() bool {
	dismissed := false
	for _, e := range s.hoverCandidates {
		state := s.hoverStates[e.id]
		if state != nil && (state.open || state.waiting) {
			if s.within(s.focus, e.id) {
				s.focus = e.anchor
			}
			state.open = false
			state.waiting = false
			state.dismissed = true
			dismissed = true
		}
	}
	if dismissed {
		s.relayout()
	}
	return dismissed
}

// NextWakeup returns the delay until the next timed transition or animation
// frame. The host should schedule one callback, Advance its clock, and redraw.
// A false result means no timer is needed. Behavioral delays remain enabled
// under ReduceMotion; hidden/unmounted hover overlays schedule nothing.
func (s *Surface) NextWakeup() (time.Duration, bool) {
	if s.closed {
		return 0, false
	}
	delay := time.Duration(0)
	scheduled := false
	if s.NeedsAnimation() {
		delay = time.Second / 30
		scheduled = true
	}
	for _, state := range s.hoverStates {
		if state.waiting {
			next := max(time.Duration(0), state.deadline-s.now)
			if !scheduled || next < delay {
				delay = next
				scheduled = true
			}
		}
	}
	for m := range s.notificationManagers {
		if next, ok := m.wakeup(); ok && (!scheduled || next < delay) {
			delay = next
			scheduled = true
		}
	}
	return delay, scheduled
}
