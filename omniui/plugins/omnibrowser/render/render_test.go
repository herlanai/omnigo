package render

import (
	"bytes"
	"context"
	"testing"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
)

func TestScreenshotReflectsDOMMutationAndBounds(t *testing.T) {
	page := omnibrowser.New(omnibrowser.Options{})
	defer page.Close()
	ctx := context.Background()
	if err := page.SetContent(ctx, `<h1 id="title">Before</h1><p>Native Go preview</p>`); err != nil {
		t.Fatal(err)
	}
	a, _ := page.Snapshot(ctx)
	light, err := Screenshot(a, 400, 240, false)
	if err != nil {
		t.Fatal(err)
	}
	dark, err := Screenshot(a, 400, 240, true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(light.Pix, dark.Pix) {
		t.Fatal("theme ignored")
	}
	if _, err = page.Eval(ctx, `document.querySelector('#title').textContent='After'`); err != nil {
		t.Fatal(err)
	}
	b, _ := page.Snapshot(ctx)
	after, err := Screenshot(b, 400, 240, false)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(light.Pix, after.Pix) {
		t.Fatal("DOM mutation not rendered")
	}
	if _, err = Screenshot(a, 9000, 400, false); err == nil {
		t.Fatal("unbounded image")
	}
}
func TestSemanticPreviewInteraction(t *testing.T) {
	snapshot := omnibrowser.Snapshot{HTML: `<h1>Form</h1><input id="name" value="Go"><button id="action">Run</button>`}
	s, err := ui.NewSurface(400, 240, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	clicked := ""
	if err = s.Mount(View("preview", snapshot, func(selector string) { clicked = selector }, nil)); err != nil {
		t.Fatal(err)
	}
	for _, node := range s.Accessibility() {
		if node.Role == "button" && node.Label == "Run" {
			if !s.Focus(node.ID) {
				t.Fatal("button focus")
			}
			if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if clicked != `[id="action"]` {
		t.Fatal(clicked)
	}
}
