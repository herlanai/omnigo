package css

import (
	"regexp"
	"strconv"
	"strings"
)

var mediaFeature = regexp.MustCompile(`\(\s*(?:(min-|max-)?(width|height)\s*:\s*([0-9.]+)(px|rem)|(?:width|height)\s*(?:<=|>=|<|>|=)\s*[0-9.]+(?:px|rem))\s*\)`)
var rangeFeature = regexp.MustCompile(`\(\s*(width|height)\s*(<=|>=|<|>|=)\s*([0-9.]+)(px|rem)\s*\)`)

func MediaMatches(query string, width, height int) bool { return mediaMatches(query, width, height) }
func mediaMatches(query string, width, height int) bool {
	for _, alternative := range Split(strings.ToLower(query), ',') {
		if strings.Contains(alternative, "print") || strings.Contains(alternative, "not ") {
			continue
		}
		matches := true
		rest := rangeFeature.ReplaceAllStringFunc(alternative, func(text string) string {
			m := rangeFeature.FindStringSubmatch(text)
			n, _ := strconv.ParseFloat(m[3], 64)
			if m[4] == "rem" {
				n *= 16
			}
			actual := float64(width)
			if m[1] == "height" {
				actual = float64(height)
			}
			switch m[2] {
			case "<=":
				matches = matches && actual <= n
			case ">=":
				matches = matches && actual >= n
			case "<":
				matches = matches && actual < n
			case ">":
				matches = matches && actual > n
			default:
				matches = matches && actual == n
			}
			return ""
		})
		rest = mediaFeature.ReplaceAllStringFunc(rest, func(text string) string {
			m := mediaFeature.FindStringSubmatch(text)
			n, _ := strconv.ParseFloat(m[3], 64)
			if m[4] == "rem" {
				n *= 16
			}
			actual := float64(width)
			if m[2] == "height" {
				actual = float64(height)
			}
			switch m[1] {
			case "min-":
				matches = matches && actual >= n
			case "max-":
				matches = matches && actual <= n
			default:
				matches = matches && actual == n
			}
			return ""
		})
		rest = strings.NewReplacer("(prefers-color-scheme: light)", "", "(prefers-color-scheme:light)", "", "(hover:hover)", "", "screen", "", "all", "", "and", "").Replace(rest)
		if strings.TrimSpace(rest) == "" && matches {
			return true
		}
	}
	return false
}
func stripComments(source string) string {
	var b strings.Builder
	var quote byte
	for i := 0; i < len(source); i++ {
		c := source[i]
		if c == '\\' && i+1 < len(source) {
			b.WriteByte(c)
			i++
			b.WriteByte(source[i])
			continue
		}
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(source) && source[i+1] == '*' {
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				break
			}
			i += end + 3
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
func resolveVariables(value string, values map[string]string) string {
	for depth := 0; depth < 16; depth++ {
		start := strings.Index(value, "var(")
		if start < 0 {
			return value
		}
		end, nesting := start+4, 1
		for ; end < len(value); end++ {
			if value[end] == '(' {
				nesting++
			}
			if value[end] == ')' {
				nesting--
				if nesting == 0 {
					break
				}
			}
		}
		if end >= len(value) {
			return value
		}
		parts := Split(value[start+4:end], ',')
		replacement, ok := values[strings.TrimSpace(parts[0])]
		if !ok && len(parts) > 1 {
			replacement = strings.TrimSpace(strings.Join(parts[1:], ","))
		}
		value = value[:start] + replacement + value[end+1:]
	}
	return ""
}
