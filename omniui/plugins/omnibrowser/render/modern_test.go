package render

import (
	"image"
	"image/color"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"testing"
)

func TestModernLayoutPixels(t *testing.T) {
	source := `<style>body{margin:0}.row{display:flex;width:200px;height:80px;justify-content:space-between;align-items:center}.row i{display:block;width:20px;height:20px;background:red}.grid{display:grid;width:200px;grid-template-columns:1fr 3fr}.grid i{height:20px;background:blue}.grid i+i{background:green}.aspect{width:80px;aspect-ratio:2;position:relative}.fill{position:absolute;inset:0;background:red}</style><div class="row"><i></i><i></i></div><div class="grid"><i></i><i></i></div><div class="aspect"><div class="fill"></div></div>`
	img, e := Screenshot(omnibrowser.Snapshot{HTML: source}, 240, 180, false)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range []struct {
		x, y int
		want color.RGBA
	}{{5, 35, color.RGBA{255, 0, 0, 255}}, {185, 35, color.RGBA{255, 0, 0, 255}}, {95, 35, color.RGBA{255, 255, 255, 255}}, {45, 85, color.RGBA{0, 0, 255, 255}}, {55, 85, color.RGBA{0, 128, 0, 255}}, {75, 135, color.RGBA{255, 0, 0, 255}}, {75, 145, color.RGBA{255, 255, 255, 255}}} {
		if got := img.RGBAAt(p.x, p.y); got != p.want {
			t.Fatalf("(%d,%d)=%v want %v", p.x, p.y, got, p.want)
		}
	}
}
func TestGradientAndTransformedEllipse(t *testing.T) {
	img, e := Screenshot(omnibrowser.Snapshot{HTML: `<style>body{margin:0}div{width:100px;height:40px;background:linear-gradient(90deg,red,blue);transform:translate(40px,40px) rotate(90deg)}</style><div></div>`}, 180, 140, false)
	if e != nil {
		t.Fatal(e)
	}
	a, b := img.RGBAAt(90, 20), img.RGBAAt(90, 100)
	if a.R < a.B || b.B < b.R || a.G > 20 || b.G > 20 {
		t.Fatal("gradient transform", a, b)
	}
	r := image.Rect(0, 0, 100, 40)
	if ellipseCoverage(r, 10, 5) > 0 || ellipseCoverage(r, 50, 5) < .9 {
		t.Fatal("ellipse rendered as a capsule")
	}
}
func TestCSSMathAndGradientBounds(t *testing.T) {
	for v, want := range map[string]float64{"clamp(56px,5.9vw,82px)": 59, "calc(100% - 20px)": 180, "min(40px, 8vw)": 40, "max(1rem, 2px)": 16} {
		if got := viewportLength(v, 200, -1, 1000, 600); got != want {
			t.Fatal(v, got, want)
		}
	}
	if got := parseGradient("linear-gradient(90deg, red 0%, blue 100%)"); got == nil {
		t.Fatal("gradient rejected")
	}
}

func TestInlineBlockMarginsAreCountedOnce(t *testing.T) {
	img, e := Screenshot(omnibrowser.Snapshot{HTML: `<style>body{margin:0}main{width:120px}span{display:inline-block;margin-right:20px;background:red}</style><main><span>abc</span><span>abc</span></main>`}, 180, 100, false)
	if e != nil {
		t.Fatal(e)
	}
	// Both labels fit in the first line; a doubled right margin wraps the second.
	if c := img.RGBAAt(60, 2); c.R != 255 || c.G != 0 {
		t.Fatal("margin wrapped second inline block", c)
	}
}
