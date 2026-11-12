package baseui

import "testing"

func TestTextSnapshotsSurviveIMEAndHistory(t *testing.T) {
	b := NewTextBuffer("a😀中\n")
	initial := b.Snapshot()
	b.Select(1, 2)
	b.SetComposition("界")
	if !b.Snapshot().Same(initial) {
		t.Fatal("uncommitted IME changed snapshot")
	}
	b.CommitComposition()
	if b.String() != "a界中\n" || initial.String() != "a😀中\n" {
		t.Fatal("snapshot/edit")
	}
	changed := b.Snapshot()
	if !b.Undo() || !b.Snapshot().Same(initial) || !b.Redo() || !b.Snapshot().Same(changed) {
		t.Fatal("history lost persistent version")
	}
	var zero TextBuffer
	zero.Replace("😀")
	zero.Select(0, 1)
	if zero.Selected() != "😀" {
		t.Fatal("zero buffer")
	}
}
