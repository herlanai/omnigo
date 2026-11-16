package host

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	ui "github.com/herlanai/omnigo/omniui/baseui"
)

func TestContentAndOptionsValidation(t *testing.T) {
	if _, err := (ContentWindowOptions{}).normalized(); err == nil {
		t.Fatal("missing content accepted")
	}
	c, err := (ContentWindowOptions{View: func() *ui.Element { return ui.Text("t", "Hello") }}).normalized()
	if err != nil || c.Width != 1040 || c.Height != 840 {
		t.Fatal(c, err)
	}
	if _, err := (ContentWindowOptions{Width: 100000, View: c.View}).normalized(); err == nil {
		t.Fatal("oversize")
	}
	if err := Run(nil, Options{}); err == nil {
		t.Fatal("nil context")
	}
	var host Controller
	if err := host.Open(context.Background(), "missing", false); err == nil {
		t.Fatal("zero host")
	}
}
func TestFrameworkHostHasNoStudioDependency(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range tree.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(path, "github.com/herlanai/omnigo/examples/studio") {
				t.Fatal("framework imports application", file.Name(), path)
			}
		}
	}
}

func TestContentWindowNativeChromeOptions(t *testing.T) {
	base := ContentWindowOptions{Width: 800, Height: 600, View: func() *ui.Element { return ui.Text("t", "test") }}
	for _, m := range [][2]int{{-1, 0}, {0, -1}, {801, 0}, {0, 601}} {
		c := base
		c.MinWidth, c.MinHeight = m[0], m[1]
		if _, err := c.normalized(); err == nil {
			t.Fatal("accepted invalid minimum", m)
		}
	}
	layout := &WindowButtonLayout{Left: 16, Top: 17}
	base.MinWidth, base.MinHeight, base.WindowButtons = 400, 300, layout
	c, err := base.normalized()
	if err != nil {
		t.Fatal(err)
	}
	if c.WindowButtons.Spacing != 22 || layout.Spacing != 0 {
		t.Fatal("defaults mutate caller")
	}
	layout.Left = 99
	if c.WindowButtons.Left != 16 {
		t.Fatal("layout not snapshotted")
	}
	for _, spacing := range []float64{-1, 1, 15} {
		base.WindowButtons = &WindowButtonLayout{Spacing: spacing}
		if _, err := base.normalized(); err == nil {
			t.Fatal("accepted overlapping buttons")
		}
	}
}
