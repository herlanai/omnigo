package borderbeam

import (
	"bytes"
	"image/color"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"testing"
	"time"
)

func TestEffectsCachePauseDisableAndPaletteOwnership(t *testing.T) {
	for preset := Medium; preset <= PulseOutside; preset++ {
		for _, dark := range []bool{false, true} {
			surface, err := ui.NewSurface(240, 180, ui.NewTheme(dark))
			if err != nil {
				t.Fatal(err)
			}
			var state State
			colors := []color.NRGBA{{240, 30, 100, 255}, {10, 190, 240, 255}}
			o := Options{Preset: preset, Colors: colors}
			build := func() {
				t.Helper()
				if err := surface.Mount(ui.Column("root", state.Box("beam", o, ui.Text("text", "Native")).Height(80)).Padding(40)); err != nil {
					t.Fatal(err)
				}
			}
			render := func() []byte {
				t.Helper()
				im, err := surface.Render()
				if err != nil {
					t.Fatal(err)
				}
				return im.Pix
			}
			build()
			first := render()
			colors[0] = color.NRGBA{0, 0, 0, 0}
			if !bytes.Equal(first, render()) {
				t.Fatal("caller colors escaped snapshot")
			}
			surface.Advance(400 * time.Millisecond)
			second := render()
			if bytes.Equal(first, second) {
				t.Fatal("stationary preset", preset)
			}
			o.Colors = []color.NRGBA{{240, 30, 100, 255}, {10, 190, 240, 255}}
			o.Paused = true
			build()
			held := render()
			texture := state.image
			surface.Advance(1400 * time.Millisecond)
			if !bytes.Equal(held, render()) || surface.NeedsAnimation() || texture != state.image {
				t.Fatal("pause or cache failed")
			}
			o.Glow = 45
			build()
			render()
			if state.key.glow <= 28 {
				t.Fatal("cached glow did not invalidate")
			}
			o.Disabled = true
			build()
			render()
			if state.image != nil || surface.NeedsAnimation() {
				t.Fatal("disabled effect retains texture or requests frames")
			}
			surface.Close()
		}
	}
}
func TestPulseOutsideAndInsideHaveDifferentFootprints(t *testing.T) {
	surface, _ := ui.NewSurface(240, 200, ui.NewTheme(true))
	defer surface.Close()
	var state State
	build := func(p Preset) {
		if err := surface.Mount(ui.Column("root", state.Box("beam", Options{Preset: p, Palette: Ocean}, ui.Text("label", "Light")).Height(80)).Padding(50)); err != nil {
			t.Fatal(err)
		}
	}
	build(PulseOutside)
	surface.Render()
	surface.Advance(600 * time.Millisecond)
	out, _ := surface.Render()
	build(PulseInner)
	in, _ := surface.Render()
	if bytes.Equal(out.Pix, in.Pix) {
		t.Fatal("pulse modes have identical footprint")
	}
	if out.RGBAAt(44, 80) == in.RGBAAt(44, 80) {
		t.Fatal("outer glow clipped to the box")
	}
}
func TestReducedMotionAndHiddenStopFrames(t *testing.T) {
	theme := ui.NewTheme(false)
	theme.ReduceMotion = true
	surface, _ := ui.NewSurface(240, 180, theme)
	defer surface.Close()
	var state State
	surface.Mount(state.Box("beam", Options{Palette: Colorful}, ui.Text("text", "Static")))
	a, _ := surface.Render()
	surface.Advance(time.Second)
	b, _ := surface.Render()
	if !bytes.Equal(a.Pix, b.Pix) || surface.NeedsAnimation() {
		t.Fatal("reduced motion animates")
	}
	surface.Mount(state.Box("beam", Options{}, ui.Text("text", "Hidden")).Hidden(true))
	if surface.NeedsAnimation() {
		t.Fatal("hidden requests frames")
	}
}
