package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
)

func TestTextStorageDemoInteraction(t *testing.T) {
	for _, dark := range []bool{false, true} {
		d := New("text-storage")
		s, err := ui.NewSurface(850, 760, ui.NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		mount := func() {
			t.Helper()
			if err := s.Mount(d.View()); err != nil {
				t.Fatal(err)
			}
		}
		mount()
		click := func(id string) {
			t.Helper()
			if !s.Focus(id) {
				t.Fatal(id)
			}
			if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
				t.Fatal(err)
			}
			mount()
		}
		initial := d.textStorage.buffer.String()
		click("demo/snapshot")
		click("demo/load")
		if d.textStorage.buffer.Snapshot().LineCount() != 1001 || d.textStorage.saved.String() != initial {
			t.Fatal("snapshot or load")
		}
		click("demo/restore")
		if d.textStorage.buffer.String() != initial {
			t.Fatal("restore")
		}
		click("demo/undo")
		if d.textStorage.buffer.Snapshot().LineCount() != 1001 {
			t.Fatal("undo")
		}
		click("demo/redo")
		if _, err := s.Render(); err != nil {
			t.Fatal(err)
		}
		s.Close()
		d.Close()
	}
}
