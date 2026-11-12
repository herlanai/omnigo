package game

import (
	"math"
	"testing"
	"time"
)

func TestFixedStepsPauseAndCatchup(t *testing.T) {
	l := Loop{Step: 10 * time.Millisecond, MaxSteps: 3}
	n := 0
	update := func(time.Duration) { n++ }
	l.Advance(0, false, update)
	f, e := l.Advance(25*time.Millisecond, false, update)
	if e != nil || n != 2 || f.Alpha != .5 {
		t.Fatal(f, e, n)
	}
	l.Advance(30*time.Millisecond, true, update)
	l.Advance(time.Second, false, update)
	if n != 2 {
		t.Fatal("pause caught up")
	}
	f, e = l.Advance(2*time.Second, false, update)
	if e != nil || f.Steps != 3 || f.Dropped == 0 {
		t.Fatal(f, e)
	}
	if _, e = l.Advance(0, false, update); e == nil {
		t.Fatal("backward clock accepted")
	}
}
func TestCameraAndFit(t *testing.T) {
	c := Camera{Position: Point{2, 3}, Zoom: 2, Rotation: .7}
	p := Point{6, -5}
	v := Point{800, 600}
	q := c.ScreenToWorld(c.WorldToScreen(p, v), v)
	if math.Hypot(q.X-p.X, q.Y-p.Y) > 1e-9 {
		t.Fatal(q)
	}
	if f := IntegerFit(800, 600, 320, 180); f.Scale != 2 || f.X != 80 || f.Y != 120 {
		t.Fatal(f)
	}
}
