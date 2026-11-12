package icon

import (
	"fmt"
	"regexp"
	"strings"
)

var pathNumber = regexp.MustCompile(`^[+-]?(?:\d*\.\d+|\d+\.?\d*)(?:[eE][+-]?\d+)?`)

// SVG permits adjacent one-character arc flags (e.g. a10 10 0 0010-10).
// Expand them before passing paths to the renderer's older numeric tokenizer.
func normalizePath(source string) (string, error) {
	var out strings.Builder
	command := byte(0)
	count := 0
	arity := 0
	seen := false
	complete := func() bool { return arity == 0 || count > 0 && count%arity == 0 }
	for i := 0; i < len(source); {
		c := source[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' {
			i++
			continue
		}
		if strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(c)) {
			if !complete() {
				return "", fmt.Errorf("icon: incomplete path command")
			}
			if !seen && c != 'M' && c != 'm' {
				return "", fmt.Errorf("icon: path must start with move")
			}
			seen = true
			command = c
			count = 0
			switch c {
			case 'M', 'm', 'L', 'l', 'T', 't':
				arity = 2
			case 'H', 'h', 'V', 'v':
				arity = 1
			case 'C', 'c':
				arity = 6
			case 'S', 's', 'Q', 'q':
				arity = 4
			case 'A', 'a':
				arity = 7
			case 'Z', 'z':
				arity = 0
			}
			out.WriteByte(c)
			out.WriteByte(' ')
			i++
			continue
		}
		if arity == 0 {
			return "", fmt.Errorf("icon: path number without command")
		}
		token := ""
		if (command == 'A' || command == 'a') && (count%7 == 3 || count%7 == 4) {
			if c != '0' && c != '1' {
				return "", fmt.Errorf("icon: invalid arc flag")
			}
			token = source[i : i+1]
		} else {
			token = pathNumber.FindString(source[i:])
			if token == "" {
				return "", fmt.Errorf("icon: invalid path number")
			}
		}
		out.WriteString(token)
		out.WriteByte(' ')
		i += len(token)
		count++
	}
	if !seen || !complete() {
		return "", fmt.Errorf("icon: incomplete path")
	}
	return out.String(), nil
}
