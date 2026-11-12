// Package scene3d is an optional CPU triangle renderer with perspective,
// six-plane clipping and depth testing. It is not a GPU engine.
package scene3d

import (
	"errors"
	"image"
	"image/color"
	"math"
)

type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(v float64) Vec3 { return Vec3{a.X * v, a.Y * v, a.Z * v} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) unit() Vec3 {
	n := math.Sqrt(a.Dot(a))
	if n == 0 {
		return Vec3{}
	}
	return a.Scale(1 / n)
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func valid(v Vec3) bool     { return finite(v.X) && finite(v.Y) && finite(v.Z) }

type Camera struct {
	Position, Target, Up Vec3
	FOV, Near, Far       float64
}

func DefaultCamera() Camera {
	return Camera{Position: Vec3{0, 0, 3}, Target: Vec3{}, Up: Vec3{0, 1, 0}, FOV: math.Pi / 3, Near: .1, Far: 100}
}

type Triangle struct {
	A, B, C Vec3
	Color   color.NRGBA
}
type Scene struct {
	Triangles     []Triangle
	Background    color.NRGBA
	CullBackfaces bool
}
type Stats struct{ InputTriangles, ClippedTriangles, Pixels int }

func Render(scene Scene, c Camera, width, height int) (*image.NRGBA, Stats, error) {
	var stats Stats
	if width < 1 || height < 1 || width > 2048 || height > 2048 || len(scene.Triangles) > 1000000 {
		return nil, stats, errors.New("scene3d: render budget exceeded")
	}
	if !valid(c.Position) || !valid(c.Target) || !valid(c.Up) || !finite(c.FOV) || !finite(c.Near) || !finite(c.Far) || c.FOV <= 0 || c.FOV >= math.Pi || c.Near <= 0 || c.Far <= c.Near {
		return nil, stats, errors.New("scene3d: invalid camera")
	}
	forward := c.Target.Sub(c.Position).unit()
	right := forward.Cross(c.Up).unit()
	up := right.Cross(forward)
	if right.Dot(right) == 0 || forward.Dot(forward) == 0 {
		return nil, stats, errors.New("scene3d: degenerate camera")
	}
	tangent := math.Tan(c.FOV / 2)
	aspect := float64(width) / float64(height)
	out := image.NewNRGBA(image.Rect(0, 0, width, height))
	depth := make([]float64, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			out.SetNRGBA(x, y, scene.Background)
		}
	}
	planes := []func(Vec3) float64{func(v Vec3) float64 { return v.Z - c.Near }, func(v Vec3) float64 { return c.Far - v.Z }, func(v Vec3) float64 { return v.Z*tangent*aspect + v.X }, func(v Vec3) float64 { return v.Z*tangent*aspect - v.X }, func(v Vec3) float64 { return v.Z*tangent + v.Y }, func(v Vec3) float64 { return v.Z*tangent - v.Y }}
	view := func(v Vec3) Vec3 { v = v.Sub(c.Position); return Vec3{v.Dot(right), v.Dot(up), v.Dot(forward)} }
	project := func(v Vec3) Vec3 {
		return Vec3{(v.X/(v.Z*tangent*aspect) + 1) * float64(width) / 2, (1 - v.Y/(v.Z*tangent)) * float64(height) / 2, 1 / v.Z}
	}
	for _, triangle := range scene.Triangles {
		if !valid(triangle.A) || !valid(triangle.B) || !valid(triangle.C) {
			return nil, stats, errors.New("scene3d: nonfinite vertex")
		}
		stats.InputTriangles++
		polygon := []Vec3{view(triangle.A), view(triangle.B), view(triangle.C)}
		for _, distance := range planes {
			if len(polygon) == 0 {
				break
			}
			var clipped []Vec3
			previous := polygon[len(polygon)-1]
			pd := distance(previous)
			for _, current := range polygon {
				cd := distance(current)
				if (pd >= 0) != (cd >= 0) {
					clipped = append(clipped, previous.Add(current.Sub(previous).Scale(pd/(pd-cd))))
				}
				if cd >= 0 {
					clipped = append(clipped, current)
				}
				previous, pd = current, cd
			}
			polygon = clipped
		}
		for i := 1; i+1 < len(polygon); i++ {
			a, b, c := project(polygon[0]), project(polygon[i]), project(polygon[i+1])
			stats.ClippedTriangles++
			edge := func(a, b Vec3, x, y float64) float64 { return (b.X-a.X)*(y-a.Y) - (b.Y-a.Y)*(x-a.X) }
			area := edge(a, b, c.X, c.Y)
			if math.Abs(area) < 1e-12 || !finite(area) || (scene.CullBackfaces && area >= 0) {
				continue
			}
			x0, x1 := max(0, int(math.Floor(min(a.X, b.X, c.X)))), min(width-1, int(math.Ceil(max(a.X, b.X, c.X))))
			y0, y1 := max(0, int(math.Floor(min(a.Y, b.Y, c.Y)))), min(height-1, int(math.Ceil(max(a.Y, b.Y, c.Y))))
			for y := y0; y <= y1; y++ {
				for x := x0; x <= x1; x++ {
					px, py := float64(x)+.5, float64(y)+.5
					wa, wb, wc := edge(b, c, px, py)/area, edge(c, a, px, py)/area, edge(a, b, px, py)/area
					if wa < 0 || wb < 0 || wc < 0 {
						continue
					}
					z := wa*a.Z + wb*b.Z + wc*c.Z
					at := y*width + x
					if z > depth[at] {
						depth[at] = z
						out.SetNRGBA(x, y, triangle.Color)
						stats.Pixels++
					}
				}
			}
		}
	}
	return out, stats, nil
}
