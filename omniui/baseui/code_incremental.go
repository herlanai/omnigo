package baseui

import (
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"
)

type lexicalState uint8

const (
	lexNormal lexicalState = iota
	lexComment
	lexRaw
)

// A line checkpoint carries only Go's multiline lexical modes. Invalid quoted
// strings stop at the line boundary, exactly as Go's scanner does.
func scanCodeLine(line string, before lexicalState, language string) ([]codeRun, lexicalState) {
	if language != "go" {
		return []codeRun{{line, 0}}, lexNormal
	}
	prefix := ""
	if before == lexComment {
		prefix = "/*"
	}
	if before == lexRaw {
		prefix = "`"
	}
	// Reused runs must not pin an entire obsolete document string after an edit.
	source := prefix + strings.Clone(line)
	styles := make([]int, len(source))
	var scan scanner.Scanner
	file := token.NewFileSet().AddFile("line.go", -1, len(source))
	scan.Init(file, []byte(source), func(token.Position, string) {}, scanner.ScanComments)
	after := lexNormal
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
		case tok == token.INT || tok == token.FLOAT || tok == token.IMAG:
			style = 4
		}
		start := file.Offset(pos)
		length := len(lit)
		if length == 0 {
			length = len(tok.String())
		}
		// Scanner removes CR from comments/raw literals. Find their original extent
		// instead of using the normalized token length for painting.
		if tok == token.COMMENT && strings.HasPrefix(source[start:], "/*") {
			end := strings.Index(source[start+2:], "*/")
			if end < 0 {
				length = len(source) - start
				after = lexComment
			} else {
				length = end + 4
			}
		}
		if tok == token.COMMENT && strings.HasPrefix(source[start:], "//") {
			length = len(source) - start
		}
		if tok == token.STRING && strings.HasPrefix(source[start:], "`") {
			end := strings.IndexByte(source[start+1:], '`')
			if end < 0 {
				length = len(source) - start
				after = lexRaw
			} else {
				length = end + 2
			}
		}
		if style != 0 {
			for j := start; j < min(len(styles), start+length); j++ {
				styles[j] = style
			}
		}
	}
	runs := []codeRun{}
	for at := len(prefix); at < len(source); {
		end := at + 1
		for end < len(source) && styles[end] == styles[at] {
			end++
		}
		runs = append(runs, codeRun{source[at:end], styles[at]})
		at = end
	}
	return runs, after
}

func incrementalCode(old codeEditorCache, text, language string) codeEditorCache {
	lines := strings.Split(text, "\n")
	next := codeEditorCache{valid: true, language: language, lineCount: len(lines), lines: lines, rows: make([][]codeRun, len(lines)), states: make([]lexicalState, len(lines)), widths: make([]int, len(lines)), width: 400}
	prefix, suffix := 0, 0
	if old.valid && old.language == language {
		for prefix < min(len(lines), len(old.lines)) && lines[prefix] == old.lines[prefix] {
			prefix++
		}
		for suffix < min(len(lines), len(old.lines))-prefix && lines[len(lines)-1-suffix] == old.lines[len(old.lines)-1-suffix] {
			suffix++
		}
		copy(next.rows, old.rows[:prefix])
		copy(next.states, old.states[:prefix])
		copy(next.widths, old.widths[:prefix])
	}
	state := lexNormal
	if prefix > 0 {
		state = next.states[prefix-1]
	}
	for i := prefix; i < len(lines); i++ {
		oldIndex := len(old.lines) - (len(lines) - i)
		if i >= len(lines)-suffix && oldIndex >= 0 {
			oldBefore := lexNormal
			if oldIndex > 0 {
				oldBefore = old.states[oldIndex-1]
			}
			if state == oldBefore {
				copy(next.rows[i:], old.rows[oldIndex:])
				copy(next.states[i:], old.states[oldIndex:])
				copy(next.widths[i:], old.widths[oldIndex:])
				break
			}
		}
		next.rows[i], state = scanCodeLine(lines[i], state, language)
		next.states[i] = state
		next.widths[i] = utf8.RuneCountInString(lines[i])
		next.lexed++
	}
	for _, width := range next.widths {
		next.width = max(next.width, float64(width)*9+64)
	}
	return next
}
