package baseui

import "testing"

func TestResizePreservesFocusTextAndBounds(t *testing.T) {
	s, err := NewSurface(320, 240, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	text := NewTextBuffer("hello")
	if err = s.Mount(Column("root", Input("input", "Text", text))); err != nil {
		t.Fatal(err)
	}
	if !s.Focus("input") {
		t.Fatal("focus")
	}
	s.SetComposition("preedit")
	if err = s.Resize(640, 480); err != nil {
		t.Fatal(err)
	}
	if s.Focused() != "input" || text.String() != "hello" || text.Composition() != "preedit" {
		t.Fatal("resize lost state")
	}
	img, err := s.Render()
	if err != nil || img.Bounds().Dx() != 640 || img.Bounds().Dy() != 480 {
		t.Fatal("wrong resized bounds", err)
	}
	if err = s.Resize(-1, 20); err == nil {
		t.Fatal("negative dimensions accepted")
	}
	img, _ = s.Render()
	if img.Bounds().Dx() != 640 {
		t.Fatal("invalid resize changed bounds")
	}
	s.Close()
	if s.Resize(100, 100) == nil {
		t.Fatal("resize after close")
	}
}
