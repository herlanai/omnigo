package baseui

import (
	"fmt"
	"image"
	"math"
	"testing"
)

func benchmarkControls(b *testing.B, scale float64, reuse bool) {
	s, err := NewSurface(640, 480, NewTheme(false))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	rows := make([]*Element, 8)
	for i := range rows {
		rows[i] = Row(fmt.Sprintf("row%d", i), Label(fmt.Sprintf("label%d", i), "Account settings").Width(170), Button(fmt.Sprintf("button%d", i), "Save changes", nil), Badge(fmt.Sprintf("badge%d", i), "Ready"))
	}
	if err = s.Mount(Card("card", rows...).Padding(20)); err != nil {
		b.Fatal(err)
	}
	var dst *image.RGBA
	if reuse {
		dst = image.NewRGBA(image.Rect(0, 0, int(math.Ceil(640*scale)), int(math.Ceil(480*scale))))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if reuse {
			err = s.RenderScaledInto(dst, scale, nil)
		} else {
			_, err = s.RenderScaled(scale, nil)
		}
		if err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkControlsRender1x(b *testing.B) { benchmarkControls(b, 1, false) }
func BenchmarkControlsRender2x(b *testing.B) { benchmarkControls(b, 2, false) }

func BenchmarkControlsRenderReuse2x(b *testing.B) { benchmarkControls(b, 2, true) }
