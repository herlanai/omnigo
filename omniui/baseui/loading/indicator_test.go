package loading

import (
	"bytes"
	"testing"
	"time"

	"omnigo.dev/omnigo/omniui/baseui"
)

func TestPauseAndReducedMotion(t *testing.T) {
	for _, style := range []Style{Ring, Dots, Bars, Clock, Ripple, Orbit, Pulse} {
		i := New(style)
		s, err := baseui.NewSurface(80, 60, baseui.NewTheme(false))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Mount(i.View("indicator", "Loading", Options{Size: 24})); err != nil {
			t.Fatal(err)
		}
		a, _ := s.Render()
		s.Advance(200 * time.Millisecond)
		b, _ := s.Render()
		if bytes.Equal(a.Pix, b.Pix) {
			t.Fatal("indicator did not animate", style)
		}
		s.Mount(i.View("indicator", "Loading", Options{Size: 24, Paused: true}))
		a, _ = s.Render()
		s.Advance(500 * time.Millisecond)
		b, _ = s.Render()
		if !bytes.Equal(a.Pix, b.Pix) || s.NeedsAnimation() {
			t.Fatal("paused indicator animates", style)
		}
		s.Close()
		theme := baseui.NewTheme(true)
		theme.ReduceMotion = true
		s, err = baseui.NewSurface(80, 60, theme)
		if err != nil {
			t.Fatal(err)
		}
		s.Mount(New(style).View("reduced", "Loading", Options{Size: 24}))
		a, _ = s.Render()
		s.Advance(time.Second / 3)
		b, _ = s.Render()
		if !bytes.Equal(a.Pix, b.Pix) || s.NeedsAnimation() {
			t.Fatal("reduced motion animates", style)
		}
		s.Close()
	}
}
