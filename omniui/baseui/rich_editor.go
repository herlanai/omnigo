package baseui

import "strings"

type TextFormat struct{ Bold, Italic, Code, Strike bool }
type richEditSnapshot struct {
	text          string
	formats       []TextFormat
	anchor, caret int
}

// RichEditorState is an editable styled document, with rune-based selection and
// a shared undo history for content and formatting. All access is UI-confined.
type RichEditorState struct {
	Text       *TextBuffer
	formats    []TextFormat
	previous   string
	undo, redo []richEditSnapshot
	onChange   func([]TextSpan)
}

func NewRichEditor(text string) *RichEditorState {
	s := &RichEditorState{Text: NewTextBuffer(text)}
	s.previous = s.Text.String()
	s.formats = make([]TextFormat, len([]rune(s.previous)))
	s.bind()
	return s
}
func (s *RichEditorState) snapshot() richEditSnapshot {
	a, c := s.Text.Selection()
	return richEditSnapshot{s.Text.String(), append([]TextFormat(nil), s.formats...), a, c}
}
func (s *RichEditorState) push(snapshot richEditSnapshot) {
	s.undo = append(s.undo, snapshot)
	if len(s.undo) > 100 {
		copy(s.undo, s.undo[1:])
		s.undo = s.undo[:100]
	}
	s.redo = nil
}
func (s *RichEditorState) notify() {
	if s.onChange != nil {
		s.onChange(s.Spans())
	}
}
func (s *RichEditorState) bind() {
	s.Text.OnChange = func(next string) {
		old, newText := []rune(s.previous), []rune(next)
		a, c := s.Text.Selection()
		s.push(richEditSnapshot{s.previous, append([]TextFormat(nil), s.formats...), min(a, len(old)), min(c, len(old))})
		prefix := 0
		for prefix < min(len(old), len(newText)) && old[prefix] == newText[prefix] {
			prefix++
		}
		suffix := 0
		for suffix < min(len(old)-prefix, len(newText)-prefix) && old[len(old)-suffix-1] == newText[len(newText)-suffix-1] {
			suffix++
		}
		formats := make([]TextFormat, len(newText))
		copy(formats, s.formats[:prefix])
		copy(formats[len(newText)-suffix:], s.formats[len(old)-suffix:])
		inherited := TextFormat{}
		if prefix > 0 {
			inherited = s.formats[prefix-1]
		}
		for i := prefix; i < len(newText)-suffix; i++ {
			formats[i] = inherited
		}
		s.formats, s.previous = formats, next
		s.notify()
	}
}
func (s *RichEditorState) restore(snapshot richEditSnapshot) {
	s.Text = NewTextBuffer(snapshot.text)
	s.Text.Select(snapshot.anchor, snapshot.caret)
	s.formats = append([]TextFormat(nil), snapshot.formats...)
	s.previous = snapshot.text
	s.bind()
	s.notify()
}
func (s *RichEditorState) Undo() bool {
	if len(s.undo) == 0 {
		return false
	}
	s.redo = append(s.redo, s.snapshot())
	old := s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	s.restore(old)
	return true
}
func (s *RichEditorState) Redo() bool {
	if len(s.redo) == 0 {
		return false
	}
	s.undo = append(s.undo, s.snapshot())
	next := s.redo[len(s.redo)-1]
	s.redo = s.redo[:len(s.redo)-1]
	s.restore(next)
	return true
}

// ToggleFormat applies a mark to the selection; it clears the mark if every
// selected rune already has it. Empty selection deliberately makes no change.
func (s *RichEditorState) ToggleFormat(mark string) {
	if s.Text.Composition() != "" {
		return
	}
	a, c := s.Text.Selection()
	lo, hi := min(a, c), max(a, c)
	if lo == hi {
		return
	}
	get := func(f TextFormat) bool {
		switch mark {
		case "bold":
			return f.Bold
		case "italic":
			return f.Italic
		case "code":
			return f.Code
		case "strike":
			return f.Strike
		}
		panic("baseui: unknown text format")
	}
	all := true
	for _, f := range s.formats[lo:hi] {
		all = all && get(f)
	}
	s.push(s.snapshot())
	for i := lo; i < hi; i++ {
		switch mark {
		case "bold":
			s.formats[i].Bold = !all
		case "italic":
			s.formats[i].Italic = !all
		case "code":
			s.formats[i].Code = !all
		case "strike":
			s.formats[i].Strike = !all
		}
	}
	s.notify()
}
func (s *RichEditorState) Spans() []TextSpan {
	runes := []rune(s.Text.String())
	spans := []TextSpan{}
	for i := 0; i < len(runes); {
		end := i + 1
		for end < len(runes) && s.formats[end] == s.formats[i] {
			end++
		}
		f := s.formats[i]
		spans = append(spans, TextSpan{Text: string(runes[i:end]), Bold: f.Bold, Italic: f.Italic, Code: f.Code, Strike: f.Strike})
		i = end
	}
	return spans
}

// RichTextEditor exposes content editing and live styled preview. It is a split
// editor, not yet a WYSIWYG block editor; tables, embeds and inline links remain
// separate capabilities. Formatting buttons preserve the text selection.
func RichTextEditor(id string, state *RichEditorState, onChange func([]TextSpan)) *Element {
	if state == nil || state.Text == nil {
		panic("baseui: nil rich editor")
	}
	state.onChange = onChange
	buttons := []*Element{}
	for _, mark := range []string{"bold", "italic", "code", "strike"} {
		buttons = append(buttons, Button(id+"/"+mark, strings.ToUpper(mark[:1])+mark[1:], func() { state.ToggleFormat(mark) }).Variant(Outline).Size(ControlSmall))
	}
	buttons = append(buttons, Button(id+"/undo", "Undo", func() { state.Undo() }).Variant(Ghost).Disabled(len(state.undo) == 0), Button(id+"/redo", "Redo", func() { state.Redo() }).Variant(Ghost).Disabled(len(state.redo) == 0))
	edit := Textarea(id+"/input", "Document", state.Text).Height(140)
	edit.onKey = func(ev KeyEvent) bool {
		if state.Text.Composition() != "" || !ev.Command {
			return false
		}
		switch ev.Key {
		case "B":
			state.ToggleFormat("bold")
		case "I":
			state.ToggleFormat("italic")
		case "Z":
			if ev.Shift {
				state.Redo()
			} else {
				state.Undo()
			}
		default:
			return false
		}
		return true
	}
	return Column(id, Row(id+"/toolbar", buttons...).Gap(4), edit, Label(id+"/preview-label", "Formatted preview"), RichText(id+"/preview", state.Spans(), RichTextOptions{Size: 14})).Gap(12)
}
