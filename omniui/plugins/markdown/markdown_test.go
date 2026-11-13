package markdown

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strings"
	"testing"
)

func TestNativeBlocksStylesLinksAndCodeCopy(t *testing.T) {
	source := "# Heading\n\n**Bold** and *italic* with `code`, ~~removed~~ and [open][ref].\n\n> Quote\n\n1. First\n2. Second\n\n- [x] Done\n\n| Key | Value |\n| --- | --- |\n| a | b |\n\n```go\n\tprintln(\"hello\")\n```\n\n[ref]: https://example.test\n"
	for _, dark := range []bool{false, true} {
		var state State
		opened, copied := "", ""
		root, e := state.View("md", source, Options{OnLink: func(v string) { opened = v }, OnCopyCode: func(v string) { copied = v }})
		if e != nil {
			t.Fatal(e)
		}
		s, _ := ui.NewSurface(360, 900, ui.NewTheme(dark))
		defer s.Close()
		if e = s.Mount(root); e != nil {
			t.Fatal(e)
		}
		heading, table, code := false, false, false
		for _, n := range s.Accessibility() {
			heading = heading || n.Role == "heading"
			table = table || n.Role == "table"
			if strings.Contains(n.Description, "https://example.test") {
				s.Focus(n.ID)
				s.Key(ui.KeyEvent{Key: "Enter"})
			}
			if strings.HasSuffix(n.ID, "/copy") {
				code = true
				s.Focus(n.ID)
				s.Key(ui.KeyEvent{Key: "Enter"})
			}
		}
		if !heading || !table || !code || opened != "https://example.test" || copied != "\tprintln(\"hello\")\n" {
			t.Fatal(heading, table, code, opened, copied)
		}
		if _, e = s.RenderScaled(1.25, nil); e != nil {
			t.Fatal(e)
		}
	}
}
func TestUnsafeURLsLiteralHTMLAndAppend(t *testing.T) {
	for _, v := range []string{"javascript:alert(1)", "data:text/html,test", "//evil.test", "https://user:pass@example.test", "https://example.test\n"} {
		if SafeLink(v) {
			t.Fatal("unsafe", v)
		}
	}
	var s State
	root, e := s.View("md", "Text [bad](javascript:alert(1))\n\n<script>alert(1)</script>", Options{OnLink: func(string) { t.Fatal("unsafe activation") }})
	if e != nil {
		t.Fatal(e)
	}
	surface, _ := ui.NewSurface(400, 300, ui.NewTheme(false))
	defer surface.Close()
	if e = surface.Mount(root); e != nil {
		t.Fatal(e)
	}
	for _, n := range surface.Accessibility() {
		if strings.Contains(n.Description, "javascript:") {
			t.Fatal("unsafe interactive link")
		}
	}
	old := s.doc
	_, e = s.View("md", old.Source(), Options{})
	if e != nil || s.doc != old {
		t.Fatal("unchanged document reparsed")
	}
	_, e = s.View("md", "```go\npart", Options{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.View("md", "```go\npartial\n```\n", Options{})
	if e != nil || s.doc.Source() != "```go\npartial\n```\n" {
		t.Fatal(e)
	}
	if _, e = Parse(strings.Repeat("x", MaxSourceBytes+1)); e == nil {
		t.Fatal("source limit ignored")
	}
}

func TestAlignedTableTasksAlertsAndWrappedCode(t *testing.T) {
	source := "| Left | Middle | Right |\n| :--- | :---: | ---: |\n| Long content which wraps over multiple lines | x | 42 |\n\n- [x] Done\n- [ ] Pending\n\n> [!TIP]\n> Keep **formatting** and [links](/docs).\n>\n> Another paragraph.\n\n```go\n\tprintln(\"" + strings.Repeat("x", 100) + "\")\n```\n"
	for _, dark := range []bool{false, true} {
		var state State
		copied := ""
		root, err := state.View("md", source, Options{WrapCode: true, OnCopyCode: func(v string) { copied = v }})
		if err != nil {
			t.Fatal(err)
		}
		s, _ := ui.NewSurface(420, 1400, ui.NewTheme(dark))
		defer s.Close()
		if err = s.Mount(root); err != nil {
			t.Fatal(err)
		}
		var headers, cells []ui.Rect
		tasks, notes := 0, 0
		for _, n := range s.Accessibility() {
			if n.Role == "columnheader" {
				b, _ := s.Bounds(n.ID)
				headers = append(headers, b)
			}
			if n.Role == "cell" {
				b, _ := s.Bounds(n.ID)
				cells = append(cells, b)
			}
			if n.Role == "checkbox" {
				tasks++
				if s.Focus(n.ID) {
					t.Fatal("read-only task is interactive")
				}
			}
			if n.Role == "note" {
				notes++
			}
			if strings.HasSuffix(n.ID, "/copy") {
				s.Focus(n.ID)
				s.Key(ui.KeyEvent{Key: "Enter"})
			}
		}
		if len(headers) != 3 || len(cells) != 3 || tasks != 2 || notes != 1 {
			t.Fatalf("headers %d cells %d tasks %d notes %d", len(headers), len(cells), tasks, notes)
		}
		for i := range headers {
			if headers[i].X != cells[i].X || headers[i].W != cells[i].W {
				t.Fatal("columns do not align", headers, cells)
			}
		}
		if copied != "\tprintln(\""+strings.Repeat("x", 100)+"\")\n" {
			t.Fatalf("copy changed source %q", copied)
		}
		for _, c := range state.code {
			if !c.Wrap || c.Language != "go" {
				t.Fatal("code options lost")
			}
		}
		if _, err = s.RenderScaled(1.25, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplicationImagesAndAlertMarkers(t *testing.T) {
	var state State
	called := 0
	_, err := state.View("md", "![A **local** image](asset.png \"Preview\")", Options{Image: func(id, url, title, alt string) *ui.Element {
		called++
		if url != "asset.png" || title != "Preview" || alt != "A local image" {
			t.Fatal(url, title, alt)
		}
		return ui.Text(id, alt)
	}})
	if err != nil || called != 1 {
		t.Fatal(called, err)
	}
	_, err = state.View("md", "![bad](javascript:alert)\n\n![bad](data:image/png,abc)", Options{Image: func(string, string, string, string) *ui.Element { t.Fatal("unsafe image callback"); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"NOTE", "TIP", "IMPORTANT", "WARNING", "CAUTION"} {
		doc, err := Parse("> [!" + kind + "]\n> A **strong** message")
		if err != nil {
			t.Fatal(err)
		}
		_, runs, ok := alertContent(doc.root.FirstChild(), doc.source)
		if !ok {
			t.Fatal("alert not detected", kind)
		}
		var text strings.Builder
		bold := false
		for _, run := range runs {
			text.WriteString(run.Text)
			bold = bold || run.Bold
		}
		if text.String() != "A strong message" || !bold {
			t.Fatal(text.String(), bold)
		}
	}
}
