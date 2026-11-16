package baseui

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

type menuTestIcon struct{}

func (menuTestIcon) Size() (float64, float64) { return 20, 20 }
func (menuTestIcon) Rasterize(w, h int, c color.NRGBA, _ float64) (image.Image, error) {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(im, im.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return im, nil
}
func TestMenuButtonAlignmentAndActivation(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, e := NewSurface(240, 80, ThemePreset(dark, StyleNova, AccentBlue, false))
		if e != nil {
			t.Fatal(e)
		}
		calls := 0
		b := MenuButton("item", "Agent", menuTestIcon{}, func() { calls++ }).Size(ControlNormal)
		if b.width != 0 {
			t.Fatal("sized menu became an icon-only square")
		}
		if e = s.Mount(Column("root", b)); e != nil {
			t.Fatal(e)
		}
		r, _ := s.Bounds("item")
		if r.W < 80 {
			t.Fatal("label not measured")
		}
		im, e := s.Render()
		if e != nil {
			t.Fatal(e)
		}
		// A solid test icon occupies x=12..31, vertically centered at y=8..27.
		if im.At(14, 10) == im.At(14, 4) {
			t.Fatal("leading icon not vertically centered")
		}
		for _, x := range []float64{16, 100, 230} {
			for _, kind := range []string{"down", "up"} {
				if e = s.Pointer(PointerEvent{X: x, Y: 18, Kind: kind}); e != nil {
					t.Fatal(e)
				}
			}
		}
		if calls != 3 {
			t.Fatalf("whole row activation: %d", calls)
		}
		if e = s.Mount(Column("root", MenuButton("item", "Agent", menuTestIcon{}, func() { calls++ }).Disabled(true))); e != nil {
			t.Fatal(e)
		}
		s.Pointer(PointerEvent{X: 16, Y: 18, Kind: "down"})
		s.Pointer(PointerEvent{X: 16, Y: 18, Kind: "up"})
		if calls != 3 {
			t.Fatal("disabled row activated")
		}
		s.Close()
	}
}
