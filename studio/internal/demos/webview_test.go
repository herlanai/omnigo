package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strings"
	"testing"
)

type webDemoFake struct {
	html          string
	zoom          float64
	back, forward int
	visible       bool
}

func (w *webDemoFake) LoadHTML(s string)             { w.html = s }
func (w *webDemoFake) Back()                         { w.back++ }
func (w *webDemoFake) Forward()                      { w.forward++ }
func (w *webDemoFake) SetZoom(v float64)             { w.zoom = v }
func (w *webDemoFake) Place(_, _ ui.Rect, v, _ bool) { w.visible = v }
func (w *webDemoFake) Focus()                        {}
func TestWebViewToolbar(t *testing.T) {
	for _, dark := range []bool{false, true} {
		d := New("web-view")
		defer d.Close()
		w := &webDemoFake{}
		d.NativeWeb = ui.NewWebViewState(w)
		d.webDemo = &webViewDemo{page: w, zoom: 1, dark: dark}
		s, err := ui.NewSurface(1040, 840, ui.NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		mount := func() {
			t.Helper()
			if err := s.Mount(d.webViewContent()); err != nil {
				t.Fatal(err)
			}
			s.SyncNativeViews()
		}
		click := func(id string) {
			t.Helper()
			mount()
			if !s.Focus(id) {
				t.Fatal(id)
			}
			if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
				t.Fatal(err)
			}
			mount()
		}
		click("web/about")
		if !strings.Contains(w.html, "Go layout + WebKit") {
			t.Fatal("about not loaded")
		}
		click("web/notes")
		if !strings.Contains(w.html, "textarea") {
			t.Fatal("notes not loaded")
		}
		click("web/zoom")
		if w.zoom != 1.25 {
			t.Fatal(w.zoom)
		}
		click("web/back")
		click("web/forward")
		if w.back != 1 || w.forward != 1 {
			t.Fatal("navigation")
		}
		click("web/visibility")
		if w.visible {
			t.Fatal("hidden page still visible")
		}
		click("web/visibility")
		if !w.visible {
			t.Fatal("page not restored")
		}
	}
}
