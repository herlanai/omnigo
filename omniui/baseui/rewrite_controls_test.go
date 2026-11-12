package baseui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"io"
	"testing"
	"time"
)

func TestVirtualMillionRowsBuildsOnlyViewportAndKeyboardEnd(t *testing.T) {
	s := surface(t)
	state := &VirtualListState{}
	built := 0
	options := VirtualListOptions{Count: 1000000, RowHeight: 48, Height: 192, Overscan: 2, Disabled: func(i int) bool { return i == 1 }}
	build := func() *Element {
		built = 0
		return VirtualList("list", state, options, func(id string, i int) *Element { built++; return Text(id, "content") })
	}
	mount(t, s, build())
	if built > 8 {
		t.Fatal("materialized offscreen rows", built)
	}
	s.Focus("list/item/0/select")
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, build())
	if state.Selected != "2" {
		t.Fatal("disabled row not skipped", state.Selected)
	}
	s.Key(KeyEvent{Key: "End"})
	mount(t, s, build())
	if state.Selected != "999999" || s.Focused() != "list/item/999999/select" || built > 8 {
		t.Fatal("end navigation", state.Selected, s.Focused(), built)
	}
	if _, ok := s.Bounds("list/item/0/select"); ok {
		t.Fatal("offscreen row retained")
	}
	options.Count = 3
	mount(t, s, build())
	if state.Scroll.Y != 0 {
		t.Fatal("shrunk list offset", state.Scroll.Y)
	}
	mount(t, s, build().ReadOnly(true))
	s.Focus("list/item/0/select")
	s.Key(KeyEvent{Key: "Down"})
	if state.Selected != "999999" {
		t.Fatal("readonly selection changed")
	}
}
func TestVirtualGridRowsAndDuplicateKeys(t *testing.T) {
	s := surface(t)
	state := &VirtualListState{}
	calls := 0
	o := GridViewOptions{VirtualListOptions: VirtualListOptions{Count: 1000000, RowHeight: 60, Height: 120, Overscan: 1}, Width: 310, MinWidth: 90, Gap: 10}
	build := func() *Element {
		return GridView("grid", state, o, func(id string, i int) *Element { calls++; return Text(id, "cell") })
	}
	mount(t, s, build())
	if calls > 12 {
		t.Fatal(calls)
	}
	s.Focus("grid/item/0/select")
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, build())
	if state.Selected != "3" {
		t.Fatal("2D navigation", state.Selected)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate keys accepted")
		}
	}()
	o.Key = func(int) string { return "same" }
	build()
}
func TestTimelineExplicitClockPauseRepeatAndUnmount(t *testing.T) {
	s := surface(t)
	state := &AnimationTimeline{Duration: time.Second, Repeat: 1, Alternate: true}
	build := func() *Element { return Timeline("t", state, func(*Painter, Rect, float64) {}) }
	mount(t, s, build())
	s.Advance(0)
	s.Advance(500 * time.Millisecond)
	if state.Progress() != .5 {
		t.Fatal(state.Progress())
	}
	state.Paused = true
	s.Advance(time.Second)
	if state.Progress() != .5 || s.NeedsAnimation() {
		t.Fatal("paused timer")
	}
	state.Paused = false
	s.Advance(1500 * time.Millisecond)
	if state.Progress() != 1 {
		t.Fatal("resume", state.Progress())
	}
	mount(t, s, Text("empty", "unmounted"))
	s.Advance(5 * time.Second)
	mount(t, s, build())
	s.Advance(5 * time.Second)
	if state.Progress() != 1 {
		t.Fatal("hidden time counted")
	}
	s.Advance(6 * time.Second)
	if !state.Settled() || state.Progress() != 0 || s.NeedsAnimation() {
		t.Fatal("alternate completion", state.Progress())
	}
	state.Seek(250 * time.Millisecond)
	if state.Tween(0, 100, 0, time.Second, false) != 25 {
		t.Fatal("seek/tween")
	}
	state.Restart()
	mount(t, s, build().Disabled(true))
	s.Advance(7 * time.Second)
	s.Advance(8 * time.Second)
	if state.Progress() != 0 {
		t.Fatal("disabled timeline advanced")
	}
}
func TestVideoFramesPauseSeekAndOwnership(t *testing.T) {
	red := image.NewRGBA(image.Rect(0, 0, 2, 2))
	red.Set(0, 0, color.RGBA{R: 255, A: 255})
	blue := image.NewRGBA(red.Bounds())
	blue.Set(0, 0, color.RGBA{B: 255, A: 255})
	p, err := NewVideoPlayer([]VideoFrame{{red, time.Second}, {blue, time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	red.Set(0, 0, color.RGBA{})
	if p.frames[0].Image.At(0, 0) != (color.RGBA{R: 255, A: 255}) {
		t.Fatal("pixels aliased")
	}
	s := surface(t)
	build := func() *Element { return Video("v", p) }
	mount(t, s, build())
	s.Advance(0)
	first, _ := s.Render()
	s.Advance(time.Second)
	second, _ := s.Render()
	if bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("video did not advance")
	}
	p.Pause(true)
	s.Advance(10 * time.Second)
	if p.Position() != time.Second || s.NeedsAnimation() {
		t.Fatal("paused video")
	}
	p.Seek(0)
	if p.Position() != 0 {
		t.Fatal("seek")
	}
	p.Close()
	if p.frames != nil || p.ends != nil {
		t.Fatal("frames retained")
	}
	if _, err = s.Render(); err != nil {
		t.Fatal(err)
	}
}

type testVideoDecoder struct {
	frame image.Image
	count int
}

func (d *testVideoDecoder) Next(ctx context.Context) (VideoFrame, error) {
	if d.count == 2 {
		return VideoFrame{}, io.EOF
	}
	d.count++
	return VideoFrame{d.frame, time.Second}, nil
}
func TestVideoDecoderCancellationAndGIFDisposal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := &testVideoDecoder{frame: image.NewRGBA(image.Rect(0, 0, 1, 1))}
	if _, err := DecodeVideo(ctx, d); err != context.Canceled || d.count != 0 {
		t.Fatal("decode cancellation", err)
	}
	p, err := DecodeVideo(context.Background(), d)
	if err != nil || len(p.frames) != 2 {
		t.Fatal(err)
	}
	palette := color.Palette{color.Transparent, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	a := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	a.SetColorIndex(0, 0, 1)
	b := image.NewPaletted(image.Rect(1, 0, 2, 1), palette)
	b.SetColorIndex(1, 0, 2)
	p, err = NewGIFVideo(&gif.GIF{Image: []*image.Paletted{a, b}, Delay: []int{10, 10}, Disposal: []byte{gif.DisposalBackground, 0}, Config: image.Config{Width: 2, Height: 1}, LoopCount: -1})
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := p.frames[1].Image.At(0, 0).RGBA()
	if alpha != 0 {
		t.Fatal("background disposal retained previous frame")
	}
}
func TestCodeEditorIndentFindIMEAndHistory(t *testing.T) {
	s := surface(t)
	state := &CodeEditorState{Text: NewTextBuffer("func main() {\n    value := 1\n}")}
	build := func() *Element { return CodeEditor("code", state, CodeEditorOptions{Language: "go"}) }
	mount(t, s, build())
	s.Focus("code/input")
	state.Text.Select(17, 17)
	s.Key(KeyEvent{Key: "Tab"})
	mount(t, s, build())
	if state.Text.String() == "func main() {\n    value := 1\n}" {
		t.Fatal("tab not inserted")
	}
	if !state.FindNext("value", false) || state.Text.Selected() != "value" {
		t.Fatal("find")
	}
	original := state.Text.String()
	s.SetComposition("中")
	s.Key(KeyEvent{Key: "Enter"})
	if state.Text.String() != original {
		t.Fatal("IME intercepted")
	}
	state.Text.CancelComposition()
	s.Key(KeyEvent{Key: "Z", Command: true})
	if state.Text.String() != "func main() {\n    value := 1\n}" {
		t.Fatal("undo")
	}
	mount(t, s, build().ReadOnly(true))
	s.Focus("code/input")
	before := state.Text.String()
	s.Key(KeyEvent{Key: "Tab"})
	s.TextInput("bad")
	if state.Text.String() != before {
		t.Fatal("readonly edit")
	}
}
func TestRichEditorFormatsEditsAndUndoTogether(t *testing.T) {
	s := NewRichEditor("Hello world")
	s.Text.Select(0, 5)
	s.ToggleFormat("bold")
	if !s.Spans()[0].Bold {
		t.Fatal("format")
	}
	s.Text.Select(5, 5)
	s.Text.Replace("!")
	if s.Spans()[0].Text != "Hello!" || !s.Spans()[0].Bold {
		t.Fatal("insert lost mark", s.Spans())
	}
	if !s.Undo() || s.Text.String() != "Hello world" || !s.Spans()[0].Bold {
		t.Fatal("text undo")
	}
	if !s.Undo() || s.Spans()[0].Bold {
		t.Fatal("format undo")
	}
	if !s.Redo() || !s.Spans()[0].Bold {
		t.Fatal("format redo")
	}
	surface := surface(t)
	mount(t, surface, RichTextEditor("rich", s, nil).ReadOnly(true))
	surface.Focus("rich/input")
	surface.Key(KeyEvent{Key: "B", Command: true})
	if !s.Spans()[0].Bold {
		t.Fatal("readonly formatting")
	}
}

type testNativeWeb struct {
	visible, interactive bool
	bounds, clip         Rect
	focused              int
}

func (w *testNativeWeb) Place(b, c Rect, v, i bool) {
	w.bounds, w.clip, w.visible, w.interactive = b, c, v, i
}
func (w *testNativeWeb) Focus() { w.focused++ }
func TestWebViewPlacementAndUnmountLifecycle(t *testing.T) {
	s := surface(t)
	host := &testNativeWeb{}
	state := NewWebViewState(host)
	mount(t, s, WebView("web", state).Width(100).Height(80))
	s.SyncNativeViews()
	if !host.visible || host.bounds.W != 100 {
		t.Fatal("not placed")
	}
	s.SetActive(false)
	if !host.visible {
		t.Fatal("background window lost its web content")
	}
	s.SetActive(true)
	s.Focus("web")
	s.Key(KeyEvent{Key: "Enter"})
	if host.focused != 2 {
		t.Fatal("focus")
	}
	mount(t, s, WebView("web", state).ReadOnly(true))
	s.SyncNativeViews()
	if host.interactive {
		t.Fatal("readonly host")
	}
	mount(t, s, Text("other", "Other"))
	s.SyncNativeViews()
	if host.visible {
		t.Fatal("unmounted web visible")
	}
	mount(t, s, WebView("web", state))
	s.SyncNativeViews()
	s.Close()
	if host.visible {
		t.Fatal("closed web visible")
	}
}

func TestTimelineInactiveWindowDoesNotCatchUp(t *testing.T) {
	s := surface(t)
	state := &AnimationTimeline{Duration: time.Second}
	mount(t, s, Timeline("t", state, func(*Painter, Rect, float64) {}))
	s.Advance(0)
	s.Advance(200 * time.Millisecond)
	s.SetActive(false)
	if s.NeedsAnimation() {
		t.Fatal("inactive animation schedules work")
	}
	s.SetActive(true)
	s.Advance(10 * time.Second)
	if state.Position != 200*time.Millisecond {
		t.Fatal("inactive time counted", state.Position)
	}
}

func BenchmarkVirtualListMount(b *testing.B) {
	for _, count := range []int{1000, 1000000} {
		name := "1000"
		if count == 1000000 {
			name = "1000000"
		}
		b.Run(name, func(b *testing.B) {
			s, err := NewSurface(640, 480, NewTheme(false))
			if err != nil {
				b.Fatal(err)
			}
			defer s.Close()
			state := &VirtualListState{}
			options := VirtualListOptions{Count: count, RowHeight: 48, Height: 240, Overscan: 2}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				view := VirtualList("list", state, options, func(id string, i int) *Element { return Text(id, "Row content") })
				if err = s.Mount(view); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
