package baseui

import (
	"fmt"
	"time"
)

type ToastOptions struct {
	ID, Title, Description, ActionLabel string
	Duration                            time.Duration
	Action                              func()
	Error                               bool
}
type toastRecord struct {
	options   ToastOptions
	remaining time.Duration
	active    bool
}

// ToastManager is confined to a UI goroutine and has no goroutine/timer of its
// own. Zero Duration is persistent. The Surface supplies a monotonic clock.
type ToastManager struct {
	records                          []*toastRecord
	serial                           uint64
	last                             time.Duration
	initialized, paused, interacting bool
}

func (m *ToastManager) Add(o ToastOptions) string {
	if o.Duration < 0 {
		panic("baseui: negative toast duration")
	}
	if o.ID == "" {
		for {
			m.serial++
			o.ID = fmt.Sprintf("toast-%d", m.serial)
			exists := false
			for _, r := range m.records {
				if r.options.ID == o.ID {
					exists = true
					break
				}
			}
			if !exists {
				break
			}
		}
	}
	for _, r := range m.records {
		if r.options.ID == o.ID {
			r.options = o
			r.remaining = o.Duration
			r.active = true
			return o.ID
		}
	}
	m.records = append(m.records, &toastRecord{options: o, remaining: o.Duration, active: true})
	return o.ID
}
func (m *ToastManager) Dismiss(id string) {
	for _, r := range m.records {
		if r.options.ID == id {
			r.active = false
		}
	}
}
func (m *ToastManager) Clear() {
	for _, r := range m.records {
		r.active = false
	}
}

// Pause supports application backgrounding. Call after advancing the Surface
// to the backgrounding event time, as for other native input events.
func (m *ToastManager) Pause(v bool) { m.paused = v }
func (m *ToastManager) Count() int {
	n := 0
	for _, r := range m.records {
		if r.active {
			n++
		}
	}
	return n
}
func (m *ToastManager) advance(now time.Duration) bool {
	if !m.initialized {
		m.last = now
		m.initialized = true
		return false
	}
	elapsed := max(time.Duration(0), now-m.last)
	m.last = now
	changed := false
	if m.paused || m.interacting {
		return false
	}
	for _, r := range m.records {
		if r.active && r.options.Duration > 0 {
			r.remaining = max(time.Duration(0), r.remaining-elapsed)
			if r.remaining == 0 {
				r.active = false
				changed = true
			}
		}
	}
	return changed
}
func (m *ToastManager) wakeup() (time.Duration, bool) {
	if m.paused || m.interacting {
		return 0, false
	}
	delay := time.Duration(0)
	found := false
	for _, r := range m.records {
		if r.active && r.options.Duration > 0 && (!found || r.remaining < delay) {
			delay = r.remaining
			found = true
		}
	}
	return delay, found
}
func Toast(id string, o ToastOptions, dismiss func()) *Element {
	actions := []*Element{}
	if o.ActionLabel != "" {
		actions = append(actions, Button(id+"/action", o.ActionLabel, o.Action).Variant(Outline).Disabled(o.Action == nil))
	}
	actions = append(actions, Button(id+"/dismiss", "Dismiss", dismiss).Variant(Ghost).Disabled(dismiss == nil))
	e := Item(id, o.Title, o.Description, actions...).Role("status")
	if o.Error {
		e.role = "alert"
	}
	return e
}

// Sonner presents a nonmodal bottom-right notification viewport. Hovering or
// focusing anywhere in it pauses deadlines. It never steals keyboard focus.
func Sonner(id string, manager *ToastManager) *Element {
	if manager == nil {
		panic("baseui: nil toast manager")
	}
	children := []*Element{}
	live := manager.records[:0]
	for _, r := range manager.records {
		if !r.active {
			continue
		}
		live = append(live, r)
		o := r.options
		action := o.Action
		o.Action = func() {
			if !r.active || action == nil {
				return
			}
			r.active = false
			action()
		}
		e := Toast(id+"/"+o.ID, o, func() { r.active = false })
		e.toast = r
		children = append(children, e)
	}
	manager.records = live
	e := Column(id, children...).Width(360).Role("region").Label("Notifications")
	e.kind = ModalKind
	e.passive = true
	e.notifications = manager
	e.padding = 0
	return e
}
func (s *Surface) syncNotifications() {
	if s.closed {
		return
	}
	seen := map[*ToastManager]bool{}
	changed := false
	for _, item := range s.entries {
		m := item.element.notifications
		if m == nil {
			continue
		}
		seen[m] = true
		if !s.notificationManagers[m] {
			m.initialized = false
		}
		changed = m.advance(s.now) || changed
		m.interacting = s.inactive || s.pointerWithin(item.element.id) || s.within(s.focus, item.element.id)
	}
	for m := range s.notificationManagers {
		if !seen[m] {
			m.initialized = false
			m.interacting = false
		}
	}
	s.notificationManagers = seen
	if changed {
		s.relayout()
		if !s.focusEligible(s.focus) {
			s.changeFocus("")
		}
	}
}

// SetActive pauses notification countdowns while a native window is inactive.
// Advance to the event timestamp before changing activation state.
func (s *Surface) SetActive(active bool) {
	if s.closed {
		return
	}
	s.inactive = !active
	if !active {
		for _, item := range s.entries {
			if item.element.onFrame != nil {
				item.element.onFrame(s.now, false, s.theme.ReduceMotion)
			}
		}
	}
	s.SyncNativeViews()
	s.syncNotifications()
}
