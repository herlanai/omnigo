package mobile

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"math"
	"testing"
)

func TestCanvasTouchResizeAndBackgroundCancellation(t *testing.T) {
	count, closed := 0, 0
	c, err := newCanvas(Options{Create: func() Content {
		return Content{View: func() *ui.Element { return ui.Column("root", ui.Button("add", "Add", func() { count++ })).Padding(20) }, Close: func() { closed++ }}
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.resize(300, 500, 2); err != nil {
		t.Fatal(err)
	}
	r, ok := c.surface.Bounds("add")
	if !ok {
		t.Fatal("missing button")
	}
	x, y := (r.X+r.W/2)*2, (r.Y+r.H/2)*2
	for _, ev := range []struct {
		id   int64
		kind string
	}{{1, "down"}, {2, "down"}, {2, "up"}, {1, "up"}} {
		if err := c.touch(ev.id, ev.kind, x, y); err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatal("secondary pointer activated control", count)
	}
	c.touch(3, "down", x, y)
	c.activate(false)
	c.activate(true)
	c.touch(3, "up", x, y)
	if count != 1 {
		t.Fatal("background transition activated control")
	}
	if err = c.resize(500, 300, 3); err != nil {
		t.Fatal(err)
	}
	frame, err := c.render(0)
	if err != nil || frame.Bounds().Dx() != 1500 || frame.Bounds().Dy() != 900 {
		t.Fatal("incorrect scaled frame", err)
	}
	if err = c.resize(500, 300, math.NaN()); err == nil {
		t.Fatal("invalid scale accepted")
	}
	if c.width != 500 || c.scale != 3 {
		t.Fatal("invalid resize modified viewport")
	}
	if err = c.close(); err != nil {
		t.Fatal(err)
	}
	c.close()
	if closed != 1 {
		t.Fatal("close not once", closed)
	}
	if err = c.touch(1, "down", x, y); err == nil {
		t.Fatal("input after closure accepted")
	}
}
func TestCanvasRejectsMissingFactoryAndView(t *testing.T) {
	if _, err := newCanvas(Options{}); err == nil {
		t.Fatal("nil factory")
	}
	n := 0
	if _, err := newCanvas(Options{Create: func() Content { return Content{Close: func() { n++ }} }}); err == nil || n != 1 {
		t.Fatal(err, n)
	}
}
