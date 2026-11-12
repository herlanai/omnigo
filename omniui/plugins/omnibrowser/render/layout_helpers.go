package render

import (
	"golang.org/x/net/html"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

func edges(s map[string]string, key string, relative float64) [4]int {
	parts := strings.Fields(s[key])
	var result [4]int
	if len(parts) > 0 && len(parts) <= 4 {
		v := make([]int, len(parts))
		for i, p := range parts {
			v[i] = int(length(p, relative, 0))
		}
		switch len(v) {
		case 1:
			result = [4]int{v[0], v[0], v[0], v[0]}
		case 2:
			result = [4]int{v[0], v[1], v[0], v[1]}
		case 3:
			result = [4]int{v[0], v[1], v[2], v[1]}
		case 4:
			copy(result[:], v)
		}
	}
	for i, side := range []string{"top", "right", "bottom", "left"} {
		if value := s[key+"-"+side]; value != "" {
			result[i] = int(length(value, relative, 0))
		}
	}
	return result
}
func columns(value string) int {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "repeat(") {
		n, _ := strconv.Atoi(strings.TrimSpace(strings.Split(strings.TrimPrefix(value, "repeat("), ",")[0]))
		return max(1, min(24, n))
	}
	return max(1, min(24, len(strings.Fields(value))))
}

// Break CJK at character boundaries; keep Latin word boundaries where possible.
func textTokens(text string) []string {
	var out []string
	var word strings.Builder
	space := false
	flush := func() {
		if word.Len() > 0 {
			out = append(out, word.String())
			word.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsSpace(r) {
			flush()
			space = true
			continue
		}
		if space {
			out = append(out, " ")
			space = false
		}
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			flush()
			out = append(out, string(r))
		} else {
			word.WriteRune(r)
		}
	}
	flush()
	return out
}

func viewportLength(v string, relative, fallback float64, width, height int) float64 {
	return cssMath(v, relative, fallback, width, height, 0)
}

var translatePattern = regexp.MustCompile(`translate(X|Y)?\(([^)]+)\)`)

func translation(v string, w, h int) (int, int) {
	x, y := 0, 0
	signed := func(v string, relative int) int {
		n, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(v), "%"), "px"), 64)
		if err != nil {
			return 0
		}
		if strings.Contains(v, "%") {
			n *= float64(relative) / 100
		}
		return int(n)
	}
	for _, m := range translatePattern.FindAllStringSubmatch(v, -1) {
		parts := strings.Split(m[2], ",")
		if m[1] == "Y" {
			y += signed(parts[0], h)
		} else {
			x += signed(parts[0], w)
			if len(parts) > 1 {
				y += signed(parts[1], h)
			}
		}
	}
	return x, y
}

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func nodeText(n *html.Node) string {
	if n.Type == html.TextNode {
		return strings.Join(strings.Fields(n.Data), " ")
	}
	if n.Data == "style" || n.Data == "script" {
		return ""
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(nodeText(c))
	}
	return b.String()
}
func isBold(s map[string]string) bool {
	if s["font-weight"] == "bold" || s["font-weight"] == "bolder" {
		return true
	}
	n, _ := strconv.Atoi(s["font-weight"])
	return n >= 600
}

func cornerRadii(value string, relative float64) [4]float64 {
	parts := strings.Fields(value)
	var r [4]float64
	if len(parts) < 1 || len(parts) > 4 {
		return r
	}
	v := make([]float64, len(parts))
	for i, p := range parts {
		v[i] = length(p, relative, 0)
	}
	switch len(v) {
	case 1:
		r = [4]float64{v[0], v[0], v[0], v[0]}
	case 2:
		r = [4]float64{v[0], v[1], v[0], v[1]}
	case 3:
		r = [4]float64{v[0], v[1], v[2], v[1]}
	case 4:
		copy(r[:], v)
	}
	return r
}

// Grid's common fixed/fraction tracks, with a bounded equal-track fallback for repeat/minmax.
func gridTracks(value string, available, gap int) []int {
	count := columns(value)
	parts := strings.Fields(value)
	weights := make([]float64, count)
	out := make([]int, count)
	remaining := max(0, available-gap*(count-1))
	total := 0.0
	for i := range out {
		v := "1fr"
		if len(parts) == count && !strings.HasPrefix(value, "repeat(") {
			v = parts[i]
		}
		if strings.HasSuffix(v, "fr") {
			weights[i], _ = strconv.ParseFloat(strings.TrimSuffix(v, "fr"), 64)
			weights[i] = max(.001, weights[i])
			total += weights[i]
		} else {
			out[i] = int(length(v, float64(available), float64(remaining/count)))
			remaining -= out[i]
		}
	}
	for i := range out {
		if weights[i] > 0 {
			out[i] = int(float64(max(0, remaining)) * weights[i] / total)
		}
	}
	return out
}

func aspectHeight(value string, w int) int {
	parts := strings.Split(value, "/")
	a, e := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if e != nil || a <= 0 {
		return 0
	}
	b := 1.0
	if len(parts) == 2 {
		b, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	}
	return int(max(0, min(16384, float64(w)*b/a)))
}
