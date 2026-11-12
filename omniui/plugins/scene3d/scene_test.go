package scene3d

import (
	"image/color"
	"testing"
)

func TestDepthAndClipping(t *testing.T) {
	red := color.NRGBA{255, 0, 0, 255}
	blue := color.NRGBA{0, 0, 255, 255}
	near := Triangle{Vec3{-1, -1, 1}, Vec3{1, -1, 1}, Vec3{0, 1, 1}, red}
	far := Triangle{Vec3{-1, -1, 0}, Vec3{1, -1, 0}, Vec3{0, 1, 0}, blue}
	for _, triangles := range [][]Triangle{{near, far}, {far, near}} {
		img, stats, e := Render(Scene{Triangles: triangles}, DefaultCamera(), 80, 80)
		if e != nil || stats.Pixels == 0 || img.NRGBAAt(40, 40) != red {
			t.Fatal(stats, e)
		}
	}
	cross := near
	cross.A.Z = 3.1
	img, stats, e := Render(Scene{Triangles: []Triangle{cross}}, DefaultCamera(), 80, 80)
	if e != nil || img == nil || stats.ClippedTriangles == 0 {
		t.Fatal("near-plane clipping", stats, e)
	}
	if _, _, e = Render(Scene{}, DefaultCamera(), 3000, 1); e == nil {
		t.Fatal("unbounded target")
	}
}
