package demos

import (
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/text/rope"
	"strings"
)

type textStorageDemo struct {
	buffer   *ui.TextBuffer
	saved    rope.Text
	hasSaved bool
}

func (d *Demo) textStorageContent() *ui.Element {
	if d.textStorage == nil {
		d.textStorage = &textStorageDemo{buffer: ui.NewTextBuffer("Persistent text snapshots\nEdit this document, save a version, then continue editing.\nUnicode: café · Ελληνικά\n")}
	}
	state := d.textStorage
	doc := state.buffer.Snapshot()
	sum := doc.Summary()
	_, caret := state.buffer.Selection()
	at := doc.FromRune(caret)
	saved := "No snapshot saved"
	if state.hasSaved {
		saved = fmt.Sprintf("Saved version: %d bytes · %d lines\n%s", state.saved.Len(), state.saved.LineCount(), state.saved.Slice(0, state.saved.FromRune(80)))
	}
	return ui.Column("demo/text-storage",
		ui.Text("demo/text-stats", fmt.Sprintf("%d bytes · %d runes · %d UTF-16 units · %d lines", sum.Bytes, sum.Runes, sum.UTF16, doc.LineCount())),
		ui.Text("demo/text-position", fmt.Sprintf("Caret: byte %d · UTF-16 %d · line %d", at, doc.UTF16(at), doc.LineOf(at)+1)).Muted(true),
		ui.Textarea("demo/document", "Document", state.buffer).Height(220),
		ui.Row("demo/history", ui.Button("demo/snapshot", "Save snapshot", func() { state.saved = state.buffer.Snapshot(); state.hasSaved = true }), ui.Button("demo/restore", "Restore snapshot", func() { state.buffer.Set(state.saved.String()) }).Disabled(!state.hasSaved), ui.Button("demo/undo", "Undo", func() { state.buffer.Undo() }), ui.Button("demo/redo", "Redo", func() { state.buffer.Redo() })).Wrap(true).Gap(8),
		ui.Row("demo/fixtures", ui.Button("demo/load", "Load 1,000 lines", func() { state.buffer.Set(strings.Repeat("Editable text · café · λ\n", 1000)) }), ui.Button("demo/insert", "Insert at selection", func() { state.buffer.Replace("[inserted]") })).Wrap(true).Gap(8),
		ui.Text("demo/saved", saved).Muted(true),
	).Gap(12)
}
