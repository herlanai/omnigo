// Package markdown builds Go-native reading controls from a CommonMark/GFM AST.
// It does not generate HTML or execute scripts, and never fetches remote images.
package markdown

import (
	"errors"
	"fmt"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"html"
	"net/url"
	ui "omnigo.dev/omnigo/omniui/baseui"
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
	OnLink     func(string)
	OnCopyCode func(string)
	CodeBlock  func(id, language, source string) *ui.Element
	FontSize   float64
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
	if s.rich == nil {
		s.rich = map[string]*ui.RichTextState{}
	}
	if s.code == nil {
		s.code = map[string]*ui.CodeBlockState{}
	}
	usedRich := map[string]*ui.RichTextState{}
	usedCode := map[string]*ui.CodeBlockState{}
	paragraph := func(key string, n ast.Node, size float64, bold bool) *ui.Element {
		st := s.rich[key]
		if st == nil {
			st = &ui.RichTextState{}
		}
		usedRich[key] = st
		runs := inline(n, d.source, ui.TextSpan{Bold: bold})
		return ui.RichText(key, runs, ui.RichTextOptions{Size: size, State: st, OnLink: o.OnLink})
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
		title := language
		if title == "" {
			title = "Code"
		}
		return ui.CodeBlock(key, title, source, st, 0, o.OnCopyCode)
	}
	block = func(n ast.Node, key string) *ui.Element {
		switch v := n.(type) {
		case *ast.Heading:
			return paragraph(key, n, o.FontSize*[]float64{1, 2, 1.65, 1.4, 1.2, 1.1, 1}[v.Level], true).Role("heading")
		case *ast.Paragraph, *ast.TextBlock:
			return paragraph(key, n, o.FontSize, false)
		case *ast.FencedCodeBlock:
			return code(key, string(v.Language(d.source)), string(v.Lines().Value(d.source)))
		case *ast.CodeBlock:
			return code(key, "", string(v.Lines().Value(d.source)))
		case *ast.ThematicBreak:
			return ui.Separator(key)
		case *ast.Blockquote:
			return ui.Column(key, children(n, key)...).Padding(12).PaintSurface(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, p.Theme().Muted, 6)
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 3, H: r.H}, p.Theme().Border, 1)
			})
		case *ast.List:
			var items []*ui.Element
			index := v.Start
			if index < 1 {
				index = 1
			}
			i := 0
			for item := n.FirstChild(); item != nil; item = item.NextSibling() {
				p := fmt.Sprintf("%s/%d", key, i)
				marker := "•"
				if v.IsOrdered() {
					marker = strconv.Itoa(index) + "."
				}
				items = append(items, ui.Row(p, ui.Text(p+"/marker", marker).Width(24), ui.Column(p+"/body", children(item, p+"/body")...).Grow(1)).Gap(6))
				index++
				i++
			}
			return ui.Column(key, items...).Gap(6).Role("list")
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
					cells = append(cells, paragraph(fmt.Sprintf("%s/%d", p, column), cell, o.FontSize, header).Grow(1))
					column++
				}
				rows = append(rows, ui.Row(p, cells...).Padding(8).Role("row"), ui.Separator(p+"/rule"))
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
	root := ui.Column(id, children(d.root, id)...).Gap(12).Role("document").Label("Markdown document")
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
			if v.IsChecked {
				add("[x] ", s)
			} else {
				add("[ ] ", s)
			}
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
