package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
)

func TestTitlebarToggleRemainsReachable(t *testing.T) {
	for _, id := range []string{"button", "terminal", "plugin/borderbeam"} {
		d := New(id)
		d.TitleBarAvailable = true
		s, _ := ui.NewSurface(960, 820, ui.NewTheme(false))
		for _, want := range []bool{true, false, true, false} {
			if err := s.Mount(d.View()); err != nil {
				t.Fatal(err)
			}
			if !s.Focus("window/titlebar") {
				t.Fatal("titlebar toggle unreachable", id)
			}
			if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
				t.Fatal(err)
			}
			if d.TitleBarHidden != want {
				t.Fatal("titlebar state not toggled", id)
			}
		}
		s.Close()
		d.Close()
	}
}
