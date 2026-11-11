package baseui

import (
	"bytes"
	"image/color"
	"math"
	"testing"
	"time"
)

func surface(t testing.TB) *Surface {
	t.Helper()
	s, err := NewSurface(640, 480, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func mount(t testing.TB, s *Surface, root *Element) {
	t.Helper()
	if err := s.Mount(root); err != nil {
		t.Fatal(err)
	}
}
func click(t testing.TB, s *Surface, id string) {
	t.Helper()
	r, ok := s.Bounds(id)
	if !ok {
		t.Fatal("missing", id)
	}
	for _, kind := range []string{"down", "up"} {
		if err := s.Pointer(PointerEvent{X: r.X + r.W/2, Y: r.Y + r.H/2, Kind: kind}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestButtonLifecycleAndSnapshot(t *testing.T) {
	s := surface(t)
	calls := 0
	b := Button("button", "Run", func() { calls++ })
	mount(t, s, Column("root", b, Button("off", "Off", func() { t.Fatal("disabled fired") }).Disabled(true)))
	b.Disabled(true) // Builders no longer own the mounted frame.
	click(t, s, "button")
	if calls != 1 {
		t.Fatal(calls)
	}
	if s.Focus("off") {
		t.Fatal("disabled focused")
	}
	click(t, s, "off")
	if calls != 1 {
		t.Fatal(calls)
	}
	s.Focus("button")
	for _, key := range []KeyEvent{{Key: "Space"}, {Key: "Space", Repeat: true}, {Key: "Space", Up: true}, {Key: "Enter", Repeat: true}} {
		if err := s.Key(key); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatal("activation repeated", calls)
	}
	r, _ := s.Bounds("button")
	s.Pointer(PointerEvent{X: r.X + 2, Y: r.Y + 2, Kind: "down"})
	s.Pointer(PointerEvent{Kind: "cancel"})
	s.Pointer(PointerEvent{X: r.X + 2, Y: r.Y + 2, Kind: "up"})
	if calls != 2 {
		t.Fatal("canceled press fired")
	}
}

func TestAtomicMountAndInheritedDisabled(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Button("keep", "Keep", nil)))
	if err := s.Mount(Column("new", Button("dup", "a", nil), Button("dup", "b", nil))); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, ok := s.Bounds("keep"); !ok {
		t.Fatal("failed mount replaced valid tree")
	}
	if err := s.Mount(Button("nan", "bad", nil).Width(math.NaN())); err == nil {
		t.Fatal("NaN accepted")
	}
	mount(t, s, Column("disabled", Button("child", "No", func() { t.Fatal("disabled ancestor ignored") })).Disabled(true))
	click(t, s, "child")
}

func TestModalFocusIsolationAndReturn(t *testing.T) {
	s := surface(t)
	open := false
	view := func() *Element {
		return Column("root", Button("launch", "Open", func() { open = true }), Button("background", "Background", func() { t.Fatal("modal background activated") }), Dialog("dialog", "Title", open, func() { open = false }, Input("inside", "Name", NewTextBuffer(""))))
	}
	mount(t, s, view())
	click(t, s, "launch")
	mount(t, s, view())
	if s.Focus("background") {
		t.Fatal("background accepted focus")
	}
	for range 8 {
		s.Key(KeyEvent{Key: "Tab"})
		if s.focus == "launch" || s.focus == "background" {
			t.Fatal("focus escaped")
		}
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	if open || s.Focused() != "launch" {
		t.Fatal(open, s.Focused())
	}
}

func TestTextInputSelectionReadOnlyAndComposition(t *testing.T) {
	s := surface(t)
	buffer := NewTextBuffer("世界")
	mount(t, s, Column("root", Input("edit", "Editor", buffer)))
	s.Focus("edit")
	s.SetComposition("你好")
	if buffer.String() != "世界" {
		t.Fatal("composition committed early")
	}
	s.CommitComposition()
	if buffer.String() != "世界你好" {
		t.Fatal(buffer.String())
	}
	s.Key(KeyEvent{Key: "A", Command: true})
	s.TextInput("A\nB")
	if buffer.String() != "AB" {
		t.Fatal(buffer.String())
	}
	s.Key(KeyEvent{Key: "Z", Command: true})
	if buffer.String() != "世界你好" {
		t.Fatal("undo", buffer.String())
	}
	mount(t, s, Column("root", Input("edit", "Editor", buffer).ReadOnly(true)))
	s.TextInput("bad")
	s.Key(KeyEvent{Key: "Backspace"})
	if buffer.String() != "世界你好" {
		t.Fatal("readonly edited")
	}
	s.Key(KeyEvent{Key: "Home"})
	_, caret := buffer.Selection()
	if caret != 0 {
		t.Fatal("readonly navigation blocked")
	}
}

func TestCompositionEnterDoesNotSubmit(t *testing.T) {
	s := surface(t)
	buffer := NewTextBuffer("")
	submissions := 0
	mount(t, s, Input("edit", "Name", buffer).OnSubmit(func() { submissions++ }))
	s.Focus("edit")
	s.SetComposition("你好")
	s.Key(KeyEvent{Key: "Enter"})
	if submissions != 0 || buffer.String() != "" {
		t.Fatal("IME Enter submitted or committed early")
	}
	s.CommitComposition()
	s.Key(KeyEvent{Key: "Enter"})
	if submissions != 1 || buffer.String() != "你好" {
		t.Fatal(submissions, buffer.String())
	}
}

func TestSliderClampAndCallbacks(t *testing.T) {
	s := surface(t)
	value := .5
	mount(t, s, Column("root", Slider("slider", "Value", value, 0, 1, .1, func(n float64) { value = n })))
	s.Focus("slider")
	s.Key(KeyEvent{Key: "End"})
	if value != 1 {
		t.Fatal(value)
	}
	s.Key(KeyEvent{Key: "Right"})
	if value != 1 {
		t.Fatal(value)
	}
	r, _ := s.Bounds("slider")
	s.Pointer(PointerEvent{X: r.X + r.W/2, Y: r.Y + r.H/2, Kind: "down"})
	s.Pointer(PointerEvent{X: -100, Y: r.Y, Kind: "move"})
	s.Pointer(PointerEvent{Kind: "cancel"})
	if value != 0 {
		t.Fatal(value)
	}
}

func TestClipRenderIsolationAndIdle(t *testing.T) {
	s := surface(t)
	mount(t, s, Column("root", Canvas("canvas", "Paint", func(p *Painter, r Rect) {
		p.Fill(Rect{X: -100, Y: -100, W: 1000, H: 1000}, color.NRGBA{R: 255, A: 255}, 0)
	}).Width(20).Height(20)).Padding(10))
	a, err := s.Render()
	if err != nil {
		t.Fatal(err)
	}
	if a.RGBAAt(15, 15).R != 255 {
		t.Fatal("canvas did not draw")
	}
	if got := a.RGBAAt(30, 30); got.R == 255 && got.G == 0 {
		t.Fatal("paint escaped clip")
	}
	b, _ := s.Render()
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("idle render changed")
	}
	a.Pix[0] = 0
	c, _ := s.Render()
	if bytes.Equal(a.Pix, c.Pix) {
		t.Fatal("caller mutation affected surface")
	}
	if s.NeedsAnimation() {
		t.Fatal("static frame animates")
	}
	mount(t, s, Spinner("spin", "Loading"))
	if !s.NeedsAnimation() {
		t.Fatal("spinner not scheduled")
	}
	s.Advance(time.Second)
	if err := s.Advance(0); err == nil {
		t.Fatal("backwards clock accepted")
	}
	reduced := NewTheme(false)
	reduced.ReduceMotion = true
	r, err := NewSurface(100, 100, reduced)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	mount(t, r, Spinner("spin", "Loading"))
	if r.NeedsAnimation() {
		t.Fatal("reduced motion animates")
	}
}

func TestCallbackPanicDoesNotLoseSurface(t *testing.T) {
	s := surface(t)
	mount(t, s, Button("panic", "bad", func() { panic("bad plugin") }))
	s.Focus("panic")
	if err := s.Key(KeyEvent{Key: "Enter"}); err == nil {
		t.Fatal("panic not reported")
	}
	if _, err := s.Render(); err != nil {
		t.Fatal(err)
	}
}

func TestAccessibleNameDoesNotReplaceVisualLabel(t *testing.T) {
	s := surface(t)
	mount(t, s, Button("day", "11", nil).Width(32).Height(32))
	if bounds, _ := s.Bounds("day"); bounds.W != 32 || bounds.H != 32 {
		t.Fatal("root dimensions ignored", bounds)
	}
	a, _ := s.Render()
	mount(t, s, Button("day", "11", nil).Width(32).Height(32).Label("2026-11-11"))
	b, _ := s.Render()
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("accessible name replaced visible label")
	}
	if s.Accessibility()[0].Label != "2026-11-11" {
		t.Fatal("accessible label lost")
	}
}

func BenchmarkMount100Controls(b *testing.B) {
	s := surface(b)
	children := make([]*Element, 100)
	for i := range children {
		children[i] = Text(string(rune(i+100)), "value")
	}
	root := Column("root", children...)
	b.ReportAllocs()
	for b.Loop() {
		if err := s.Mount(root); err != nil {
			b.Fatal(err)
		}
	}
}
