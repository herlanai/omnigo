package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
	"time"
)

func TestBorderBeamStudioControls(t *testing.T) {
	d := New("plugin/borderbeam")
	defer d.Close()
	s, _ := ui.NewSurface(960, 820, ui.NewTheme(false))
	defer s.Close()
	mount := func() {
		t.Helper()
		if err := s.Mount(d.View()); err != nil {
			t.Fatal(err)
		}
	}
	press := func(id string) {
		t.Helper()
		if !s.Focus(id) {
			t.Fatal("missing control", id)
		}
		if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
			t.Fatal(err)
		}
		mount()
	}
	mount()
	for _, id := range []string{"beam", "beam/sample/0", "beam/sample/1", "beam/sample/2", "beam/sample/3", "beam/speed/slider", "beam/strength/slider", "beam/glow/slider"} {
		if _, ok := s.Bounds(id); !ok {
			t.Fatal("missing reference content", id)
		}
	}
	press("beam/preset/2")
	if d.plugins.beamSpeed != 3.1 {
		t.Fatal("Line duration")
	}
	press("beam/preset/4")
	if d.plugins.beamSpeed != 2.3 {
		t.Fatal("pulse duration")
	}
	s.Render()
	s.Advance(500 * time.Millisecond)
	s.Render()
	press("pause")
	if !d.plugins.paused || s.NeedsAnimation() {
		t.Fatal("pause not synchronized")
	}
	press("beam/hide")
	if !d.plugins.beamHidden || s.NeedsAnimation() {
		t.Fatal("hide not synchronized")
	}
	press("beam/action")
	if d.plugins.paused {
		t.Fatal("hero action did not resume")
	}
}
