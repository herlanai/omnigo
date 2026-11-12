package render

import (
	"fmt"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
	"golang.org/x/net/html"
	"image"
	"image/color"
	"image/draw"
	"math"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser"
	"github.com/herlanai/omnigo/omniui/plugins/omnibrowser/css"
	"strconv"
	"strings"
	"time"
)

func length(value string, relative, fallback float64) float64 {
	value = strings.TrimSpace(value)
	scale := 1.
	if strings.HasSuffix(value, "%") {
		scale = relative / 100
		value = strings.TrimSuffix(value, "%")
	} else if strings.HasSuffix(value, "rem") {
		scale = 16
		value = strings.TrimSuffix(value, "rem")
	} else if strings.HasSuffix(value, "em") {
		scale = 16
		value = strings.TrimSuffix(value, "em")
	} else {
		value = strings.TrimSuffix(value, "px")
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return fallback
	}
	return max(0, min(16384, n*scale))
}
func paintColor(v string, fallback color.NRGBA) color.NRGBA {
	named := map[string]string{"black": "#000000", "white": "#ffffff", "red": "#ff0000", "green": "#008000", "blue": "#0000ff", "gray": "#808080", "yellow": "#ffff00", "transparent": "#00000000"}
	if n, ok := named[strings.ToLower(v)]; ok {
		v = n
	}
	if strings.HasPrefix(v, "rgb(") || strings.HasPrefix(v, "rgba(") {
		parts := strings.Fields(strings.NewReplacer(",", " ", "/", " ", ")", "").Replace(v[strings.IndexByte(v, '(')+1:]))
		if len(parts) == 3 || len(parts) == 4 {
			channels := [4]uint8{0, 0, 0, 255}
			for i, p := range parts {
				n, err := strconv.ParseFloat(strings.TrimSuffix(p, "%"), 64)
				if err != nil {
					return fallback
				}
				if strings.HasSuffix(p, "%") {
					n *= 2.55
				} else if i == 3 {
					n *= 255
				}
				channels[i] = uint8(max(0, min(255, math.Round(n))))
			}
			return color.NRGBA{channels[0], channels[1], channels[2], channels[3]}
		}
	}
	if !strings.HasPrefix(v, "#") {
		return fallback
	}
	s := v[1:]
	if len(s) == 3 || len(s) == 4 {
		var b strings.Builder
		for _, c := range s {
			b.WriteRune(c)
			b.WriteRune(c)
		}
		s = b.String()
	}
	if len(s) != 6 && len(s) != 8 {
		return fallback
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return fallback
	}
	if len(s) == 6 {
		return color.NRGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}
	}
	return color.NRGBA{uint8(n >> 24), uint8(n >> 16), uint8(n >> 8), uint8(n)}
}

type paintBox struct {
	gradient    *gradient
	ellipse     bool
	matrix      f64.Aff3
	transformed bool
	clip        image.Rectangle
	clipped     bool
	bold        bool
	rect        image.Rectangle
	bg          color.NRGBA
	text        string
	fg          color.NRGBA
	size        float64
	radius      float64
	corners     [4]float64
	border      int
	borderColor color.NRGBA
	img         image.Image
}

// Screenshot paints the documented block/flex-row CSS subset. Unsupported
// layout modes are not a Chromium-equivalent rendering implementation.
func Screenshot(snapshot omnibrowser.Snapshot, width, height int, dark bool) (*image.RGBA, error) {
	return ScreenshotAt(snapshot, width, height, dark, 0)
}

// ScreenshotAt samples supported CSS transform animations at an explicit document time.
// It does not advance JavaScript timers or execute requestAnimationFrame callbacks.
func ScreenshotAt(snapshot omnibrowser.Snapshot, width, height int, dark bool, elapsed time.Duration) (*image.RGBA, error) {
	if width < 64 || height < 64 || width > 4096 || height > 4096 || width*height > 8_000_000 {
		return nil, fmt.Errorf("omnibrowser: invalid screenshot dimensions")
	}
	if len(snapshot.HTML) > 2<<20 {
		return nil, fmt.Errorf("document too large to render")
	}
	root, err := html.Parse(strings.NewReader(snapshot.HTML))
	if err != nil {
		return nil, err
	}
	images := decodeImages(snapshot)
	sheet := css.DocumentViewport(root, width, height)
	styles := sheet.Styles(root)
	generatePseudos(root, sheet, styles)
	sampleAnimations(sheet, styles, elapsed)
	for _, style := range styles {
		style["font-size"] = fmt.Sprintf("%.3fpx", viewportLength(style["font-size"], 16, 16, width, height))
		// Resolve absolute math before intrinsic sizing; percentages still need their containing box.
		for _, key := range []string{"width", "height", "min-width", "max-width", "min-height", "max-height", "gap", "padding-left", "padding-right", "padding-top", "padding-bottom"} {
			value := style[key]
			if strings.Contains(value, "(") && !strings.Contains(value, "%") {
				v := viewportLength(value, 0, math.NaN(), width, height)
				if !math.IsNaN(v) && !math.IsInf(v, 0) {
					style[key] = fmt.Sprintf("%.3fpx", v)
				}
			}
		}
	}
	type faceKey struct {
		size float64
		bold bool
	}
	faces := map[faceKey]font.Face{}
	defer func() {
		for _, f := range faces {
			_ = f.Close()
		}
	}()
	styledFace := func(size float64, bold bool) font.Face {
		size = max(8, min(96, size))
		key := faceKey{size, bold}
		if f := faces[key]; f != nil {
			return f
		}
		f := newStyledFace(size, bold)
		faces[key] = f
		return f
	}
	boxes := []paintBox{}
	containing := map[*html.Node]image.Rectangle{}
	resolve := func(v string, relative, fallback float64) float64 {
		return viewportLength(v, relative, fallback, width, height)
	}

	visited := 0
	var layout func(*html.Node, int, int, int, int) int
	layout = func(n *html.Node, x, y, w, depth int) int {
		visited++
		if visited > 10000 || depth > 128 || y > height {
			return 0
		}
		s := styles[n]
		if s["display"] == "none" || s["opacity"] == "0" {
			return 0
		}
		positioned := s["position"] == "absolute" || s["position"] == "fixed"
		cb := image.Rect(0, 0, width, height)
		if positioned {
			if s["position"] != "fixed" {
				for parent := n.Parent; parent != nil; parent = parent.Parent {
					if styles[parent]["position"] != "" && styles[parent]["position"] != "static" {
						if rect, ok := containing[parent]; ok {
							cb = rect
						}
						break
					}
				}
			}
			x, y, w = cb.Min.X, cb.Min.Y, cb.Dx()
			if s["left"] != "" && s["left"] != "auto" {
				x += int(resolve(s["left"], float64(cb.Dx()), 0))
			}
			if s["top"] != "" && s["top"] != "auto" {
				y += int(resolve(s["top"], float64(cb.Dy()), 0))
			}
			if s["left"] != "" && s["right"] != "" {
				w = max(0, w-int(resolve(s["left"], float64(w), 0))-int(resolve(s["right"], float64(w), 0)))
			}
		}
		parentHeight := height
		if r, ok := containing[n.Parent]; ok && r.Dy() > 0 {
			parentHeight = r.Dy()
		}
		if positioned {
			parentHeight = cb.Dy()
		}
		fs := max(8, min(96, resolve(s["font-size"], 16, 16)))
		line := max(int(math.Ceil(fs*1.4)), styledFace(fs, isBold(s)).Metrics().Height.Ceil())
		fg := paintColor(s["color"], color.NRGBA{32, 32, 32, 255})
		if n.Type == html.TextNode {
			words := textTokens(strings.TrimSpace(n.Data))
			if len(words) == 0 {
				return 0
			}
			text := ""
			used := 0
			f := styledFace(fs, isBold(s))
			emit := func() {
				boxes = append(boxes, paintBox{rect: image.Rect(x, y+used, x+w, y+used+line), text: text, fg: fg, size: fs, bold: isBold(s)})
				used += line
			}
			for _, word := range words {
				next := text + word
				if font.MeasureString(f, next).Ceil() > w && text != "" {
					emit()
					text = strings.TrimLeft(word, " ")
				} else {
					text = next
				}
			}
			emit()
			return used
		}
		if n.Type != html.ElementNode && n.Type != html.DocumentNode {
			return 0
		}

		if n.Data == "img" {
			source := images[imageAddress(snapshot.URL, nodeAttr(n, "src"))]
			if source != nil {
				iw, ih := source.Bounds().Dx(), source.Bounds().Dy()
				wv := s["width"]
				if wv == "" {
					wv = nodeAttr(n, "width")
				}
				hv := s["height"]
				if hv == "" {
					hv = nodeAttr(n, "height")
				}
				iw = int(resolve(wv, float64(w), float64(iw)))
				ih = int(resolve(hv, float64(height), float64(ih)))
				if hv == "" || hv == "auto" {
					ih = iw * source.Bounds().Dy() / max(1, source.Bounds().Dx())
				}
				if iw > 0 && ih > 0 {
					boxes = append(boxes, paintBox{rect: image.Rect(x, y, x+iw, y+ih), img: source})
				}
				return ih
			}
		}
		margin := edges(s, "margin", float64(w))
		padding := edges(s, "padding", float64(w))
		bw := 0
		bp := strings.Fields(s["border"])
		if len(bp) >= 2 && bp[1] != "none" {
			bw = int(resolve(bp[0], float64(w), 0))
		}
		for i := range padding {
			padding[i] += bw
		}
		available := max(0, w-margin[1]-margin[3])
		contentWidth := int(resolve(s["width"], float64(available), float64(available)))
		if positioned && (s["width"] == "" || s["width"] == "auto") && (s["left"] == "" || s["right"] == "") {
			contentWidth = min(available, intrinsicWidth(n, styles, styledFace, images, snapshot.URL, 0))
		}
		if maxWidth := s["max-width"]; maxWidth != "" && maxWidth != "none" {
			contentWidth = min(contentWidth, int(resolve(maxWidth, float64(available), float64(available))))
		}
		if minWidth := s["min-width"]; minWidth != "" {
			contentWidth = max(contentWidth, int(resolve(minWidth, float64(available), 0)))
		}
		if s["box-sizing"] != "border-box" && s["width"] != "" && s["width"] != "auto" {
			contentWidth += padding[1] + padding[3]
		}
		x += margin[3]
		y += margin[0]
		w = max(0, min(contentWidth, width))
		if strings.Contains(s["margin"], "auto") || s["margin-left"] == "auto" {
			x += max(0, (available-w)/2)
		}
		inner := max(0, w-padding[1]-padding[3])
		bg := paintColor(s["background-color"], color.NRGBA{})
		if s["background"] != "" {
			bg = paintColor(s["background"], bg)
		}
		index := len(boxes)
		initialH := int(max(resolve(s["height"], float64(parentHeight), 0), resolve(s["min-height"], float64(parentHeight), 0)))
		if initialH == 0 && (s["height"] == "" || s["height"] == "auto") {
			initialH = aspectHeight(s["aspect-ratio"], w)
		}
		containing[n] = image.Rect(x, y, x+w, y+initialH)
		border, borderColor := 0, fg
		parts := strings.Fields(s["border"])
		if len(parts) >= 2 && parts[1] != "none" {
			border = int(resolve(parts[0], float64(w), 0))
			if len(parts) > 2 {
				borderColor = paintColor(parts[2], fg)
			}
		}
		if s["border-color"] != "" {
			borderColor = paintColor(s["border-color"], borderColor)
		}
		radius := resolve(strings.Fields(s["border-radius"] + " 0")[0], float64(w), 0)
		boxes = append(boxes, paintBox{rect: image.Rect(x, y, x+w, y), bg: bg, gradient: backgroundGradient(s), ellipse: s["border-radius"] == "50%", border: border, borderColor: borderColor, radius: radius, corners: cornerRadii(s["border-radius"], float64(w))})
		if n.Data == "svg" {
			sw := w
			sh := int(resolve(s["height"], float64(containing[n.Parent].Dy()), 24))
			if v := nodeAttr(n, "width"); v != "" && s["width"] == "" {
				sw = int(resolve(v, float64(w), 24))
			}
			if v := nodeAttr(n, "height"); v != "" && s["height"] == "" {
				sh = int(resolve(v, float64(height), 24))
			}
			boxes[index].rect = image.Rect(x, y, x+sw, y+sh)
			boxes[index].img = rasterSVG(prepareSVG(n, styles), sw, sh, fg)
			if s["filter"] == "invert()" || s["filter"] == "invert(1)" || s["filter"] == "invert(100%)" {
				boxes[index].img = invertImage(boxes[index].img)
			}
			return sh
		}
		type item struct {
			start, end, height int
			node               *html.Node
		}
		var items []item
		used := 0
		gap := int(resolve(s["gap"], float64(w), 0))
		if s["display"] == "grid" || (s["display"] == "flex" || s["display"] == "inline-flex") && s["flex-direction"] != "column" {
			var children []*html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if (c.Type == html.ElementNode || c.Type == html.TextNode && strings.TrimSpace(c.Data) != "") && styles[c]["display"] != "none" && styles[c]["position"] != "absolute" && styles[c]["position"] != "fixed" {
					children = append(children, c)
				}
			}
			count := len(children)

			if count > 0 {
				if s["display"] == "grid" {
					cols := columns(s["grid-template-columns"])
					tracks := gridTracks(s["grid-template-columns"], inner, gap)
					cols = len(tracks)
					rowH := 0
					for i, c := range children {
						if i > 0 && i%cols == 0 {
							used += rowH + gap
							rowH = 0
						}
						cx := 0
						for _, tw := range tracks[:i%cols] {
							cx += tw + gap
						}
						start := len(boxes)
						ch := layout(c, x+padding[3]+cx, y+padding[0]+used, tracks[i%cols], depth+1)
						if count <= cols {
							items = append(items, item{start, len(boxes), ch, c})
						}
						rowH = max(rowH, ch)
					}
					used += rowH
				} else {
					widths := make([]int, count)
					fixed, growing := gap*(count-1), 0
					for i, c := range children {
						cs := styles[c]
						if cs["flex"] == "1" || cs["flex-grow"] == "1" {
							growing++
							widths[i] = -1
							continue
						}
						widths[i] = max(1, intrinsicWidth(c, styles, styledFace, images, snapshot.URL, 0))
						fixed += widths[i]
					}
					for i, w := range widths {
						if w < 0 {
							widths[i] = max(0, (inner-fixed)/max(1, growing))
						}
					}
					cursor, rowH := 0, 0
					itemGap := gap
					if growing == 0 && fixed < inner {
						if s["justify-content"] == "space-between" && count > 1 {
							itemGap += (inner - fixed) / (count - 1)
						}
						if s["justify-content"] == "center" {
							cursor = (inner - fixed) / 2
						}
						if s["justify-content"] == "flex-end" {
							cursor = inner - fixed
						}
					}
					for i, c := range children {
						cw := min(inner, widths[i])
						if s["flex-wrap"] == "wrap" && cursor > 0 && cursor+cw > inner {
							used += rowH + gap
							cursor, rowH = 0, 0
						}
						start := len(boxes)
						ch := layout(c, x+padding[3]+cursor, y+padding[0]+used, cw, depth+1)
						if s["flex-wrap"] != "wrap" {
							items = append(items, item{start, len(boxes), ch, c})
						}
						rowH = max(rowH, ch)
						cursor += cw + itemGap
					}
					used += rowH
				}
			}

		} else {
			var run []*html.Node
			flush := func() {
				if len(run) > 0 {
					used += inlineFormat(run, x+padding[3], y+padding[0]+used, inner, depth+1, s, styles, &boxes, styledFace, layout, images, snapshot.URL)
					run = nil
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				cs := styles[c]
				if cs["position"] == "absolute" || cs["position"] == "fixed" || cs["display"] == "none" {
					continue
				}
				if s["display"] != "flex" && inlineLevel(c, cs) {
					run = append(run, c)
					continue
				}
				flush()
				spacing := 0
				if used > 0 && s["display"] == "flex" {
					spacing = gap
				}
				ch := layout(c, x+padding[3], y+padding[0]+used+spacing, inner, depth+1)
				if ch > 0 {
					used += spacing + ch
				}
			}
			flush()
		}

		if n.Data == "input" || n.Data == "textarea" {
			text := nodeAttr(n, "value")
			if text == "" {
				text = nodeAttr(n, "placeholder")
			}
			if text != "" {
				tx, ty := x+padding[3], y+padding[0]
				if s["text-align"] == "center" {
					tx = x + (w-font.MeasureString(styledFace(fs, isBold(s)), text).Ceil())/2
				}
				if s["box-sizing"] == "border-box" {
					ty = y + max(padding[0], (int(resolve(s["height"], float64(parentHeight), float64(line)))-line)/2)
				}
				boxes = append(boxes, paintBox{rect: image.Rect(tx, ty, x+w, ty+line), text: text, fg: fg, size: fs, bold: isBold(s)})
				used = max(used, line)
			}
		}
		h := int(resolve(s["height"], float64(parentHeight), float64(used+padding[0]+padding[2])))
		if s["box-sizing"] != "border-box" && s["height"] != "" && s["height"] != "auto" {
			h += padding[0] + padding[2]
		}
		if s["height"] == "" || s["height"] == "auto" {
			h = max(h, aspectHeight(s["aspect-ratio"], w))
		}
		h = max(h, int(resolve(s["min-height"], float64(parentHeight), 0)))
		if positioned && (s["height"] == "" || s["height"] == "auto") && s["top"] != "" && s["bottom"] != "" {
			h = max(0, cb.Dy()-int(resolve(s["top"], float64(cb.Dy()), 0))-int(resolve(s["bottom"], float64(cb.Dy()), 0)))
		}
		for childIndex, child := range items {
			align := s["align-items"]
			if v := styles[child.node]["align-self"]; v != "" && v != "auto" {
				align = v
			}
			dy := 0
			if align == "center" {
				dy = (max(used, h-padding[0]-padding[2]) - child.height) / 2
			}
			if align == "end" || align == "flex-end" {
				dy = max(used, h-padding[0]-padding[2]) - child.height
			}
			dx := 0
			justify := styles[child.node]["justify-self"]
			if justify == "" {
				justify = s["justify-items"]
			}
			if s["display"] == "grid" && justify == "center" && child.start < child.end {
				tracks := gridTracks(s["grid-template-columns"], inner, gap)
				dx = (tracks[childIndex%len(tracks)] - boxes[child.start].rect.Dx()) / 2
			}
			for i := child.start; i < child.end; i++ {
				if boxes[i].transformed {
					boxes[i].matrix = multiply(f64.Aff3{1, 0, float64(dx), 0, 1, float64(dy)}, multiply(boxes[i].matrix, f64.Aff3{1, 0, float64(-dx), 0, 1, float64(-dy)}))
				}
				boxes[i].rect = boxes[i].rect.Add(image.Pt(dx, dy))
				if boxes[i].clipped {
					boxes[i].clip = boxes[i].clip.Add(image.Pt(dx, dy))
				}
			}
		}
		boxes[index].rect.Max.Y = y + h
		containing[n] = image.Rect(x, y, x+w, y+h)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if styles[c]["position"] == "absolute" || styles[c]["position"] == "fixed" {
				layout(c, x, y, w, depth+1)
			}
		}
		if s["overflow"] == "hidden" || s["overflow"] == "clip" {
			clip := image.Rect(x, y, x+w, y+h)
			for i := index + 1; i < len(boxes); i++ {
				if boxes[i].clipped {
					boxes[i].clip = boxes[i].clip.Intersect(clip)
				} else {
					boxes[i].clip = clip
					boxes[i].clipped = true
				}
			}
		}
		if s["transform"] != "" && s["transform"] != "none" {
			m := cssTransform(s["transform"], boxes[index].rect)
			for i := index; i < len(boxes); i++ {
				if boxes[i].transformed {
					boxes[i].matrix = multiply(m, boxes[i].matrix)
				} else {
					boxes[i].matrix = m
					boxes[i].transformed = true
				}
			}
		}
		dx, dy := 0, 0
		if s["position"] == "relative" {
			dx += int(resolve(s["left"], float64(w), 0))
			dy += int(resolve(s["top"], float64(parentHeight), 0))
		}
		if positioned && s["top"] == "" && s["bottom"] != "" {
			dy += cb.Max.Y - int(resolve(s["bottom"], float64(cb.Dy()), 0)) - h - y
		}
		if positioned && s["left"] == "" && s["right"] != "" {
			dx += cb.Max.X - int(resolve(s["right"], float64(cb.Dx()), 0)) - w - x
		}
		if dx != 0 || dy != 0 {
			for i := index; i < len(boxes); i++ {
				if boxes[i].transformed {
					boxes[i].matrix = multiply(f64.Aff3{1, 0, float64(dx), 0, 1, float64(dy)}, multiply(boxes[i].matrix, f64.Aff3{1, 0, float64(-dx), 0, 1, float64(-dy)}))
				}
				boxes[i].rect = boxes[i].rect.Add(image.Pt(dx, dy))
				if boxes[i].clipped {
					boxes[i].clip = boxes[i].clip.Add(image.Pt(dx, dy))
				}
			}
		}
		if s["visibility"] == "hidden" {
			boxes = boxes[:index]
		}
		if positioned {
			return 0
		}
		return h + margin[0] + margin[2]
	}
	layout(root, 0, 0, width, 0)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	base := color.NRGBA{255, 255, 255, 255}
	if dark {
		base = color.NRGBA{24, 28, 26, 255}
	}
	draw.Draw(img, img.Bounds(), &image.Uniform{base}, image.Point{}, draw.Src)
	for _, box := range boxes {
		target := img
		if box.clipped {
			area := box.clip.Intersect(img.Bounds())
			if area.Empty() {
				continue
			}
			target = img.SubImage(area).(*image.RGBA)
		}
		finalTarget := target
		if box.transformed {
			if box.rect.Empty() || box.rect.Dx() > 4096 || box.rect.Dy() > 4096 || box.rect.Dx()*box.rect.Dy() > 8_000_000 {
				continue
			}
			target = image.NewRGBA(box.rect)
		}
		paintDecoration(target, box)
		if box.img != nil {
			xdraw.CatmullRom.Scale(target, box.rect, box.img, box.img.Bounds(), draw.Over, nil)
		}
		if box.text != "" {
			f := styledFace(box.size, box.bold)
			d := font.Drawer{Dst: target, Src: &image.Uniform{box.fg}, Face: f, Dot: fixed.P(box.rect.Min.X, box.rect.Min.Y+f.Metrics().Ascent.Ceil())}
			d.DrawString(box.text)
		}
		if box.transformed {
			xdraw.CatmullRom.Transform(finalTarget, box.matrix, target, target.Bounds(), draw.Over, nil)
		}
	}
	return img, nil
}
