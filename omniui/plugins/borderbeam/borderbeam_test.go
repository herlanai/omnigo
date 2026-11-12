package borderbeam

import (
	"bytes"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
	"time"
)

func TestPresetsPauseAndChildren(t *testing.T) {
	for preset := Medium; preset <= PulseOutside; preset++ {
		s, _ := ui.NewSurface(240, 120, ui.NewTheme(false))
		state := State{}
		clicks := 0
		build := func(paused bool) *ui.Element {
			return state.Box("box", Options{Preset: preset, Paused: paused}, ui.Button("action", "Run", func() { clicks++ }))
		}
		if e := s.Mount(build(false)); e != nil {
			t.Fatal(e)
		}
		a, e := s.Render()
		if e != nil {
			t.Fatal(e)
		}
		s.Advance(300 * time.Millisecond)
		b, e := s.Render()
		if e != nil || bytes.Equal(a.Pix, b.Pix) {
			t.Fatal("no animation", preset, e)
		}
		s.Focus("action")
		s.Key(ui.KeyEvent{Key: "Enter"})
		if clicks != 1 {
			t.Fatal("decoration blocked child")
		}
		s.Mount(build(true))
		a, _ = s.Render()
		s.Advance(time.Second)
		b, _ = s.Render()
		if !bytes.Equal(a.Pix, b.Pix) || s.NeedsAnimation() {
			t.Fatal("pause failed")
		}
		s.Close()
	}
}
