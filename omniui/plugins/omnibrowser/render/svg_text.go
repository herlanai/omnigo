package render

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html"
	"image"
	"strings"
)

// Prepare a private SVG tree. Text becomes font outlines before rasterization,
// preserving painter order, group transforms, clipping and viewBox scaling.
func prepareSVG(n *html.Node, styles map[*html.Node]map[string]string) *html.Node {
	budget := 4096
	var clone func(*html.Node) *html.Node
	clone = func(src *html.Node) *html.Node {
		dst := &html.Node{Type: src.Type, Data: src.Data, DataAtom: src.DataAtom, Namespace: src.Namespace, Attr: append([]html.Attribute(nil), src.Attr...)}
		s := styles[src]
		if v := s["transform"]; v != "" && v != "none" {
			m := cssTransform(v, image.Rectangle{})
			dst.Attr = append([]html.Attribute{}, dst.Attr...)
			for i := len(dst.Attr) - 1; i >= 0; i-- {
				if dst.Attr[i].Key == "transform" {
					dst.Attr = append(dst.Attr[:i], dst.Attr[i+1:]...)
				}
			}
			dst.Attr = append(dst.Attr, html.Attribute{Key: "transform", Val: fmt.Sprintf("matrix(%g %g %g %g %g %g)", m[0], m[3], m[1], m[4], m[2], m[5])})
		}
		set := func(k, v string) {
			for i := range dst.Attr {
				if dst.Attr[i].Key == k {
					dst.Attr[i].Val = v
					return
				}
			}
			dst.Attr = append(dst.Attr, html.Attribute{Key: k, Val: v})
		}
		for _, k := range []string{"fill", "stroke", "stroke-width", "opacity", "fill-opacity", "stroke-opacity", "font-size", "font-weight", "text-anchor", "letter-spacing"} {
			if v := s[k]; v != "" {
				set(k, v)
			}
		}
		if src.Data == "text" {
			dst.Data = "g"
			dst.DataAtom = 0
			for _, k := range []string{"x", "y", "text-anchor", "font-size", "font-weight", "letter-spacing"} {
				for i := len(dst.Attr) - 1; i >= 0; i-- {
					if dst.Attr[i].Key == k {
						dst.Attr = append(dst.Attr[:i], dst.Attr[i+1:]...)
					}
				}
			}
			// The bounded simple-text profile handles x/y/dx/dy and anchor for a text run.
			x, y := length(nodeAttr(src, "x"), 0, 0), length(nodeAttr(src, "y"), 0, 0)
			x += viewportLength(nodeAttr(src, "dx"), 0, 0, 1024, 768)
			y += viewportLength(nodeAttr(src, "dy"), 0, 0, 1024, 768)
			size := length(s["font-size"], 16, 16)
			if size <= 0 {
				size = 16
			}
			size = min(256, size)
			text := strings.Join(strings.Fields(nodeText(src)), " ")
			runes := []rune(text)
			if len(runes) > budget {
				runes = runes[:budget]
			}
			budget -= len(runes)
			text = string(runes)
			path, advance := glyphPath(text, size, isBold(s), viewportLength(s["letter-spacing"], size, 0, 1024, 768))
			anchor := s["text-anchor"]
			if anchor == "" {
				anchor = nodeAttr(src, "text-anchor")
			}
			if anchor == "middle" {
				x -= advance / 2
			}
			if anchor == "end" {
				x -= advance
			}
			dst.AppendChild(&html.Node{Type: html.ElementNode, Data: "path", Namespace: "svg", Attr: []html.Attribute{{Key: "d", Val: path}, {Key: "transform", Val: fmt.Sprintf("translate(%g %g)", x, y)}}})
			return dst
		}
		for c := src.FirstChild; c != nil; c = c.NextSibling {
			dst.AppendChild(clone(c))
		}
		return dst
	}
	return clone(n)
}
func glyphPath(text string, size float64, bold bool, spacing float64) (string, float64) {
	sources := bundledFonts()
	offset := 0
	if bold {
		offset = 2
	}
	fonts := append(append([]*sfnt.Font{}, parsedFonts()...), sources[offset], sources[offset+1])
	var buffer sfnt.Buffer
	var out strings.Builder
	cursor := 0.0
	var previous rune
	var lastFont *sfnt.Font
	count := 0
	for _, r := range text {
		count++
		if count > 4096 || out.Len() > 1<<20 {
			break
		}
		selected := fonts[0]
		var glyph sfnt.GlyphIndex
		for _, f := range fonts {
			g, e := f.GlyphIndex(&buffer, r)
			if e == nil && g != 0 {
				selected, glyph = f, g
				break
			}
		}
		ppem := fixed.Int26_6(size * 64)
		if lastFont == selected {
			a, _ := selected.GlyphIndex(&buffer, previous)
			k, _ := selected.Kern(&buffer, a, glyph, ppem, font.HintingNone)
			cursor += float64(k) / 64
		}
		segments, e := selected.LoadGlyph(&buffer, glyph, ppem, nil)
		if e != nil {
			continue
		}
		for _, seg := range segments {
			switch seg.Op {
			case sfnt.SegmentOpMoveTo:
				fmt.Fprintf(&out, "M%.3f %.3f", cursor+float64(seg.Args[0].X)/64, float64(seg.Args[0].Y)/64)
			case sfnt.SegmentOpLineTo:
				fmt.Fprintf(&out, "L%.3f %.3f", cursor+float64(seg.Args[0].X)/64, float64(seg.Args[0].Y)/64)
			case sfnt.SegmentOpQuadTo:
				fmt.Fprintf(&out, "Q%.3f %.3f %.3f %.3f", cursor+float64(seg.Args[0].X)/64, float64(seg.Args[0].Y)/64, cursor+float64(seg.Args[1].X)/64, float64(seg.Args[1].Y)/64)
			case sfnt.SegmentOpCubeTo:
				fmt.Fprintf(&out, "C%.3f %.3f %.3f %.3f %.3f %.3f", cursor+float64(seg.Args[0].X)/64, float64(seg.Args[0].Y)/64, cursor+float64(seg.Args[1].X)/64, float64(seg.Args[1].Y)/64, cursor+float64(seg.Args[2].X)/64, float64(seg.Args[2].Y)/64)
			}
		}
		advance, _ := selected.GlyphAdvance(&buffer, glyph, ppem, font.HintingNone)
		cursor += float64(advance)/64 + spacing
		previous, lastFont = r, selected
	}
	if count > 0 {
		cursor -= spacing
	}
	return out.String(), cursor
}
