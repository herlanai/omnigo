package terminal

import (
	"bytes"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strings"
	"testing"
)

func TestVTChunkBoundariesColorsAndErasing(t *testing.T) {
	input := []byte("hello\rWORLD\r\n\x1b[31mred\x1b[0m \x1b[38;2;10;20;30mRGB\x1b[0m\r\nUnicode: é\x1b[2;2H!\x1b[K")
	a, _ := NewScreen(24, 5)
	b, _ := NewScreen(24, 5)
	if e := a.Feed(input); e != nil {
		t.Fatal(e)
	}
	for _, v := range input {
		if e := b.Feed([]byte{v}); e != nil {
			t.Fatal(e)
		}
	}
	ac, bc := a.Cells(), b.Cells()
	for i := range ac {
		if ac[i] != bc[i] {
			t.Fatalf("chunk boundary changed cell %d", i)
		}
	}
	if !strings.HasPrefix(a.Text(), "WORLD") {
		t.Fatal(a.Text())
	}
	if ac[24].Foreground != indexedColor(1) || ac[25].Rune != '!' {
		t.Fatal(ac[24:27])
	}
	if !strings.Contains(a.Text(), "é") {
		t.Fatal("UTF-8 lost")
	}
}
func TestVTAlternateResizeHistoryAndLimits(t *testing.T) {
	s, _ := NewScreen(8, 3)
	s.HistoryLimit = 2
	s.Feed([]byte("main\x1b[?1049hALT\x1b[s\x1b[H"))
	if e := s.Resize(5, 2); e != nil {
		t.Fatal(e)
	}
	s.Feed([]byte("\x1b[?1049l"))
	if !strings.HasPrefix(s.Text(), "main") {
		t.Fatal("main screen lost", s.Text())
	}
	s.Feed([]byte("\r\n1\r\n2\r\n3\r\n4"))
	if len(s.History()) != 2 {
		t.Fatal("history unbounded")
	}
	snapshot := s.Cells()
	snapshot[0].Rune = 'X'
	if s.Cells()[0].Rune == 'X' {
		t.Fatal("snapshot aliases screen")
	}
	s.Feed([]byte("\x1b]52;c;" + strings.Repeat("a", 10000) + "\a"))
	if len(s.sequence) > 256 || len(s.pending) > 4 {
		t.Fatal("unbounded escape state")
	}
	if _, e := NewScreen(1000, 1000); e == nil {
		t.Fatal("screen budget ignored")
	}
}
func TestVTViewportInputReadOnlyAndSnapshot(t *testing.T) {
	for _, dark := range []bool{false, true} {
		screen, _ := NewScreen(20, 4)
		screen.Feed([]byte("\x1b[32mready\x1b[0m"))
		s, _ := ui.NewSurface(320, 180, ui.NewTheme(dark))
		defer s.Close()
		var sent []byte
		build := func() *ui.Element {
			return View("vt", screen, ViewOptions{OnInput: func(v []byte) { sent = append(sent, v...) }})
		}
		if e := s.Mount(build()); e != nil {
			t.Fatal(e)
		}
		s.Focus("vt")
		s.TextInput("ls")
		s.Key(ui.KeyEvent{Key: "Enter"})
		s.Key(ui.KeyEvent{Key: "Up"})
		if string(sent) != "ls\r\x1b[A" {
			t.Fatalf("%q", sent)
		}
		a, _ := s.RenderScaled(2, nil)
		screen.Feed([]byte("changed"))
		b, _ := s.RenderScaled(2, nil)
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Fatal("mounted viewport changed without remount")
		}
		sent = nil
		s.Mount(build().ReadOnly(true))
		s.Focus("vt")
		s.TextInput("blocked")
		s.Key(ui.KeyEvent{Key: "Enter"})
		if len(sent) > 0 {
			t.Fatal("readonly wrote to terminal")
		}
		s.Mount(build().Disabled(true))
		if s.Focus("vt") {
			t.Fatal("disabled terminal focused")
		}
	}
}
