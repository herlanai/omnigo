package render

import (
	"golang.org/x/image/font"
	"golang.org/x/net/html"
	"image"
	"image/color"
	"strings"
)

// Inline formatting shares a line across adjacent text and inline elements.
// Replaced elements/inline blocks remain separate boxes within that line.
func inlineFormat(nodes []*html.Node, x, y, w, depth int, parent map[string]string, styles map[*html.Node]map[string]string, boxes *[]paintBox, face func(float64, bool) font.Face, layout func(*html.Node, int, int, int, int) int, images map[string]image.Image, base string) int {
	cursor, used, lineHeight, start := 0, 0, 0, len(*boxes)
	flush := func() {
		if len(*boxes) == start {
			return
		}
		offset := 0
		switch parent["text-align"] {
		case "center":
			offset = max(0, (w-cursor)/2)
		case "right", "end":
			offset = max(0, w-cursor)
		}
		for i := start; i < len(*boxes); i++ {
			(*boxes)[i].rect = (*boxes)[i].rect.Add(image.Pt(offset, 0))
		}
		used += lineHeight
		cursor, lineHeight = 0, 0
		start = len(*boxes)
	}
	var walk func(*html.Node, int)
	walk = func(n *html.Node, d int) {
		if d > 128 {
			return
		}
		s := styles[n]
		if s["display"] == "none" {
			return
		}
		if n.Data == "br" {
			flush()
			return
		}
		size := max(8, min(96, length(s["font-size"], 16, 16)))
		f := face(size, isBold(s))
		lh := max(int(size*1.4), f.Metrics().Height.Ceil())
		if v := s["line-height"]; v != "" && v != "normal" {
			lh = int(length(v, size, float64(lh)))
			if !strings.ContainsAny(v, "%pxrem") {
				lh = int(length(v, 1, 1.4) * size)
			}
		}
		if n.Type == html.TextNode {
			for _, token := range textTokens(n.Data) {
				if token == " " && cursor == 0 {
					continue
				}
				advance := font.MeasureString(f, token).Ceil()
				if cursor > 0 && cursor+advance > w {
					flush()
					if token == " " {
						continue
					}
				}
				*boxes = append(*boxes, paintBox{rect: image.Rect(x+cursor, y+used, x+cursor+advance, y+used+lh), text: token, fg: paintColor(s["color"], color.NRGBA{32, 32, 32, 255}), size: size, bold: isBold(s)})
				cursor += advance
				lineHeight = max(lineHeight, lh)
			}
			return
		}
		if s["display"] == "inline" {
			margin, padding := edges(s, "margin", float64(w)), edges(s, "padding", float64(w))
			cursor += margin[3] + padding[3]
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, d+1)
			}
			cursor += margin[1] + padding[1]
			return
		}
		outerMargin := edges(s, "margin", float64(w))
		natural := max(16, intrinsicWidth(n, styles, face, images, base, 0)-outerMargin[1]-outerMargin[3])
		if n.Data == "input" {
			natural = 160
			if nodeAttr(n, "type") == "submit" || nodeAttr(n, "type") == "button" {
				natural = font.MeasureString(f, nodeAttr(n, "value")).Ceil() + 20
			}
		}
		if img := images[imageAddress(base, nodeAttr(n, "src"))]; n.Data == "img" && img != nil {
			natural = img.Bounds().Dx()
		}
		widthValue := s["width"]
		if widthValue == "" {
			widthValue = nodeAttr(n, "width")
		}
		cw := int(length(widthValue, float64(w), float64(natural)))
		pad := edges(s, "padding", float64(w))
		margin := edges(s, "margin", float64(w))
		if s["box-sizing"] != "border-box" && widthValue != "" {
			cw += pad[1] + pad[3]
		}
		cw += margin[1] + margin[3]
		if cursor > 0 && cursor+cw > w {
			flush()
		}
		h := layout(n, x+cursor, y+used, cw, d+1)
		cursor += cw
		lineHeight = max(lineHeight, h)
	}
	for _, n := range nodes {
		walk(n, depth)
	}
	flush()
	return used
}
func inlineLevel(n *html.Node, s map[string]string) bool {
	return n.Type == html.TextNode || n.Data == "br" || s["display"] == "inline" || s["display"] == "inline-block" || s["display"] == "inline-flex" || s["float"] == "left" || s["float"] == "right"
}

func intrinsicWidth(n *html.Node, styles map[*html.Node]map[string]string, face func(float64, bool) font.Face, images map[string]image.Image, base string, depth int) int {
	if depth > 128 {
		return 0
	}
	s := styles[n]
	if s["display"] == "none" {
		return 0
	}
	size := max(8, min(96, length(s["font-size"], 16, 16)))
	if n.Type == html.TextNode {
		return font.MeasureString(face(size, isBold(s)), strings.Join(strings.Fields(n.Data), " ")).Ceil()
	}
	padding := edges(s, "padding", 0)
	margin := edges(s, "margin", 0)
	if v := s["width"]; v != "" && v != "auto" && v != "fit-content" && v != "max-content" && v != "min-content" && !strings.Contains(v, "%") {
		width := int(length(v, 0, 0))
		if s["box-sizing"] != "border-box" {
			width += padding[1] + padding[3]
		}
		return width + margin[1] + margin[3]
	}
	if n.Data == "svg" {
		return int(length(nodeAttr(n, "width"), 0, 24))
	}
	if n.Data == "img" {
		if v := nodeAttr(n, "width"); v != "" {
			return int(length(v, 0, 0))
		}
		if img := images[imageAddress(base, nodeAttr(n, "src"))]; img != nil {
			return img.Bounds().Dx()
		}
	}
	current, widest, count := 0, 0, 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		cs := styles[c]
		if cs["display"] == "none" || cs["position"] == "absolute" || cs["position"] == "fixed" {
			continue
		}
		width := intrinsicWidth(c, styles, face, images, base, depth+1)
		if inlineLevel(c, cs) || (s["display"] == "flex" || s["display"] == "inline-flex") && s["flex-direction"] != "column" {
			if count > 0 && (s["display"] == "flex" || s["display"] == "inline-flex") {
				current += int(length(s["gap"], 0, 0))
			}
			count++
			current += width
		} else {
			widest = max(widest, current, width)
			current = 0
		}
	}
	return max(widest, current) + padding[1] + padding[3] + margin[1] + margin[3]
}
