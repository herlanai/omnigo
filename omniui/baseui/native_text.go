package baseui

import (
	"errors"
	"github.com/herlanai/omnigo/internal/text/rope"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// TextRange uses UTF-16 code units, as required by native text services. Location
// -1 means the current selection for edits and no marked range in snapshots.
type TextRange struct{ Location, Length int }
type NativeTextState struct {
	Text             string
	Selected, Marked TextRange
	Editable         bool
}

func utf16Length(text string) int {
	n := 0
	for _, r := range text {
		n += utf16.RuneLen(r)
	}
	return n
}
func rangeBytes(text string, r TextRange) (int, int, bool) {
	if r.Location < 0 || r.Length < 0 {
		return 0, 0, false
	}
	doc := rope.New(text)
	total := doc.Summary().UTF16
	if r.Location > total || r.Length > total-r.Location {
		return 0, 0, false
	}
	a, b := doc.FromUTF16(r.Location), doc.FromUTF16(r.Location+r.Length)
	return a, b, doc.UTF16(a) == r.Location && doc.UTF16(b) == r.Location+r.Length
}

// NativeText returns the focused input's virtual document, including uncommitted
// marked text. Call only on the UI goroutine. Read-only text remains queryable.
func (s *Surface) NativeText() (NativeTextState, bool) { return s.AccessibleText(s.focus) }

// AccessibleText exposes an input's text state to native assistive technology.
// It does not move focus or permit mutation of disabled controls.
func (s *Surface) AccessibleText(id string) (NativeTextState, bool) {
	e, ok := s.enabled(id)
	if !ok || e.element.buffer == nil {
		return NativeTextState{}, false
	}
	b := e.element.buffer
	a, c := b.Selection()
	lo, hi := min(a, c), max(a, c)
	start, end := b.document.FromRune(lo), b.document.FromRune(hi)
	state := NativeTextState{Text: b.text, Selected: TextRange{b.document.UTF16(start), b.document.UTF16(end) - b.document.UTF16(start)}, Marked: TextRange{-1, 0}, Editable: !e.element.readOnly}
	if b.composition != "" {
		state.Text = b.text[:start] + b.composition + b.text[end:]
		state.Marked = TextRange{b.document.UTF16(start), utf16Length(b.composition)}
		selected := b.markedSelection
		if _, _, ok := rangeBytes(b.composition, selected); !ok {
			selected = TextRange{state.Marked.Length, 0}
		}
		state.Selected = TextRange{state.Marked.Location + selected.Location, selected.Length}
	}
	return state, true
}

// EditNativeText replaces a native UTF-16 range. selected is relative to inserted
// text and marks the result; nil commits one undoable edit. Invalid/split-surrogate
// ranges are rejected without changing text, selection or undo history.
func (s *Surface) EditNativeText(text string, replacement TextRange, selected *TextRange) error {
	state, ok := s.NativeText()
	if !ok || !state.Editable {
		return errors.New("baseui: no editable native input")
	}
	text = strings.ToValidUTF8(text, "�")
	if selected != nil {
		if _, _, valid := rangeBytes(text, *selected); !valid {
			return errors.New("baseui: invalid marked selection")
		}
	}
	if replacement.Location == -1 {
		replacement = state.Selected
		if state.Marked.Location >= 0 {
			replacement = state.Marked
		}
	}
	a, z, valid := rangeBytes(state.Text, replacement)
	if !valid {
		return errors.New("baseui: invalid native replacement range")
	}
	item, _ := s.enabled(s.focus)
	b := item.element.buffer
	unionA, unionZ := a, z
	markA, markZ := 0, 0
	originalA, originalZ := b.document.FromRune(min(b.anchor, b.caret)), b.document.FromRune(max(b.anchor, b.caret))
	if state.Marked.Location >= 0 {
		markA, markZ, _ = rangeBytes(state.Text, state.Marked)
		unionA = min(a, markA)
		unionZ = max(z, markZ)
	}
	value := state.Text[unionA:a] + text + state.Text[z:unionZ]
	nativeToOriginal := func(offset int) int {
		if state.Marked.Location < 0 {
			return offset
		}
		if offset <= markA {
			return offset
		}
		if offset >= markZ {
			return originalZ + offset - markZ
		}
		return originalA
	}
	begin, end := nativeToOriginal(unionA), nativeToOriginal(unionZ)
	return guarded(func() {
		b.Select(b.document.RuneOffset(begin), b.document.RuneOffset(end))
		if selected != nil {
			b.SetComposition(value)
			b.markedSelection = TextRange{utf16Length(state.Text[unionA:a]) + selected.Location, selected.Length}
		} else {
			b.CancelComposition()
			s.replaceInput(item.element, value)
		}
	})
}
func (s *Surface) CancelComposition() {
	if item, ok := s.enabled(s.focus); ok && item.element.buffer != nil {
		item.element.buffer.CancelComposition()
	}
}

// NativeTextRect returns an insertion rectangle in surface coordinates for an
// exact UTF-16 boundary. It uses the same font and scrolling rules as painting.
func (s *Surface) NativeTextRect(location int) (Rect, bool) {
	state, ok := s.NativeText()
	if !ok {
		return Rect{}, false
	}
	at, _, valid := rangeBytes(state.Text, TextRange{location, 0})
	if !valid {
		return Rect{}, false
	}
	item, _ := s.enabled(s.focus)
	e := item.element
	metrics := s.textSurface(e)
	prefix := state.Text[:at]
	line := strings.Count(prefix, "\n")
	start := strings.LastIndex(prefix, "\n") + 1
	height := metrics.lineHeight()
	top := item.bounds.Y + 8
	if e.kind == InputKind {
		top = item.bounds.Y + (item.bounds.H-height)/2
		line = 0
	}
	inset := inputTextInset(e)
	x := item.bounds.X + inset + metrics.textWidth(prefix[start:])
	if e.kind == InputKind {
		caret, _, _ := rangeBytes(state.Text, TextRange{state.Selected.Location + state.Selected.Length, 0})
		x -= max(0, metrics.textWidth(state.Text[:caret])-(item.bounds.W-2*inset))
	}
	if e.otpLength > 0 {
		x = item.bounds.X + float64(utf8.RuneCountInString(prefix))*item.bounds.W/float64(e.otpLength)
		top = item.bounds.Y + (item.bounds.H-height)/2
	}
	return Rect{X: x, Y: top + float64(line)*height, W: 1, H: height}, true
}
func (s *Surface) NativeTextIndex(x, y float64) (int, bool) {
	state, ok := s.NativeText()
	if !ok {
		return 0, false
	}
	doc := rope.New(state.Text)
	item, _ := s.enabled(s.focus)
	metrics := s.textSurface(item.element)
	row := 0
	if item.element.kind == TextareaKind {
		row = max(0, min(doc.LineCount()-1, int((y-item.bounds.Y-8)/metrics.lineHeight())))
	}
	start, end := doc.LineStart(row), doc.LineEnd(row)
	for at := start; at < end; {
		_, size := utf8.DecodeRuneInString(state.Text[at:])
		a, _ := s.NativeTextRect(doc.UTF16(at))
		b, _ := s.NativeTextRect(doc.UTF16(at + size))
		if x < (a.X+b.X)/2 {
			return doc.UTF16(at), true
		}
		at += size
	}
	return doc.UTF16(end), true
}

// AccessibilityAction invokes an existing control operation on the UI goroutine.
// Removed, disabled and read-only targets reject mutations; values use normal
// input validation and callbacks rather than directly assigning the buffer.
func (s *Surface) AccessibilityAction(id, action, value string) error {
	item, ok := s.enabled(id)
	if !ok {
		return errors.New("baseui: inaccessible target")
	}
	if action == "focus" {
		if !s.Focus(id) {
			return errors.New("baseui: target cannot receive focus")
		}
		return nil
	}
	if item.element.readOnly {
		return errors.New("baseui: read-only target")
	}
	switch action {
	case "press":
		if item.element.activate == nil {
			return errors.New("baseui: target has no press action")
		}
		return guarded(item.element.activate)
	case "value":
		if item.element.buffer == nil {
			return errors.New("baseui: target has no text value")
		}
		return guarded(func() {
			item.element.buffer.CancelComposition()
			item.element.buffer.Select(0, item.element.buffer.document.Summary().Runes)
			s.replaceInput(item.element, value)
		})
	case "increment", "decrement":
		if item.element.kind != SliderKind {
			return errors.New("baseui: target has no adjustment action")
		}
		if !s.Focus(id) {
			return errors.New("baseui: target cannot receive focus")
		}
		key := "Right"
		if action == "decrement" {
			key = "Left"
		}
		return s.Key(KeyEvent{Key: key})
	}
	return errors.New("baseui: unknown accessibility action")
}

// SelectAccessibleText changes selection without editing content. Read-only
// inputs allow selection. Active composition must be finished or canceled first.
func (s *Surface) SelectAccessibleText(id string, selection TextRange) error {
	item, ok := s.enabled(id)
	if !ok || item.element.buffer == nil {
		return errors.New("baseui: inaccessible text")
	}
	b := item.element.buffer
	if b.composition != "" {
		return errors.New("baseui: selection during composition")
	}
	a, z, valid := rangeBytes(b.String(), selection)
	if !valid {
		return errors.New("baseui: invalid accessible selection")
	}
	b.Select(b.document.RuneOffset(a), b.document.RuneOffset(z))
	return nil
}
