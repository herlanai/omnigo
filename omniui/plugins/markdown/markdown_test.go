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
