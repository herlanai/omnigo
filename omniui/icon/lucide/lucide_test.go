package lucide

import (
	"bytes"
	"image"
	"image/color"
	"sync"
	"testing"
)

func TestEveryBundledIconParsesAndDraws(t *testing.T) {
	names := Names()
	if len(names) != 2350 {
		t.Fatalf("unexpected pinned catalog size: %d", len(names))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			v, e := Get(name)
			if e != nil {
				t.Fatal(e)
			}
			pixels, e := v.Rasterize(24, 24, color.NRGBA{255, 0, 0, 255}, 0)
			if e != nil {
				t.Fatal(e)
			}
			ink := 0
			for y := 0; y < 24; y++ {
				for x := 0; x < 24; x++ {
					r, g, b, a := pixels.At(x, y).RGBA()
					if a > 0 {
						ink++
						if g != 0 || b != 0 || r == 0 {
							t.Fatal("not tinted")
						}
					}
				}
			}
			if ink == 0 {
				t.Fatal("empty raster")
			}
		})
	}
}
func TestLookupOwnershipAndConcurrentRendering(t *testing.T) {
	for _, name := range []string{"../search", "Search", "missing-icon", "search.svg"} {
		if _, e := Get(name); e == nil {
			t.Fatal(name)
		}
	}
	source, _ := SVG("search")
	source[0] = '!'
	again, _ := SVG("search")
	if again[0] == '!' {
		t.Fatal("source ownership")
	}
	v := Search()
	a, _ := v.Rasterize(48, 48, color.NRGBA{20, 90, 180, 128}, 2)
	b, _ := v.Rasterize(48, 48, color.NRGBA{20, 90, 180, 128}, 2)
	if !bytes.Equal(a.(*image.RGBA).Pix, b.(*image.RGBA).Pix) {
		t.Fatal("cache differs")
	}
	a.(*image.RGBA).Pix[0] = 255
	c, _ := v.Rasterize(48, 48, color.NRGBA{20, 90, 180, 128}, 2)
	if c.(*image.RGBA).Pix[0] != 0 {
		t.Fatal("cache aliases result")
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if _, e := v.Rasterize(24+n, 24+n, color.NRGBA{20, 90, 180, 255}, float64(n%3+1)); e != nil {
				t.Error(e)
			}
		}(n)
	}
	wg.Wait()
}
