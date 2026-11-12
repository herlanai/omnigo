package baseui

import "errors"

// Resize relayouts the mounted tree while preserving control state and focus.
// Like Mount it must be called on the owning UI goroutine.
func (s *Surface) Resize(width, height int) error {
	if s.closed {
		return errors.New("baseui: surface closed")
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || int64(width)*int64(height) > 64_000_000 {
		return errors.New("baseui: invalid surface dimensions")
	}
	if s.width == width && s.height == height {
		return nil
	}
	oldWidth, oldHeight := s.width, s.height
	s.width, s.height = width, height
	if s.root != nil {
		if err := s.Mount(s.root); err != nil {
			s.width, s.height = oldWidth, oldHeight
			return err
		}
	}
	return nil
}
