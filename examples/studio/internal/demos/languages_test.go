package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
)

func TestNativeLanguageSwitchPreservesApplicationState(t *testing.T) {
	for _, dark := range []bool{false, true} {
		d := New("i18n")
		s, err := ui.NewSurface(900, 700, ui.NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Mount(d.View()); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"demo/add", "demo/add", "demo/de"} {
			if !s.Focus(id) {
				t.Fatal("missing control", id)
			}
			if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
				t.Fatal(err)
			}
			if err = s.Mount(d.View()); err != nil {
				t.Fatal(err)
			}
		}
		if d.Count != 2 || d.language.Language() != "de" {
			t.Fatal("lost state or locale")
		}
		text, err := d.language.Text("files", map[string]any{"n": d.Count})
		if err != nil || text != "2 Dateien ausgewählt" {
			t.Fatal(text, err)
		}
		if _, err = s.Render(); err != nil {
			t.Fatal(err)
		}
		s.Close()
		d.Close()
	}
}
