package baseui

import (
	"strings"
	"unicode/utf8"

	"github.com/herlanai/omnigo/internal/text/rope"
)

type textSnapshot struct {
	document      rope.Text
	anchor, caret int
}

// TextBuffer owns text and selection in rune offsets, not byte offsets. IME
// composition remains separate until CommitComposition. Undo is bounded to 100
// edits. Grapheme-cluster navigation remains pending. Native host adapters must use
// UTF-16-aware Surface methods rather than treating rune offsets as OS offsets.
type TextBuffer struct {
	text            string
	document        rope.Text
	anchor, caret   int
	composition     string
	markedSelection TextRange
	undo, redo      []textSnapshot
	Limit           int // Maximum runes; zero is unlimited.
	OnChange        func(string)
}

func NewTextBuffer(text string) *TextBuffer {
	b := &TextBuffer{}
	b.text = strings.ToValidUTF8(text, "�")
	b.document = rope.New(b.text)
	b.anchor = b.document.Summary().Runes
	b.caret = b.anchor
	return b
}
func (b *TextBuffer) String() string        { return b.text }
func (b *TextBuffer) Selection() (int, int) { return b.anchor, b.caret }
func (b *TextBuffer) Composition() string   { return b.composition }
func (b *TextBuffer) Select(anchor, caret int) {
	n := b.document.Summary().Runes
	b.anchor = max(0, min(n, anchor))
	b.caret = max(0, min(n, caret))
}
func (b *TextBuffer) Selected() string {
	return b.document.Slice(b.document.FromRune(min(b.anchor, b.caret)), b.document.FromRune(max(b.anchor, b.caret)))
}

// Snapshot is an immutable, concurrently readable version of the committed text.
// Composition is deliberately separate until committed.
func (b *TextBuffer) Snapshot() TextSnapshot { return b.document }
func (b *TextBuffer) snapshot() textSnapshot { return textSnapshot{b.document, b.anchor, b.caret} }
func (b *TextBuffer) restore(s textSnapshot) {
	b.document = s.document
	b.text = s.document.String()
	b.anchor = s.anchor
	b.caret = s.caret
	b.composition = ""
	if b.OnChange != nil {
		b.OnChange(b.text)
	}
}
func (b *TextBuffer) Replace(text string) { b.replace(text, b.Limit) }
func (b *TextBuffer) replace(text string, limit int) {
	start, end := min(b.anchor, b.caret), max(b.anchor, b.caret)
	insert := []rune(strings.ToValidUTF8(text, "�"))
	if limit > 0 {
		insert = insert[:min(len(insert), max(0, limit-(b.document.Summary().Runes-(end-start))))]
	}
	next := b.document.Replace(b.document.FromRune(start), b.document.FromRune(end), string(insert))
	if next.Same(b.document) {
		b.Select(start+len(insert), start+len(insert))
		return
	}
	b.undo = append(b.undo, b.snapshot())
	if len(b.undo) > 100 {
		copy(b.undo, b.undo[len(b.undo)-100:])
		clear(b.undo[100:])
		b.undo = b.undo[:100]
	}
	b.redo = nil
	b.document = next
	b.text = next.String()
	b.anchor = start + len(insert)
	b.caret = b.anchor
	b.composition = ""
	if b.OnChange != nil {
		b.OnChange(b.text)
	}
}
func (b *TextBuffer) Set(text string) { b.Select(0, utf8.RuneCountInString(b.text)); b.Replace(text) }
func (b *TextBuffer) SetComposition(text string) {
	b.composition = strings.ToValidUTF8(text, "�")
	b.markedSelection = TextRange{utf16Length(b.composition), 0}
}
func (b *TextBuffer) CancelComposition() { b.composition = "" }
func (b *TextBuffer) CommitComposition() { text := b.composition; b.composition = ""; b.Replace(text) }
func (b *TextBuffer) Undo() bool {
	if len(b.undo) == 0 {
		return false
	}
	b.redo = append(b.redo, b.snapshot())
	last := b.undo[len(b.undo)-1]
	b.undo[len(b.undo)-1] = textSnapshot{}
	b.undo = b.undo[:len(b.undo)-1]
	b.restore(last)
	return true
}
func (b *TextBuffer) Redo() bool {
	if len(b.redo) == 0 {
		return false
	}
	b.undo = append(b.undo, b.snapshot())
	last := b.redo[len(b.redo)-1]
	b.redo[len(b.redo)-1] = textSnapshot{}
	b.redo = b.redo[:len(b.redo)-1]
	b.restore(last)
	return true
}
func (b *TextBuffer) key(key KeyEvent, multiline, readOnly bool) bool {
	// Candidate navigation/Enter belongs to the IME until the host commits the
	// composition. Never submit a form or edit committed text in that interval.
	if b.composition != "" {
		if key.Key == "Escape" {
			b.CancelComposition()
		}
		return true
	}
	n := utf8.RuneCountInString(b.text)
	if key.Command {
		switch key.Key {
		case "A":
			b.Select(0, n)
			return true
		case "Z":
			if !readOnly {
				if key.Shift {
					b.Redo()
				} else {
					b.Undo()
				}
			}
			return true
		}
	}
	switch key.Key {
	case "Left", "Right", "Home", "End":
		position := b.caret
		switch key.Key {
		case "Left":
			if !key.Shift && b.anchor != b.caret {
				position = min(b.anchor, b.caret)
			} else {
				position--
			}
		case "Right":
			if !key.Shift && b.anchor != b.caret {
				position = max(b.anchor, b.caret)
			} else {
				position++
			}
		case "Home":
			position = 0
		case "End":
			position = n
		}
		anchor := position
		if key.Shift {
			anchor = b.anchor
		}
		b.Select(anchor, position)
		return true
	case "Backspace", "Delete":
		if readOnly {
			return true
		}
		if b.anchor == b.caret {
			if key.Key == "Backspace" {
				b.Select(b.caret-1, b.caret)
			} else {
				b.Select(b.caret, b.caret+1)
			}
		}
		b.Replace("")
		return true
	case "Enter":
		if multiline && !readOnly {
			b.Replace("\n")
			return true
		}
	case "Escape":
		if b.composition != "" {
			b.CancelComposition()
			return true
		}
	}
	return false
}

// TextSnapshot is an immutable document version. Editing returns a new version.
type TextSnapshot = rope.Text

// TextSummary describes the byte, rune, UTF-16 and newline counts of a snapshot.
type TextSummary = rope.Summary
