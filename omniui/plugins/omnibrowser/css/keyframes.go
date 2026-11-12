package css

import (
	"golang.org/x/net/html"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Keyframe struct {
	Offset       float64
	Declarations []Declaration
}

var frameRule = regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)

func parseKeyframes(body string) []Keyframe {
	var frames []Keyframe
	for _, m := range frameRule.FindAllStringSubmatch(body, 256) {
		for _, key := range strings.Split(m[1], ",") {
			key = strings.TrimSpace(key)
			offset := 0.0
			switch key {
			case "from":
				offset = 0
			case "to":
				offset = 1
			default:
				if !strings.HasSuffix(key, "%") {
					continue
				}
				v, e := strconv.ParseFloat(strings.TrimSuffix(key, "%"), 64)
				if e != nil || v < 0 || v > 100 {
					continue
				}
				offset = v / 100
			}
			var decls []Declaration
			for _, d := range Declarations(m[2]) {
				if !d.Important {
					decls = append(decls, d)
				}
			}
			frames = append(frames, Keyframe{offset, decls})
		}
	}
	sort.SliceStable(frames, func(i, j int) bool { return frames[i].Offset < frames[j].Offset })
	return frames
}

// Important author declarations outrank the animation origin.
func (s Sheet) Important(n *html.Node, property string) bool {
	for _, r := range s.Rules {
		if r.Pseudo == "" && r.Selector.Match(n) {
			for _, d := range r.Declarations {
				if d.Name == property && d.Important {
					return true
				}
			}
		}
	}
	for _, a := range n.Attr {
		if a.Key == "style" {
			for _, d := range Declarations(a.Val) {
				if d.Name == property && d.Important {
					return true
				}
			}
		}
	}
	return false
}
