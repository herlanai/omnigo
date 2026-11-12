package baseui

// notifyPointerHover tracks only visible, enabled targets. Callbacks receive
// immutable logical bounds, never a Painter, and execute on the UI goroutine.
func (s *Surface) notifyPointerHover(target string, ev PointerEvent) error {
	item, ok := s.enabled(target)
	if !ok || item.element.onHover == nil || item.element.readOnly {
		target = ""
	}
	if s.hoverListener != target {
		old := s.hoverListener
		s.hoverListener = ""
		if i, exists := s.index[old]; exists {
			previous := s.entries[i]
			if previous.element.onHover != nil {
				if err := guarded(func() {
					kind := "leave"
					if ev.Kind == "cancel" {
						kind = "cancel"
					}
					previous.element.onHover(PointerEvent{Kind: kind}, previous.bounds)
				}); err != nil {
					return err
				}
			}
		}
	}
	if target == "" {
		return nil
	}
	s.hoverListener = target
	return guarded(func() { item.element.onHover(ev, item.bounds) })
}
