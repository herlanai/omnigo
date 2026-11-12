package baseui

import (
	"fmt"
	"strconv"
	"strings"
)

// GoToLineState keeps validation input separate from the document history.
type GoToLineState struct {
	Query *TextBuffer
	Error string
}

// GoToLine selects a one-based line in a document without changing its text.
// The caller can scroll or return focus through onNavigate after selection.
func GoToLine(id string, state *GoToLineState, document *TextBuffer, onNavigate func(int)) *Element {
	if state == nil || document == nil {
		panic("baseui: nil go-to-line state")
	}
	if state.Query == nil {
		state.Query = NewTextBuffer("1")
	}
	total := strings.Count(document.String(), "\n") + 1
	goTo := func() {
		if state.Query.composition != "" {
			return
		}
		line, err := strconv.Atoi(strings.TrimSpace(state.Query.String()))
		if err != nil || line < 1 || line > total {
			state.Error = fmt.Sprintf("Enter a line from 1 to %d", total)
			return
		}
		runes := []rune(document.String())
		offset, current := 0, 1
		for offset < len(runes) && current < line {
			if runes[offset] == '\n' {
				current++
			}
			offset++
		}
		document.Select(offset, offset)
		state.Error = ""
		if onNavigate != nil {
			onNavigate(line)
		}
	}
	input := Input(id+"/line", "Line number", state.Query).Placeholder(fmt.Sprintf("1–%d", total)).Grow(1)
	input.OnKey(func(e KeyEvent) bool {
		if e.Key == "Enter" && !e.Repeat {
			goTo()
			return true
		}
		return false
	})
	return Column(id, Row(id+"/row", input, Button(id+"/go", "Go to line", goTo).Variant(Outline)), Text(id+"/error", state.Error).Role("status").Hidden(state.Error == ""))
}

// CompletionItem is supplied by a language service or the application.
// InsertText is literal text; snippets and automatic trigger detection are separate capabilities.
type CompletionItem struct {
	ID, Label, Detail, InsertText string
	Disabled                      bool
}
type CompletionState struct {
	Search     CommandState
	source     string
	start, end int
	attached   bool
}

// Begin captures a rune range and document snapshot. Stale results cannot overwrite later edits.
func (s *CompletionState) Begin(document *TextBuffer, start, end int) bool {
	s.attached = false
	if document == nil || start < 0 || end < start || end > len([]rune(document.String())) {
		return false
	}
	s.source = document.String()
	s.start, s.end = start, end
	s.attached = true
	s.Search = CommandState{}
	return true
}
func (s *CompletionState) Apply(document *TextBuffer, item CompletionItem) bool {
	if !s.attached || document == nil || document.String() != s.source || document.composition != "" || item.Disabled {
		return false
	}
	document.Select(s.start, s.end)
	document.Replace(item.InsertText)
	s.attached = false
	return true
}

// CompletionMenu provides filtering and keyboard selection through the shared Go command list.
func CompletionMenu(id string, state *CompletionState, document *TextBuffer, items []CompletionItem, onApply func(CompletionItem)) *Element {
	if state == nil {
		panic("baseui: nil completion state")
	}
	entries := make([]CommandItem, len(items))
	for i, item := range items {
		entries[i] = CommandItem{ID: item.ID, Label: item.Label, Shortcut: item.Detail, Disabled: item.Disabled, Action: func() {
			if state.Apply(document, item) && onApply != nil {
				onApply(item)
			}
		}}
	}
	stale := document == nil || !state.attached
	if document != nil {
		stale = stale || document.String() != state.source
	}
	return Command(id, &state.Search, entries).Disabled(stale)
}
