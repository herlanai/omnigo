// Package i18n implements immutable catalogs and UI-owned language selection.
package i18n

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/currency"
	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

type segment struct {
	text, name, kind, style string
	branches                map[string][]segment
	offset                  float64
}
type parser struct {
	s          string
	pos, nodes int
}

func compile(s string) ([]segment, error) {
	if len(s) > 65536 || !utf8.ValidString(s) {
		return nil, fmt.Errorf("i18n: invalid or oversized message")
	}
	p := parser{s: s}
	return p.sequence(0, false)
}
func (p *parser) space() {
	for p.pos < len(p.s) && strings.ContainsRune(" \t\r\n", rune(p.s[p.pos])) {
		p.pos++
	}
}
func (p *parser) token() string {
	p.space()
	start := p.pos
	for p.pos < len(p.s) && !strings.ContainsRune("{}, \t\r\n", rune(p.s[p.pos])) {
		p.pos++
	}
	return p.s[start:p.pos]
}
func (p *parser) sequence(depth int, nested bool) ([]segment, error) {
	if depth > 32 {
		return nil, fmt.Errorf("i18n: message nesting exceeds limit")
	}
	var out []segment
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, segment{text: text.String()})
			text.Reset()
		}
	}
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		if c == '}' {
			if !nested {
				return nil, fmt.Errorf("i18n: unexpected closing brace")
			}
			p.pos++
			flush()
			return out, nil
		}
		if c == '\'' {
			if p.pos+1 < len(p.s) && p.s[p.pos+1] == '\'' {
				text.WriteByte('\'')
				p.pos += 2
				continue
			}
			if p.pos+1 < len(p.s) && strings.ContainsRune("{}#", rune(p.s[p.pos+1])) {
				p.pos++
				for p.pos < len(p.s) {
					if p.s[p.pos] == '\'' {
						p.pos++
						if p.pos < len(p.s) && p.s[p.pos] == '\'' {
							text.WriteByte('\'')
							p.pos++
							continue
						}
						break
					}
					text.WriteByte(p.s[p.pos])
					p.pos++
				}
				continue
			}
		}
		if c == '#' {
			flush()
			out = append(out, segment{kind: "hash"})
			p.pos++
			continue
		}
		if c != '{' {
			text.WriteByte(c)
			p.pos++
			continue
		}
		flush()
		p.nodes++
		if p.nodes > 4096 {
			return nil, fmt.Errorf("i18n: too many arguments")
		}
		p.pos++
		node := segment{name: p.token(), kind: "argument"}
		if node.name == "" {
			return nil, fmt.Errorf("i18n: empty argument")
		}
		p.space()
		if p.pos >= len(p.s) {
			return nil, fmt.Errorf("i18n: unclosed argument")
		}
		if p.s[p.pos] == '}' {
			p.pos++
			out = append(out, node)
			continue
		}
		if p.s[p.pos] != ',' {
			return nil, fmt.Errorf("i18n: expected argument comma")
		}
		p.pos++
		node.kind = p.token()
		p.space()
		switch node.kind {
		case "number":
			if p.pos < len(p.s) && p.s[p.pos] == ',' {
				p.pos++
				start := p.pos
				for p.pos < len(p.s) && p.s[p.pos] != '}' {
					p.pos++
				}
				node.style = strings.TrimSpace(p.s[start:p.pos])
			}
			if strings.HasPrefix(node.style, "::currency/") {
				if _, err := currency.ParseISO(strings.TrimPrefix(node.style, "::currency/")); err != nil {
					return nil, err
				}
			} else if node.style != "" && node.style != "integer" && node.style != "percent" && node.style != "::group-off" && node.style != "::.00" {
				return nil, fmt.Errorf("i18n: unsupported number style %q", node.style)
			}
			if p.pos >= len(p.s) || p.s[p.pos] != '}' {
				return nil, fmt.Errorf("i18n: unclosed number")
			}
			p.pos++
		case "select", "plural", "selectordinal":
			if p.pos >= len(p.s) || p.s[p.pos] != ',' {
				return nil, fmt.Errorf("i18n: expected selector comma")
			}
			p.pos++
			node.branches = map[string][]segment{}
			p.space()
			if node.kind != "select" && strings.HasPrefix(p.s[p.pos:], "offset:") {
				p.pos += 7
				v, err := strconv.ParseFloat(p.token(), 64)
				if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
					return nil, fmt.Errorf("i18n: invalid plural offset")
				}
				node.offset = v
			}
			for {
				p.space()
				if p.pos >= len(p.s) {
					return nil, fmt.Errorf("i18n: unclosed selector")
				}
				if p.s[p.pos] == '}' {
					p.pos++
					break
				}
				key := p.token()
				if key == "" {
					return nil, fmt.Errorf("i18n: empty selector")
				}
				if _, ok := node.branches[key]; ok {
					return nil, fmt.Errorf("i18n: duplicate selector %q", key)
				}
				if node.kind != "select" {
					if strings.HasPrefix(key, "=") {
						v, err := strconv.ParseFloat(key[1:], 64)
						if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
							return nil, fmt.Errorf("i18n: invalid exact selector")
						}
						key = "=" + strconv.FormatFloat(v, 'f', -1, 64)
					} else if key != "zero" && key != "one" && key != "two" && key != "few" && key != "many" && key != "other" {
						return nil, fmt.Errorf("i18n: invalid plural category")
					}
				}
				if _, exists := node.branches[key]; exists {
					return nil, fmt.Errorf("i18n: duplicate selector %q", key)
				}
				p.space()
				if p.pos >= len(p.s) || p.s[p.pos] != '{' {
					return nil, fmt.Errorf("i18n: expected selector body")
				}
				p.pos++
				body, err := p.sequence(depth+1, true)
				if err != nil {
					return nil, err
				}
				node.branches[key] = body
			}
			if _, ok := node.branches["other"]; !ok {
				return nil, fmt.Errorf("i18n: selector requires other")
			}
		default:
			return nil, fmt.Errorf("i18n: unsupported formatter %q", node.kind)
		}
		out = append(out, node)
	}
	if nested {
		return nil, fmt.Errorf("i18n: unclosed selector body")
	}
	flush()
	return out, nil
}
func numeric(v any) (float64, error) {
	var n float64
	switch v := v.(type) {
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case int32:
		n = float64(v)
	case uint:
		n = float64(v)
	case uint64:
		n = float64(v)
	case uint32:
		n = float64(v)
	case float64:
		n = v
	case float32:
		n = float64(v)
	default:
		return 0, fmt.Errorf("i18n: expected numeric argument, got %T", v)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 9007199254740991 {
		return 0, fmt.Errorf("i18n: number outside exact supported range")
	}
	return n, nil
}
func pluralCategory(tag language.Tag, n float64, ordinal bool) string {
	s := strconv.FormatFloat(math.Abs(n), 'f', -1, 64)
	point := strings.IndexByte(s, '.')
	if point < 0 {
		point = len(s)
	}
	var digits []byte
	for i := range s {
		if s[i] != '.' {
			digits = append(digits, s[i]-'0')
		}
	}
	scale := len(digits) - point
	rules := plural.Cardinal
	if ordinal {
		rules = plural.Ordinal
	}
	return []string{"other", "zero", "one", "two", "few", "many"}[rules.MatchDigits(tag, digits, point, scale)]
}
func numberText(tag language.Tag, n float64, style string) string {
	p := message.NewPrinter(tag)
	if strings.HasPrefix(style, "::currency/") {
		unit, _ := currency.ParseISO(strings.TrimPrefix(style, "::currency/"))
		return p.Sprintf("%v", currency.Symbol(unit.Amount(n)))
	}
	switch style {
	case "integer":
		return p.Sprintf("%v", number.Decimal(n, number.Scale(0)))
	case "percent":
		return p.Sprintf("%v", number.Percent(n))
	case "::group-off":
		return p.Sprintf("%v", number.Decimal(n, number.NoSeparator()))
	case "::.00":
		return p.Sprintf("%v", number.Decimal(n, number.Scale(2)))
	}
	return p.Sprintf("%v", number.Decimal(n))
}
func render(nodes []segment, tag language.Tag, args map[string]any, hash *float64, out *strings.Builder) error {
	for _, node := range nodes {
		if out.Len() > 1<<20 {
			return fmt.Errorf("i18n: output budget exceeded")
		}
		if node.kind == "" {
			out.WriteString(node.text)
			continue
		}
		if node.kind == "hash" {
			if hash == nil {
				out.WriteByte('#')
			} else {
				out.WriteString(numberText(tag, *hash, ""))
			}
			continue
		}
		value, ok := args[node.name]
		if !ok {
			return fmt.Errorf("i18n: missing argument %q", node.name)
		}
		switch node.kind {
		case "argument":
			switch v := value.(type) {
			case string:
				if len(v) > (1<<20)-out.Len() {
					return fmt.Errorf("i18n: output budget exceeded")
				}
				out.WriteString(v)
			case bool:
				out.WriteString(strconv.FormatBool(v))
			default:
				n, err := numeric(value)
				if err != nil {
					return err
				}
				out.WriteString(numberText(tag, n, ""))
			}
		case "number":
			n, err := numeric(value)
			if err != nil {
				return err
			}
			out.WriteString(numberText(tag, n, node.style))
		case "select":
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("i18n: select needs string %q", node.name)
			}
			branch, ok := node.branches[v]
			if !ok {
				branch = node.branches["other"]
			}
			if err := render(branch, tag, args, hash, out); err != nil {
				return err
			}
		case "plural", "selectordinal":
			n, err := numeric(value)
			if err != nil {
				return err
			}
			adjusted := n - node.offset
			branch, ok := node.branches["="+strconv.FormatFloat(n, 'f', -1, 64)]
			if !ok {
				branch, ok = node.branches[pluralCategory(tag, adjusted, node.kind == "selectordinal")]
				if !ok {
					branch = node.branches["other"]
				}
			}
			if err := render(branch, tag, args, &adjusted, out); err != nil {
				return err
			}
		}
	}
	if out.Len() > 1<<20 {
		return fmt.Errorf("i18n: output budget exceeded")
	}
	return nil
}
