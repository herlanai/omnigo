package gpu

import (
	"image"
	"image/color"
	"math"
	"testing"
)

type fakeRenderer struct{ closed int }

func (f *fakeRenderer) render(w, h int, _ color.NRGBA, _ []Rect) (*image.RGBA, error) {
	return image.NewRGBA(image.Rect(0, 0, w, h)), nil
}
func (f *fakeRenderer) close() { f.closed++ }
func TestGeometryBudgetAndLifetime(t *testing.T) {
	b := &fakeRenderer{}
	d := &Device{backend: b}
	for _, r := range []Rect{{Width: -1}, {X: float32(math.NaN())}} {
		if _, err := d.Render(1, 1, color.NRGBA{}, []Rect{r}); err == nil {
			t.Fatal("invalid geometry")
		}
	}
	if _, err := d.Render(16000, 16000, color.NRGBA{}, nil); err == nil {
		t.Fatal("budget")
	}
	d.Close()
	d.Close()
	if b.closed != 1 {
		t.Fatal("double release")
	}
	if _, err := d.Render(1, 1, color.NRGBA{}, nil); err == nil {
		t.Fatal("closed render")
	}
}
