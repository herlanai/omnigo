package baseui_test

import (
	"bytes"
	"image/color"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/icon/lucide"
	"testing"
)

func TestIconDPIThemeAndButtonState(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, e := ui.NewSurface(200, 100, ui.NewTheme(dark))
		if e != nil {
			t.Fatal(e)
		}
		defer s.Close()
		calls := 0
		button := ui.IconButton("copy", "Copy", lucide.Copy(), func() { calls++ }).Variant(ui.Outline)
		view := func() *ui.Element {
			return ui.Row("row", ui.Icon("decorative", "", lucide.Search(), ui.IconOptions{}), ui.Icon("meaningful", "Settings", lucide.Settings(), ui.IconOptions{Size: 32, Color: color.NRGBA{20, 100, 220, 255}, StrokeWidth: 1.5}), button)
		}
		if e = s.Mount(view()); e != nil {
			t.Fatal(e)
		}
		for _, scale := range []float64{1, 1.25, 2} {
			pixels, e := s.RenderScaled(scale, nil)
			if e != nil {
				t.Fatal(e)
			}
			if pixels.Bounds().Dx() != int(200*scale) {
				t.Fatal("DPI")
			}
		}
		if s.Focus("decorative") || s.Focus("meaningful") {
			t.Fatal("image focused")
		}
		if !s.Focus("copy") {
			t.Fatal("button not focused")
		}
		s.Key(ui.KeyEvent{Key: "Enter"})
		if calls != 1 {
			t.Fatal(calls)
		}
		before, _ := s.Render()
		button.Variant(ui.Destructive)
		after, _ := s.Render()
		if !bytes.Equal(before.Pix, after.Pix) {
			t.Fatal("paint retained builder")
		}
		s.Mount(view().Disabled(true))
		if s.Focus("copy") {
			t.Fatal("disabled focus")
		}
		s.Key(ui.KeyEvent{Key: "Enter"})
		if calls != 1 {
			t.Fatal("disabled activated")
		}
		s.Mount(view().ReadOnly(true))
		s.Focus("copy")
		s.Key(ui.KeyEvent{Key: "Enter"})
		if calls != 1 {
			t.Fatal("read-only activated")
		}
		labeled, decorative := false, false
		for _, node := range s.Accessibility() {
			if node.ID == "meaningful" {
				labeled = node.Role == "img"
			}
			if node.ID == "decorative" {
				decorative = node.Role == "presentation"
			}
		}
		if !labeled || !decorative {
			t.Fatal("image semantics")
		}
	}
}
