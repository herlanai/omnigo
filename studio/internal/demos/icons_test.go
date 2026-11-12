package demos

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/icon/lucide"
	"strings"
	"testing"
)

func TestNativeIconSelectionAndCodeCopy(t *testing.T) {
	d := New("icon")
	copied := ""
	d.WriteClipboard = func(text string) bool { copied = text; return true }
	s, err := ui.NewSurface(1040, 1200, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	build := func() {
		t.Helper()
		if err = s.Mount(d.View()); err != nil {
			t.Fatal(err)
		}
	}
	build()
	name := lucide.Names()[0]
	if !s.Focus("icons/select/" + name) {
		t.Fatal("selection button not focusable")
	}
	if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	build()
	if d.icons.selected != name {
		t.Fatal("icon selection did not update")
	}
	d.icons.size = 32
	d.icons.stroke = 1.5
	build()
	if !s.Focus("icons/code/copy") {
		t.Fatal("code copy not focusable")
	}
	if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copied, "lucide.Must(\""+name+"\")") || !strings.Contains(copied, "Size: 32, StrokeWidth: 1.5") {
		t.Fatal("code does not match selected icon", copied)
	}
	d.icons.query.Set("no-such-icon-at-all")
	build()
	if _, err = s.Render(); err != nil {
		t.Fatal(err)
	}
}
