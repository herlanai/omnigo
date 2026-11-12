// Package css implements author-rule cascade for the documented CSS subset.
package css

import (
	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
	"strings"
)

type Declaration struct {
	Name, Value string
	Important   bool
}
type Rule struct {
	Pseudo       string
	Layer        int
	Selector     cascadia.Sel
	Declarations []Declaration
}
type Sheet struct {
	Keyframes   map[string][]Keyframe
	Rules       []Rule
	Diagnostics []string
}

// Split respects strings, escapes and nested function/attribute delimiters.
func Split(s string, sep byte) []string {
	var out []string
	start, depth := 0, 0
	var quote byte
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '(' || c == '[' || c == '{' {
			depth++
		}
		if c == ')' || c == ']' || c == '}' {
			depth--
		}
		if c == sep && depth == 0 {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
func Declarations(s string) []Declaration {
	var out []Declaration
	for _, part := range Split(s, ';') {
		kv := Split(part, ':')
		if len(kv) < 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		if !strings.HasPrefix(k, "--") {
			k = strings.ToLower(k)
		}
		v := strings.TrimSpace(strings.Join(kv[1:], ":"))
		important := strings.HasSuffix(strings.ToLower(v), "!important")
		if important {
			v = strings.TrimSpace(v[:len(v)-10])
		}
		if k != "" && v != "" {
			out = append(out, expandDeclaration(k, v, important)...)
		}
	}
	return out
}
func Document(root *html.Node) Sheet { return DocumentViewport(root, 1024, 768) }
func DocumentViewport(root *html.Node, width, height int) Sheet {
	var source strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "style" {
			var b strings.Builder
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				b.WriteString(c.Data)
			}
			source.WriteString(b.String())
			source.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return ParseViewport(source.String(), width, height)
}

type rank struct {
	important, inline bool
	layer             int
	specific          cascadia.Specificity
}

func (a rank) wins(b rank) bool {
	if a.important != b.important {
		return a.important
	}
	if a.inline != b.inline {
		return a.inline
	}
	if a.layer != b.layer {
		if a.important {
			if a.layer == 0 {
				return false
			}
			if b.layer == 0 {
				return true
			}
			return a.layer < b.layer
		}
		if a.layer == 0 {
			return true
		}
		if b.layer == 0 {
			return false
		}
		return a.layer > b.layer
	}
	return !a.specific.Less(b.specific)
}
func (s Sheet) Compute(n *html.Node, parent map[string]string) map[string]string {
	return s.compute(n, parent, "")
}
func (s Sheet) ComputePseudo(n *html.Node, parent map[string]string, kind string) map[string]string {
	return s.compute(n, parent, kind)
}
func (s Sheet) compute(n *html.Node, parent map[string]string, pseudo string) map[string]string {
	out := map[string]string{"display": "block", "color": "#202020", "font-size": "16px", "visibility": "visible", "background-color": "transparent"}
	if n.Data == "button" || n.Data == "input" {
		for _, a := range n.Attr {
			if a.Key == "type" && (a.Val == "submit" || a.Val == "button") {
				out["box-sizing"] = "border-box"
				out["text-align"] = "center"
			}
		}
	}
	if n.Data == "body" {
		out["margin"] = "8px"
	}
	for _, k := range []string{"color", "font-size", "font-family", "line-height", "visibility", "font-weight", "text-align", "white-space", "fill", "stroke", "stroke-width", "fill-opacity", "stroke-opacity", "text-anchor", "letter-spacing"} {
		if v := parent[k]; v != "" {
			out[k] = v
		}
	}
	if n.Data == "span" || n.Data == "a" || n.Data == "b" || n.Data == "strong" || n.Data == "em" || n.Data == "label" || n.Data == "i" || n.Data == "small" || n.Type == html.TextNode {
		out["display"] = "inline"
	}
	if n.Data == "head" || n.Data == "style" || n.Data == "script" || n.Data == "template" || n.Data == "title" || n.Data == "meta" || n.Data == "link" {
		out["display"] = "none"
	}
	for k, v := range parent {
		if strings.HasPrefix(k, "--") {
			out[k] = v
		}
	}
	if n.Data == "b" || n.Data == "strong" {
		out["font-weight"] = "bold"
	}
	if n.Data == "input" || n.Data == "button" || n.Data == "img" || n.Data == "select" || n.Data == "textarea" {
		out["display"] = "inline-block"
	}
	if n.Data == "input" {
		for _, a := range n.Attr {
			if a.Key == "type" && strings.EqualFold(a.Val, "hidden") {
				out["display"] = "none"
			}
		}
	}
	if n.Data == "input" {
		for _, a := range n.Attr {
			if a.Key == "type" && (a.Val == "submit" || a.Val == "button") {
				out["text-align"] = "center"
			}
		}
	}
	if pseudo != "" {
		out["display"] = "inline"
		delete(out, "margin")
	}
	ranks := map[string]rank{}
	apply := func(decls []Declaration, r rank) {
		for _, d := range decls {
			r.important = d.Important
			if old, ok := ranks[d.Name]; !ok || r.wins(old) {
				value := d.Value
				if value == "inherit" {
					value = parent[d.Name]
				}
				out[d.Name] = value
				ranks[d.Name] = r
			}
		}
	}
	if pseudo == "" && n.Namespace == "svg" {
		for _, a := range n.Attr {
			switch a.Key {
			case "fill", "stroke", "stroke-width", "fill-opacity", "stroke-opacity", "font-size", "font-weight", "text-anchor", "letter-spacing":
				apply([]Declaration{{Name: a.Key, Value: a.Val}}, rank{})
			}
		}
	}
	for _, rule := range s.Rules {
		if rule.Pseudo == pseudo && rule.Selector.Match(n) {
			specific := rule.Selector.Specificity()
			if pseudo != "" {
				specific[2]++
			}
			apply(rule.Declarations, rank{specific: specific, layer: rule.Layer})
		}
	}
	for _, a := range n.Attr {
		if pseudo == "" && a.Key == "style" {
			apply(Declarations(a.Val), rank{inline: true})
		}
		if a.Key == "hidden" {
			out["display"] = "none"
		}
	}
	for k, v := range out {
		if !strings.HasPrefix(k, "--") {
			out[k] = resolveVariables(v, out)
		}
	}
	if n.Data == "input" {
		for _, a := range n.Attr {
			if a.Key == "type" && strings.EqualFold(a.Val, "hidden") {
				out["display"] = "none"
			}
		}
	}
	return out
}
func (s Sheet) Styles(root *html.Node) map[*html.Node]map[string]string {
	result := map[*html.Node]map[string]string{}
	var walk func(*html.Node, map[string]string)
	walk = func(n *html.Node, parent map[string]string) {
		style := s.Compute(n, parent)
		result[n] = style
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, style)
		}
	}
	walk(root, nil)
	return result
}
