package baseui

import (
	"image/color"
	"math"
	"testing"
)

func TestBreadcrumbGeometryInkBaselineAndInteraction(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scale := range []float64{1, 1.25, 2} {
			s, err := NewSurface(400, 120, NewTheme(dark))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			build := func() *Element {
				return Column("root", Breadcrumb("crumb", []NavigationItem{{ID: "first", Label: "Ag", Action: func() { calls++ }}, {ID: "last", Label: "Ag"}})).Padding(12)
			}
			mount(t, s, build())
			a, _ := s.Bounds("crumb/first")
			b, _ := s.Bounds("crumb/last")
			sep, _ := s.Bounds("crumb/separator/last")
			if a.H != 24 || b.H != 24 || a.Y != b.Y || sep.Y+sep.H/2 != a.Y+a.H/2 || sep.W != 14 {
				t.Fatal("misaligned breadcrumb", a, b, sep)
			}
			if a.X != 12 || sep.X-a.X-a.W != 6 || b.X-sep.X-sep.W != 6 {
				t.Fatal("extra link padding or spacing", a, b, sep)
			}
			img, err := s.RenderScaled(scale, nil)
			if err != nil {
				t.Fatal(err)
			}
			ink := func(r Rect) (int, int) {
				top, bottom := img.Bounds().Max.Y, -1
				bg := color.RGBAModel.Convert(s.theme.Background).(color.RGBA)
				for y := int(math.Floor(r.Y * scale)); y < int(math.Ceil((r.Y+r.H)*scale)); y++ {
					for x := int(math.Ceil(r.X * scale)); x < int(math.Floor((r.X+r.W)*scale)); x++ {
						if img.RGBAAt(x, y) != bg {
							top = min(top, y)
							bottom = max(bottom, y)
						}
					}
				}
				return top, bottom
			}
			at, ab := ink(a)
			bt, bb := ink(b)
			if at != bt || ab != bb || ab < at {
				t.Fatal("actual glyph baselines differ", scale, at, ab, bt, bb)
			}
			click(t, s, "crumb/first")
			if calls != 1 {
				t.Fatal("link not clickable")
			}
			if s.Focus("crumb/last") {
				t.Fatal("current page should not be an action")
			}
			mount(t, s, build().ReadOnly(true))
			s.Focus("crumb/first")
			s.Key(KeyEvent{Key: "Enter"})
			if calls != 1 {
				t.Fatal("read-only breadcrumb activated")
			}
			s.Close()
		}
	}
}
func TestBreadcrumbNarrowLayoutAndDisabledLink(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Breadcrumb("crumb", []NavigationItem{{ID: "first", Label: "A very long parent section", Disabled: true, Action: func() { t.Fatal("disabled action") }}, {ID: "last", Label: "Current page"}})).Width(100))
	a, _ := s.Bounds("crumb/first")
	b, _ := s.Bounds("crumb/last")
	sep, _ := s.Bounds("crumb/separator/last")
	if a.X+a.W > sep.X || sep.X+sep.W > b.X || b.X+b.W > 100.001 {
		t.Fatal("narrow breadcrumb overlaps", a, sep, b)
	}
	if s.Focus("crumb/first") {
		t.Fatal("disabled link focused")
	}
	if _, err := s.Render(); err != nil {
		t.Fatal(err)
	}
}
