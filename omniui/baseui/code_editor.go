package baseui

import (
	"fmt"
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"
)

type CodeEditorState struct {
	Text   *TextBuffer
	Scroll ScrollState
	Find   FindBarState
}
type CodeEditorOptions struct {
	Height   float64
	Language string
	OnChange func(string)
}
type codeRun struct {
	text  string
	style int
}

func codeLines(source, language string) [][]codeRun {
	lines := strings.Split(source, "\n")
	out := make([][]codeRun, len(lines))
	if language != "go" {
		for i, line := range lines {
			out[i] = []codeRun{{line, 0}}
		}
		return out
	}
	styles := make([]int, len(source))
	var scan scanner.Scanner
	file := token.NewFileSet().AddFile("editor.go", -1, len(source))
	scan.Init(file, []byte(source), nil, scanner.ScanComments)
	for {
		pos, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		style := 0
		switch {
		case tok.IsKeyword():
			style = 1
		case tok == token.STRING || tok == token.CHAR:
			style = 2
		case tok == token.COMMENT:
			style = 3
		case tok == token.INT || tok == token.FLOAT:
			style = 4
		}
		if style == 0 {
			continue
		}
		start := file.Offset(pos)
		length := len(lit)
		if length == 0 {
			length = len(tok.String())
		}
		for j := start; j < min(len(styles), start+length); j++ {
			styles[j] = style
		}
	}
	offset := 0
	for i, line := range lines {
		for at := 0; at < len(line); {
			style := styles[offset+at]
			end := at + 1
			for end < len(line) && styles[offset+end] == style {
				end++
			}
			out[i] = append(out[i], codeRun{line[at:end], style})
			at = end
		}
		offset += len(line) + 1
	}
	return out
}
func (s *CodeEditorState) FindNext(query string, backward bool) bool {
	if s.Text == nil || query == "" {
		return false
	}
	r := []rune(s.Text.String())
	q := []rune(query)
	a, c := s.Text.Selection()
	start := max(a, c)
	direction := 1
	if backward {
		start = min(a, c) - 1
		direction = -1
	}
	for n := 0; n <= len(r); n++ {
		i := (start + n*direction) % (len(r) + 1)
		if i < 0 {
			i += len(r) + 1
		}
		if i+len(q) <= len(r) && string(r[i:i+len(q)]) == query {
			s.Text.Select(i, i+len(q))
			return true
		}
	}
	return false
}
func CodeEditor(id string, state *CodeEditorState, o CodeEditorOptions) *Element {
	if state == nil {
		panic("baseui: nil code editor")
	}
	if state.Text == nil {
		state.Text = NewTextBuffer("")
	}
	if o.Height == 0 {
		o.Height = 280
	}
	if !finite(o.Height) || o.Height <= 0 {
		panic("baseui: invalid editor height")
	}
	text := state.Text.String()
	lines := strings.Split(text, "\n")
	edit := Textarea(id+"/input", "Code editor", state.Text).FontSize(14)
	edit.monospace = true
	edit.codeRows = codeLines(text, o.Language)
	edit.captureTab = true
	face := richFace(4, 14)
	lineHeight := float64(face.Metrics().Height.Ceil()) + 4
	face.Close()
	edit.Height(float64(len(lines))*lineHeight + 20).MinHeight(o.Height - 10)
	width := 400.0
	for _, line := range lines {
		width = max(width, float64(utf8.RuneCountInString(line))*9+64)
	}
	edit.Width(width)
	edit.onChange = o.OnChange
	insert := func(value string) {
		before := state.Text.String()
		state.Text.Replace(value)
		if before != state.Text.String() && o.OnChange != nil {
			o.OnChange(state.Text.String())
		}
	}
	edit.onKey = func(ev KeyEvent) bool {
		if state.Text.Composition() != "" {
			return false
		}
		a, c := state.Text.Selection()
		r := []rune(state.Text.String())
		start := c
		for start > 0 && r[start-1] != '\n' {
			start--
		}
		end := c
		for end < len(r) && r[end] != '\n' {
			end++
		}
		switch {
		case ev.Command && ev.Key == "F":
			state.Find.Open = true
			ev.RequestFocus(id + "/find/query")
			return true
		case ev.Key == "Tab":
			if !ev.Shift {
				insert("    ")
			} else {
				n := 0
				for start+n < len(r) && n < 4 && r[start+n] == ' ' {
					n++
				}
				state.Text.Select(start, start+n)
				insert("")
			}
			return true
		case ev.Key == "Enter":
			indent := start
			for indent < len(r) && (r[indent] == ' ' || r[indent] == '\t') {
				indent++
			}
			insert("\n" + string(r[start:indent]))
			return true
		case ev.Key == "Home", ev.Key == "End", ev.Key == "Up", ev.Key == "Down":
			target := start
			if ev.Key == "End" {
				target = end
			}
			if ev.Key == "Up" {
				previous := max(0, start-1)
				begin := previous
				for begin > 0 && r[begin-1] != '\n' {
					begin--
				}
				target = min(previous, begin+c-start)
			}
			if ev.Key == "Down" {
				begin := min(len(r), end+1)
				last := begin
				for last < len(r) && r[last] != '\n' {
					last++
				}
				target = min(last, begin+c-start)
			}
			if !ev.Shift {
				a = target
			}
			state.Text.Select(a, target)
			return true
		}
		return false
	}
	matches := 0
	if state.Find.Query != nil && state.Find.Query.String() != "" {
		matches = strings.Count(text, state.Find.Query.String())
	}
	find := FindBar(id+"/find", &state.Find, FindBarOptions{Matches: matches, OnQuery: func(q string) { state.FindNext(q, false) }, OnSelect: func(int) {
		if state.Find.Query != nil {
			state.FindNext(state.Find.Query.String(), false)
		}
	}})
	return Column(id, find, ScrollArea(id+"/scroll", &state.Scroll, edit).Height(o.Height).HorizontalScroll(true), Text(id+"/status", fmt.Sprintf("%d lines · %s · Ctrl/Cmd+F find · Ctrl/Cmd+Tab leave editor", len(lines), o.Language)).FontSize(12).Muted(true)).Gap(6)
}
