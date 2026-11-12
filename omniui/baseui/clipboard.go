package baseui

// CopySelection exposes committed selected text to a platform clipboard
// adapter. It never performs OS calls and does not export IME preedit text.
func (s *Surface) CopySelection() (string, bool) {
	if s.closed {
		return "", false
	}
	item, ok := s.enabled(s.focus)
	if !ok || item.element.buffer == nil {
		return "", false
	}
	text := item.element.buffer.Selected()
	return text, text != ""
}

// CutSelection returns text only if the focused input can be edited. Platform
// adapters should write the returned text only when ok is true and err is nil.
func (s *Surface) CutSelection() (text string, ok bool, err error) {
	if s.closed {
		return "", false, nil
	}
	item, available := s.enabled(s.focus)
	if !available || item.element.buffer == nil || item.element.readOnly || item.element.buffer.Composition() != "" {
		return "", false, nil
	}
	text = item.element.buffer.Selected()
	if text == "" {
		return "", false, nil
	}
	err = guarded(func() { s.replaceInput(item.element, "") })
	return text, true, err
}

// Paste uses the normal editing path, including single-line normalization,
// OTP digit filtering, read-only checks, limits and one undo step.
func (s *Surface) Paste(text string) error { return s.TextInput(text) }
