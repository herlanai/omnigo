// Package markdown builds Go-native reading controls from a CommonMark/GFM AST.
// It does not generate HTML or execute scripts, and never fetches remote images.
package markdown

import (
	"errors"
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"html"
	"image/color"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

const MaxSourceBytes = 1 << 20

type Document struct {
	source []byte
	root   ast.Node
}

func Parse(source string) (*Document, error) {
	if len(source) > MaxSourceBytes {
		return nil, errors.New("markdown: source exceeds 1 MiB")
	}
	data := []byte(source)
	parser := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.Footnote))
	root := parser.Parser().Parse(text.NewReader(data))
	count, depth := 0, 0
	err := ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if enter {
			count++
			depth++
			if count > 20000 || depth > 64 {
				return ast.WalkStop, errors.New("markdown: document complexity exceeds limit")
			}
		} else {
			depth--
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	return &Document{data, root}, nil
}
func (d *Document) Source() string {
	if d == nil {
		return ""
	}
	return string(d.source)
}

// Options callbacks run synchronously on the UI goroutine. Link destinations
// are validated, but opening them is exclusively the application's decision.
type Options struct {
	OnLink       func(string)
	OnCopyCode   func(string)
	CodeBlock    func(id, language, source string) *ui.Element
	FontSize     float64
	LineHeight   float64
	BlockSpacing float64
	WrapCode     bool
	// Image resolves application-owned images synchronously without network I/O.
	// A nil result uses the accessible alt-text placeholder.
	Image func(id, destination, title, alt string) *ui.Element
}
type State struct {
	doc  *Document
	rich map[string]*ui.RichTextState
	code map[string]*ui.CodeBlockState
}

func (s *State) Reset() { s.doc = nil; s.rich = nil; s.code = nil }

// View reparses on changed source and reuses the AST for unchanged frames.
// Appending incomplete Markdown is supported, but this is not incremental parsing
// or the reference's animated/healed streaming implementation.
func (s *State) View(id, source string, o Options) (*ui.Element, error) {
	if s.doc == nil || string(s.doc.source) != source {
		doc, e := Parse(source)
		if e != nil {
			return nil, e
		}
		s.doc = doc
		s.rich = nil
	}
	return s.Build(id, s.doc, o), nil
}
func SafeLink(destination string) bool {
	for _, r := range destination {
		if unicode.IsControl(r) {
			return false
		}
	}
	u, e := url.Parse(destination)
	if e != nil || u.User != nil || destination == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	case "":
		return u.Host == ""
	default:
		return false
	}
}
func (s *State) Build(id string, d *Document, o Options) *ui.Element {
	if d == nil {
		panic("markdown: nil document")
	}
	if o.FontSize == 0 {
		o.FontSize = 14
	}
	if o.LineHeight == 0 {
		o.LineHeight = 1.5
	}
	if o.BlockSpacing == 0 {
		o.BlockSpacing = o.FontSize * .75
	}
	if s.rich == nil {
		s.rich = map[string]*ui.RichTextState{}
	}
	if s.code == nil {
		s.code = map[string]*ui.CodeBlockState{}
	}
	usedRich := map[string]*ui.RichTextState{}
	usedCode := map[string]*ui.CodeBlockState{}
	rich := func(key string, runs []ui.TextSpan, size float64, align ui.TextAlignment) *ui.Element {
		st := s.rich[key]
		if st == nil {
			st = &ui.RichTextState{}
		}
		usedRich[key] = st
		return ui.RichText(key, runs, ui.RichTextOptions{Size: size, State: st, OnLink: o.OnLink, LineHeight: o.LineHeight, Align: align})
	}
	paragraph := func(key string, n ast.Node, size float64, bold bool) *ui.Element {
		return rich(key, inline(n, d.source, ui.TextSpan{Bold: bold}), size, ui.TextAlignLeft)
	}
	var block func(ast.Node, string) *ui.Element
	var children func(ast.Node, string) []*ui.Element
	children = func(n ast.Node, key string) []*ui.Element {
		var result []*ui.Element
		index := 0
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			result = append(result, block(child, fmt.Sprintf("%s/%d", key, index)))
			index++
		}
		return result
	}
	code := func(key, language, source string) *ui.Element {
		if o.CodeBlock != nil {
			if custom := o.CodeBlock(key, language, source); custom != nil {
				return custom
			}
		}
		st := s.code[key]
		if st == nil {
			st = &ui.CodeBlockState{}
		}
		usedCode[key] = st
		st.Wrap = o.WrapCode
		st.Language = language
		title := language
		if title == "" {
			title = "Code"
		}
		return ui.CodeBlock(key, title, source, st, 0, o.OnCopyCode).Padding(o.FontSize * .85).Gap(8)
	}
	block = func(n ast.Node, key string) *ui.Element {
		switch v := n.(type) {
		case *ast.Heading:
			return paragraph(key, n, max(6, min(96, o.FontSize*[]float64{1, 1.6, 1.35, 1.15, 1, .95, .9}[v.Level])), true).Role("heading")
		case *ast.Paragraph, *ast.TextBlock:
			if img, ok := n.FirstChild().(*ast.Image); ok && img.NextSibling() == nil && o.Image != nil {
				destination := html.UnescapeString(string(img.Destination))
				if SafeLink(destination) && !strings.HasPrefix(strings.ToLower(destination), "mailto:") {
					var alt strings.Builder
					for _, run := range inline(img, d.source, ui.TextSpan{}) {
						alt.WriteString(run.Text)
					}
					if element := o.Image(key, destination, string(img.Title), alt.String()); element != nil {
						return element
					}
				}
			}
			return paragraph(key, n, o.FontSize, false)
		case *ast.FencedCodeBlock:
			return code(key, string(v.Language(d.source)), string(v.Lines().Value(d.source)))
		case *ast.CodeBlock:
			return code(key, "", string(v.Lines().Value(d.source)))
		case *ast.ThematicBreak:
			return ui.Separator(key)
		case *ast.Blockquote:
			if kind, runs, ok := alertContent(n, d.source); ok {
				parts := []*ui.Element{ui.Text(key+"/title", kind).FontWeight(600), rich(key+"/text", runs, o.FontSize, ui.TextAlignLeft)}
				for child, i := n.FirstChild().NextSibling(), 1; child != nil; child, i = child.NextSibling(), i+1 {
					parts = append(parts, block(child, fmt.Sprintf("%s/%d", key, i)))
				}
				return ui.Column(key, parts...).Padding(12).Gap(8).Role("note").PaintSurface(func(p *ui.Painter, r ui.Rect) {
					p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 3, H: r.H}, alertColor(kind, p.Theme()), 1)
				})
			}
			return ui.Column(key, children(n, key)...).Padding(12).PaintSurface(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, p.Theme().Muted, 6)
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 3, H: r.H}, p.Theme().Border, 1)
			})
		case *ast.List:
			var items []*ui.Element
			index := v.Start
			if !v.IsOrdered() {
				index = 1
			}
			i := 0
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				p := fmt.Sprintf("%s/%d", key, i)
				marker := "•"
				if v.IsOrdered() {
					marker = strconv.Itoa(index) + "."
				}
				mark := ui.Text(p+"/marker", marker).FontSize(o.FontSize).Width(o.FontSize * 1.6)
				if first := item.FirstChild(); first != nil {
					if task, ok := first.FirstChild().(*ext.TaskCheckBox); ok {
						mark = ui.Checkbox(p+"/marker", "", task.IsChecked, nil).Disabled(true).Width(o.FontSize * 1.6).Label("Task")
					}
				}
				items = append(items, ui.Row(p, mark, ui.Column(p+"/body", children(item, p+"/body")...).Gap(o.BlockSpacing).Grow(1)).Gap(6).Role("listitem"))
				index++
				i++
			}
			return ui.Column(key, items...).Gap(o.FontSize * .25).Role("list")
		case *ast.ListItem:
			return ui.Column(key, children(n, key)...)
		case *ext.Table:
			var rows []*ui.Element
			rowIndex := 0
			for row := n.FirstChild(); row != nil; row = row.NextSibling() {
				p := fmt.Sprintf("%s/%d", key, rowIndex)
				var cells []*ui.Element
				column := 0
				_, header := row.(*ext.TableHeader)
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					align := ui.TextAlignLeft
					if c, ok := cell.(*ext.TableCell); ok {
						if c.Alignment == ext.AlignCenter {
							align = ui.TextAlignCenter
						}
						if c.Alignment == ext.AlignRight {
							align = ui.TextAlignRight
						}
					}
					cellID := fmt.Sprintf("%s/%d", p, column)
					role := "cell"
					if header {
						role = "columnheader"
					}
					cells = append(cells, ui.Column(cellID, rich(cellID+"/text", inline(cell, d.source, ui.TextSpan{Bold: header}), o.FontSize, align)).Padding(o.FontSize*.6).Role(role))
					column++
				}
				line := ui.Grid(p, len(cells), 0, cells...).Role("row")
				if header {
					line.PaintSurface(func(p *ui.Painter, r ui.Rect) { p.Fill(r, p.Theme().Muted, 0) })
				}
				rows = append(rows, line, ui.Separator(p+"/rule"))
				rowIndex++
			}
			return ui.Column(key, rows...).Gap(0).Role("table")
		case *ast.HTMLBlock:
			source := string(v.Lines().Value(d.source))
			if v.HasClosure() {
				source += string(v.ClosureLine.Value(d.source))
			}
			return ui.RichText(key, []ui.TextSpan{{Text: source, Code: true}}, ui.RichTextOptions{Size: o.FontSize})
		case *ext.Footnote:
			return ui.Row(key, ui.Text(key+"/number", fmt.Sprintf("[%d]", v.Index)).Width(28), ui.Column(key+"/body", children(n, key+"/body")...).Grow(1))
		default:
			if n.HasChildren() {
				return ui.Column(key, children(n, key)...)
			}
			return ui.Text(key, string(n.Text(d.source))).Wrap(true)
		}
	}
	root := ui.Column(id, children(d.root, id)...).Gap(o.BlockSpacing).Role("document").Label("Markdown document")
	s.rich = usedRich
	s.code = usedCode
	return root
}
func inline(parent ast.Node, source []byte, style ui.TextSpan) []ui.TextSpan {
	var result []ui.TextSpan
	add := func(value string, s ui.TextSpan) {
		if value != "" {
			s.Text = value
			result = append(result, s)
		}
	}
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		s := style
		switch v := n.(type) {
		case *ast.Text:
			value := string(v.Value(source))
			if !s.Code {
				value = html.UnescapeString(string(util.UnescapePunctuations([]byte(value))))
			}
			add(value, s)
			if v.HardLineBreak() {
				add("\n", s)
			} else if v.SoftLineBreak() {
				add(" ", s)
			}
		case *ast.String:
			add(string(v.Value), s)
		case *ast.Emphasis:
			if v.Level == 2 {
				s.Bold = true
			} else {
				s.Italic = true
			}
			result = append(result, inline(n, source, s)...)
		case *ast.CodeSpan:
			s.Code = true
			parts := inline(n, source, s)
			for i := range parts {
				parts[i].Text = strings.ReplaceAll(parts[i].Text, "\n", " ")
			}
			result = append(result, parts...)
		case *ext.Strikethrough:
			s.Strike = true
			result = append(result, inline(n, source, s)...)
		case *ast.Link:
			destination := html.UnescapeString(string(v.Destination))
			if SafeLink(destination) {
				s.Link = destination
			}
			result = append(result, inline(n, source, s)...)
		case *ast.AutoLink:
			destination := string(v.URL(source))
			if v.AutoLinkType == ast.AutoLinkEmail {
				destination = "mailto:" + destination
			}
			if SafeLink(destination) {
				s.Link = destination
			}
			add(string(v.Label(source)), s)
		case *ast.Image:
			add("[Image: ", s)
			result = append(result, inline(n, source, s)...)
			add("]", s)
		case *ext.TaskCheckBox:
			// The containing list item supplies a read-only native checkbox.
		case *ext.FootnoteLink:
			add(fmt.Sprintf("[%d]", v.Index), s)
		case *ext.FootnoteBacklink:
		case *ast.RawHTML:
			add(string(v.Segments.Value(source)), s)
		default:
			result = append(result, inline(n, source, s)...)
		}
	}
	return result
}

// GitHub alert markers are recognized only at the beginning of a quote's
// first paragraph. Strip the marker from display spans, never from source.
func alertContent(n ast.Node, source []byte) (string, []ui.TextSpan, bool) {
	first, ok := n.FirstChild().(*ast.Paragraph)
	if !ok {
		return "", nil, false
	}
	runs := inline(first, source, ui.TextSpan{})
	if len(runs) == 0 {
		return "", nil, false
	}
	for _, kind := range []string{"Note", "Tip", "Important", "Warning", "Caution"} {
		marker := "[!" + strings.ToUpper(kind) + "]"
		// Goldmark can split the bracketed marker into multiple adjacent text nodes.
		var text strings.Builder
		for _, run := range runs {
			text.WriteString(run.Text)
		}
		all := text.String()
		if !strings.HasPrefix(all, marker) {
			continue
		}
		rest := strings.TrimPrefix(all, marker)
		if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\n") {
			continue
		}
		remaining := len(marker)
		for len(runs) > 0 && remaining > 0 {
			if len(runs[0].Text) <= remaining {
				remaining -= len(runs[0].Text)
				runs = runs[1:]
			} else {
				runs[0].Text = runs[0].Text[remaining:]
				remaining = 0
			}
		}
		for len(runs) > 0 {
			runs[0].Text = strings.TrimLeft(runs[0].Text, " \n")
			if runs[0].Text != "" {
				break
			}
			runs = runs[1:]
		}
		return kind, runs, true
	}
	return "", nil, false
}

func alertColor(kind string, theme ui.Theme) color.NRGBA {
	colors := map[string]color.NRGBA{
		"Note": {R: 37, G: 99, B: 235, A: 255}, "Tip": {R: 22, G: 128, B: 61, A: 255},
		"Important": {R: 126, G: 34, B: 206, A: 255}, "Warning": {R: 161, G: 98, B: 7, A: 255},
		"Caution": {R: 220, G: 38, B: 38, A: 255},
	}
	c := colors[kind]
	if theme.Background.R < 128 {
		c.R = uint8((uint16(c.R) + 255) / 2)
		c.G = uint8((uint16(c.G) + 255) / 2)
		c.B = uint8((uint16(c.B) + 255) / 2)
	}
	return c
}
