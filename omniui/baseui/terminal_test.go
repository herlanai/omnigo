package baseui

import (
	"strings"
	"testing"
)

func TestTerminalReferenceStatesAndDisclosure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(480, 500, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		open := DisclosureState{Open: true}
		scroll := ScrollState{}
		exit := 1
		out := TerminalOutput{Command: "go test", Output: "a\tb\n\nfailed\n", ExitCode: &exit}
		copied := ""
		build := func() *Element {
			return Terminal("term", out, &scroll, TerminalOptions{Open: &open, OnCopy: func(v string) { copied = v }})
		}
		mount(t, s, build())
		if e := s.entries[s.index["term/status"]].element; e.text != "Failed" || e.variant != Destructive {
			t.Fatal("failed exit not destructive")
		}
		click(t, s, "term/copy")
		if copied != out.Output {
			t.Fatal("copy changed original bytes")
		}
		click(t, s, "term/toggle")
		mount(t, s, build())
		if open.Open {
			t.Fatal("did not collapse")
		}
		if _, ok := s.Bounds("term/text"); ok {
			t.Fatal("collapsed output mounted")
		}
		if !s.Focus("term/toggle") {
			t.Fatal("header not focusable")
		}
		if err := s.Key(KeyEvent{Key: "Enter"}); err != nil {
			t.Fatal(err)
		}
		mount(t, s, build())
		if !open.Open {
			t.Fatal("keyboard did not expand")
		}
		out.Running = true
		mount(t, s, build())
		if s.entries[s.index["term/status"]].element.text != "Running" {
			t.Fatal("running precedence")
		}
		out.Running = false
		out.ExitCode = nil
		mount(t, s, build())
		if s.entries[s.index["term/status"]].element.text != "Pending" {
			t.Fatal("unknown exit reported finished")
		}
		mount(t, s, build().Disabled(true))
		if s.Focus("term/toggle") {
			t.Fatal("disabled header focused")
		}
		mount(t, s, build().ReadOnly(true))
		click(t, s, "term/toggle")
		if !open.Open {
			t.Fatal("read-only toggled")
		}
		if _, err := s.RenderScaled(2, nil); err != nil {
			t.Fatal(err)
		}
		s.Close()
	}
}

func TestTerminalScrollFollowAndCodeBlockBytes(t *testing.T) {
	s, err := NewSurface(300, 500, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	scroll := ScrollState{}
	out := TerminalOutput{Output: strings.Repeat("long output ", 60) + strings.Repeat("\nrow", 60)}
	build := func() *Element { return Terminal("term", out, &scroll, TerminalOptions{MaxHeight: 100}) }
	mount(t, s, build())
	x, y := scroll.Limits()
	if x <= 0 || y <= 0 || scroll.Y != y {
		t.Fatal("missing bidirectional scrolling or initial tail")
	}
	if err := s.ScrollTo("term/viewport", 0, 0); err != nil {
		t.Fatal(err)
	}
	out.Output += "\nappended"
	mount(t, s, build())
	if scroll.Y != 0 {
		t.Fatal("append stole reading position")
	}
	state := CodeBlockState{}
	copied := ""
	src := "\talpha\r\n\nlast\n"
	mount(t, s, CodeBlock("code", "Source", src, &state, 1, func(v string) { copied = v }))
	click(t, s, "code/copy")
	if copied != src {
		t.Fatal("collapsed copy lost source bytes")
	}
	click(t, s, "code/expand")
	mount(t, s, CodeBlock("code", "Source", src, &state, 1, nil))
	if !state.Expanded {
		t.Fatal("expand failed")
	}
	if _, err := s.Render(); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalHeaderAlignment(t *testing.T) {
	s, err := NewSurface(600, 420, NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state := DisclosureState{Open: true}
	scroll := ScrollState{}
	mount(t, s, Terminal("term", TerminalOutput{Command: "go test", Output: "ok"}, &scroll, TerminalOptions{Open: &state, OnCopy: func(string) {}}))
	for _, pair := range [][2]string{{"term/toggle", "term/status"}, {"term/title", "term/copy"}} {
		a, _ := s.Bounds(pair[0])
		b, _ := s.Bounds(pair[1])
		if delta := (a.Y + a.H/2) - (b.Y + b.H/2); delta < -0.5 || delta > 0.5 {
			t.Fatalf("unaligned row %v: %v / %v", pair, a, b)
		}
	}
}

func TestTerminalReferenceOutputFitsWithoutScrolling(t *testing.T) {
	for _, dark := range []bool{false, true} {
		s, err := NewSurface(640, 320, NewTheme(dark))
		if err != nil {
			t.Fatal(err)
		}
		state := DisclosureState{Open: true}
		scroll := ScrollState{}
		exit := 0
		mount(t, s, Terminal("term", TerminalOutput{Command: "go test ./...", Output: "ok  example/backend\nok  example/ui", ExitCode: &exit}, &scroll, TerminalOptions{Open: &state, OnCopy: func(string) {}}))
		if x, y := scroll.Limits(); x != 0 || y != 0 {
			t.Fatalf("short reference output unexpectedly scrolls: %g,%g", x, y)
		}
		root, _ := s.Bounds("term")
		body, _ := s.Bounds("term/output")
		if body.X-root.X != 24 {
			t.Fatal("trace indentation differs from reference")
		}
		text := s.entries[s.index["term/text"]]
		if text.bounds.H != 39 || intersect(text.bounds, text.clip) != text.bounds {
			t.Fatal("reference output clipped or wrong line spacing")
		}
		if s.entries[s.index["term/status"]].element.kind != TextKind {
			t.Fatal("status must be plain text")
		}
		for _, scale := range []float64{1, 1.25, 2} {
			if _, err := s.RenderScaled(scale, nil); err != nil {
				t.Fatal(err)
			}
		}
		s.Close()
	}
}
