package render

import (
	"golang.org/x/net/html"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strconv"
	"strings"
)

// Generated nodes live only in the private rendering tree, never the page DOM.
func generatePseudos(root *html.Node, sheet css.Sheet, styles map[*html.Node]map[string]string) {
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(nodes) >= 10000 {
			return
		}
		if n.Type == html.ElementNode {
			nodes = append(nodes, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	for _, n := range nodes {
		if n.Namespace == "svg" || styles[n]["display"] == "none" {
			continue
		}
		for _, kind := range []string{"before", "after"} {
			s := sheet.ComputePseudo(n, styles[n], kind)
			content, ok := generatedContent(s["content"], n)
			if !ok || s["display"] == "none" {
				continue
			}
			pseudo := &html.Node{Type: html.ElementNode, Data: "omni-generated-" + kind}
			styles[pseudo] = s
			if content != "" {
				text := &html.Node{Type: html.TextNode, Data: content}
				pseudo.AppendChild(text)
				ts := map[string]string{}
				for _, key := range []string{"color", "font-size", "font-weight", "line-height", "letter-spacing", "white-space"} {
					ts[key] = s[key]
				}
				ts["display"] = "inline"
				styles[text] = ts
			}
			// Make generated inline boxes participate in the normal box painter.
			if s["display"] == "inline" {
				s["display"] = "inline-block"
			}
			if kind == "before" {
				n.InsertBefore(pseudo, n.FirstChild)
			} else {
				n.AppendChild(pseudo)
			}
		}
	}
}
func generatedContent(value string, n *html.Node) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "normal" || value == "none" {
		return "", false
	}
	var out strings.Builder
	for len(value) > 0 {
		value = strings.TrimSpace(value)
		if value == "" {
			break
		}
		if strings.HasPrefix(value, "attr(") {
			i := strings.IndexByte(value, ')')
			if i < 0 {
				return "", false
			}
			out.WriteString(nodeAttr(n, strings.TrimSpace(value[5:i])))
			value = value[i+1:]
			continue
		}
		quote := value[0]
		if quote != '\'' && quote != '"' {
			return "", false
		}
		value = value[1:]
		closed := false
		for len(value) > 0 {
			c := value[0]
			value = value[1:]
			if c == quote {
				closed = true
				break
			}
			if c != '\\' {
				out.WriteByte(c)
				continue
			}
			if len(value) == 0 {
				return "", false
			}
			count := 0
			for count < len(value) && count < 6 && strings.ContainsRune("0123456789abcdefABCDEF", rune(value[count])) {
				count++
			}
			if count > 0 {
				v, _ := strconv.ParseInt(value[:count], 16, 32)
				if v == 0 || v > 0x10ffff {
					v = 0xfffd
				}
				out.WriteRune(rune(v))
				value = value[count:]
				if len(value) > 0 && strings.ContainsRune(" \t\n\r\f", rune(value[0])) {
					value = value[1:]
				}
			} else {
				out.WriteByte(value[0])
				value = value[1:]
			}
		}
		if !closed {
			return "", false
		}
		if out.Len() > 8192 {
			return "", false
		}
	}
	return out.String(), true
}
