package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
)

func TestShortcutDemoPhysicalModifiersAndRepeat(t *testing.T) {
	d := New("shortcuts")
	s, err := ui.NewSurface(850, 700, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defer d.Close()
	mount := func() {
		t.Helper()
		if err := s.Mount(d.View()); err != nil {
			t.Fatal(err)
		}
	}
	mount()
	d.shortcuts.goos = "windows"
	mount()
	if !s.Focus("demo/shortcut-probe") {
		t.Fatal("focus")
	}
	for _, e := range []ui.KeyEvent{{Key: "K", Shift: true, Super: true}, {Key: "K", Shift: true, Ctrl: true, Repeat: true}, {Key: "K", Shift: true, Ctrl: true, Up: true}, {Key: "K", Shift: true, Ctrl: true}} {
		if err := s.Key(e); err != nil {
			t.Fatal(err)
		}
	}
	if d.shortcuts.activations != 1 {
		t.Fatal("incorrect dispatch", d.shortcuts.activations)
	}
	d.shortcuts.goos = "darwin"
	mount()
	if err := s.Key(ui.KeyEvent{Key: "K", Shift: true, Super: true}); err != nil {
		t.Fatal(err)
	}
	if d.shortcuts.activations != 2 {
		t.Fatal("mac command")
	}
}
