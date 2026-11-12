package baseui

import "strings"

// Wrapping keeps explicit newlines and all non-boundary whitespace. Long tokens
// break at rune boundaries; grapheme shaping and Unicode line breaking are pending.
func (s *Surface) textLines(e *Element, width float64) []string {
	lines := strings.Split(e.text, "\n")
	if !e.wrap || width <= 0 {
		return lines
	}
	out := []string{}
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		remaining := []rune(line)
		for len(remaining) > 0 {
			lo, hi := 1, len(remaining)
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if s.textWidth(string(remaining[:mid])) <= width {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			n := lo
			if n < len(remaining) {
				for i := n - 1; i > 0; i-- {
					if remaining[i] == ' ' {
						n = i + 1
						break
					}
				}
			}
			out = append(out, string(remaining[:n]))
			remaining = remaining[n:]
		}
	}
	return out
}

// fitLabel is used by single-line secondary option text; it never slices UTF-8
// in the middle of a rune. Full grapheme-aware truncation remains separate.
func fitLabel(text string, width float64, measure func(string) float64) string {
	if measure(text) <= width {
		return text
	}
	suffix := "..."
	if measure(suffix) > width {
		return ""
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure(string(runes[:mid])+suffix) <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + suffix
}
