package css

import (
	"fmt"
	"github.com/andybalholm/cascadia"
	"strings"
)

func Parse(source string) Sheet { return ParseViewport(source, 1024, 768) }
func ParseViewport(source string, width, height int) Sheet {
	sheet := Sheet{Keyframes: map[string][]Keyframe{}}
	layers := map[string]int{}
	register := func(name string) int {
		if id := layers[name]; id != 0 {
			return id
		}
		id := len(layers) + 1
		layers[name] = id
		return id
	}
	var parse func(string, string, int)
	parse = func(source, layer string, level int) {
		if level > 32 {
			sheet.Diagnostics = append(sheet.Diagnostics, "CSS nesting limit")
			return
		}
		for len(strings.TrimSpace(source)) > 0 {
			end, delimiter := headerEnd(source)
			if end < 0 {
				break
			}
			head := strings.TrimSpace(source[:end])
			source = source[end+1:]
			if delimiter == ';' {
				if strings.HasPrefix(head, "@layer ") {
					for _, name := range Split(head[7:], ',') {
						register(strings.TrimSpace(name))
					}
				}
				continue
			}
			depth, close := 1, 0
			var quote byte
			for ; close < len(source) && depth > 0; close++ {
				c := source[close]
				if c == '\\' {
					close++
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
				if c == '{' {
					depth++
				}
				if c == '}' {
					depth--
				}
			}
			if depth != 0 {
				sheet.Diagnostics = append(sheet.Diagnostics, "Unclosed CSS rule")
				break
			}
			body := source[:close-1]
			source = source[close:]
			if strings.HasPrefix(head, "@keyframes ") {
				sheet.Keyframes[strings.TrimSpace(head[11:])] = parseKeyframes(body)
				continue
			}
			if strings.HasPrefix(head, "@layer") {
				name := strings.TrimSpace(head[6:])
				if name == "" {
					name = fmt.Sprintf("anonymous-%d", len(layers))
				}
				if layer != "" {
					name = layer + "." + name
				}
				register(name)
				parse(body, name, level+1)
				continue
			}
			if strings.HasPrefix(head, "@media") {
				if mediaMatches(strings.TrimSpace(head[6:]), width, height) {
					parse(body, layer, level+1)
				}
				continue
			}
			if strings.HasPrefix(head, "@supports") {
				condition := strings.ReplaceAll(strings.TrimSpace(head[9:]), " ", "")
				if condition == "(display:grid)" || condition == "(display:flex)" {
					parse(body, layer, level+1)
				} else {
					sheet.Diagnostics = append(sheet.Diagnostics, "Unsupported @supports condition: "+condition)
				}
				continue
			}
			if strings.HasPrefix(head, "@") {
				sheet.Diagnostics = append(sheet.Diagnostics, "Unsupported CSS at-rule: "+head)
				continue
			}
			layerID := 0
			if layer != "" {
				layerID = register(layer)
			}
			for _, part := range Split(head, ',') {
				part = strings.TrimSpace(part)
				pseudo := ""
				for _, suffix := range []string{"::before", "::after", ":before", ":after"} {
					if strings.HasSuffix(part, suffix) {
						pseudo = strings.TrimLeft(suffix, ":")
						part = strings.TrimSpace(strings.TrimSuffix(part, suffix))
						if part == "" {
							part = "*"
						}
						break
					}
				}
				selector, err := cascadia.Parse(part)
				if err != nil {
					sheet.Diagnostics = append(sheet.Diagnostics, err.Error())
					continue
				}
				sheet.Rules = append(sheet.Rules, Rule{Pseudo: pseudo, Selector: selector, Declarations: Declarations(body), Layer: layerID})
			}
		}
	}
	parse(stripComments(source), "", 0)
	return sheet
}
func headerEnd(source string) (int, byte) {
	var quote byte
	depth := 0
	for i := 0; i < len(source); i++ {
		c := source[i]
		if c == '\\' {
			i++
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
		if c == '(' || c == '[' {
			depth++
		}
		if c == ')' || c == ']' {
			depth--
		}
		if depth == 0 && (c == '{' || c == ';') {
			return i, c
		}
	}
	return -1, 0
}
