// Package icon parses monochrome SVG icons and rasterizes them entirely in Go.
// SVG data is immutable after parsing and safe to render concurrently.
package icon

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

const MaxSVGBytes = 256 << 10

var numbers = regexp.MustCompile(`[+-]?(?:\d*\.\d+|\d+\.?\d*)(?:[eE][+-]?\d+)?`)
var paintMarker = color.NRGBA{R: 1, G: 2, B: 3, A: 255}

type rasterKey struct {
	w, h   int
	c      color.NRGBA
	stroke float64
}
type SVG struct {
	doc   *oksvg.SvgIcon
	mu    sync.Mutex
	cache map[rasterKey]*image.RGBA
	order []rasterKey
}

// ParseSVG accepts the path/basic-shape subset used by Lucide. All non-none
// paint becomes the requested icon color. External resources, CSS, transforms,
// text and SVG filters are rejected rather than rendered incompletely.
func ParseSVG(data []byte) (result *SVG, err error) {
	if len(data) == 0 || len(data) > MaxSVGBytes {
		return nil, errors.New("icon: SVG size limit")
	}
	defer func() {
		if v := recover(); v != nil {
			result = nil
			err = fmt.Errorf("icon: invalid SVG: %v", v)
		}
	}()
	normalized, err := normalize(data)
	if err != nil {
		return nil, err
	}
	doc, err := oksvg.ReadIconStream(bytes.NewReader(normalized), oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	if !valid(doc.ViewBox.W) || !valid(doc.ViewBox.H) || doc.ViewBox.W <= 0 || doc.ViewBox.H <= 0 || doc.ViewBox.W > 4096 || doc.ViewBox.H > 4096 || len(doc.SVGPaths) == 0 {
		return nil, errors.New("icon: invalid or empty viewBox")
	}
	if doc.ViewBox.X != 0 || doc.ViewBox.Y != 0 {
		return nil, errors.New("icon: nonzero viewBox origin unsupported")
	}
	return &SVG{doc: doc, cache: map[rasterKey]*image.RGBA{}}, nil
}
func MustParseSVG(data []byte) *SVG {
	v, e := ParseSVG(data)
	if e != nil {
		panic(e)
	}
	return v
}
func valid(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func normalize(data []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	depth, nodes, roots := 0, 0, 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			nodes++
			if depth > 32 || nodes > 4096 {
				return nil, errors.New("icon: complexity limit")
			}
			if depth == 1 {
				roots++
				if roots != 1 || t.Name.Local != "svg" {
					return nil, errors.New("icon: expected one SVG root")
				}
			}
			switch t.Name.Local {
			case "svg":
				if depth != 1 {
					return nil, errors.New("icon: nested SVG unsupported")
				}
			case "g", "path", "circle", "ellipse", "rect", "line", "polyline", "polygon":
			default:
				return nil, fmt.Errorf("icon: unsupported element %q", t.Name.Local)
			}
			if t.Name.Space != "" && t.Name.Space != "http://www.w3.org/2000/svg" {
				return nil, errors.New("icon: foreign namespace")
			}
			t.Name.Space = ""
			attrs := []xml.Attr{}
			hasFill, hasStroke := false, false
			for _, a := range t.Attr {
				name := a.Name.Local
				if name == "xmlns" || name == "class" || name == "id" {
					continue
				}
				if a.Name.Space != "" {
					return nil, errors.New("icon: namespaced attribute unsupported")
				}
				switch name {
				case "fill", "stroke":
					if strings.Contains(strings.ToLower(a.Value), "url(") {
						return nil, errors.New("icon: paint references unsupported")
					}
					if name == "fill" {
						hasFill = true
					} else {
						hasStroke = true
					}
					if a.Value != "none" {
						a.Value = "#010203"
					}
				case "stroke-linecap":
					if a.Value != "round" && a.Value != "butt" && a.Value != "square" {
						return nil, errors.New("icon: invalid line cap")
					}
				case "stroke-linejoin":
					if a.Value != "round" && a.Value != "miter" && a.Value != "bevel" {
						return nil, errors.New("icon: invalid line join")
					}
				case "fill-rule":
					if a.Value != "evenodd" && a.Value != "nonzero" {
						return nil, errors.New("icon: invalid fill rule")
					}
				case "d", "points", "viewBox", "width", "height", "x", "y", "x1", "y1", "x2", "y2", "cx", "cy", "r", "rx", "ry", "stroke-width", "opacity", "fill-opacity", "stroke-opacity":
					if name == "d" {
						var err error
						a.Value, err = normalizePath(a.Value)
						if err != nil {
							return nil, err
						}
					}
					if strings.ContainsAny(a.Value, "NnIi") && name != "d" {
						return nil, errors.New("icon: invalid numeric value")
					}
					for _, n := range numbers.FindAllString(a.Value, -1) {
						v, e := strconv.ParseFloat(n, 64)
						if e != nil || !valid(v) || math.Abs(v) > 16384 {
							return nil, errors.New("icon: coordinate limit")
						}
					}
				default:
					return nil, fmt.Errorf("icon: unsupported attribute %q", name)
				}
				attrs = append(attrs, a)
			}
			if depth == 1 {
				if !hasFill {
					attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "fill"}, Value: "#010203"})
				}
				if !hasStroke {
					attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "stroke"}, Value: "none"})
				}
			}
			t.Attr = attrs
			token = t
		case xml.EndElement:
			depth--
			t.Name.Space = ""
			token = t
		case xml.Comment:
			continue
		case xml.ProcInst:
			if t.Target == "xml" {
				continue
			}
			return nil, errors.New("icon: processing instruction unsupported")
		case xml.Directive:
			return nil, errors.New("icon: XML directive unsupported")
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, errors.New("icon: text unsupported")
			}
		}
		if e := encoder.EncodeToken(token); e != nil {
			return nil, e
		}
	}
	if roots != 1 || depth != 0 {
		return nil, errors.New("icon: incomplete SVG")
	}
	if e := encoder.Flush(); e != nil {
		return nil, e
	}
	return output.Bytes(), nil
}
func (s *SVG) Size() (float64, float64) {
	if s == nil {
		return 0, 0
	}
	return s.doc.ViewBox.W, s.doc.ViewBox.H
}
func copyImage(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}

// Rasterize returns owned pixels. Stroke is in SVG viewBox units; zero preserves
// the source width. The bounded cache holds at most eight small rendered images.
func (s *SVG) Rasterize(width, height int, c color.NRGBA, stroke float64) (image.Image, error) {
	if s == nil || width < 1 || height < 1 || width > 2048 || height > 2048 || width*height > 1048576 || !valid(stroke) || stroke < 0 || stroke > 64 {
		return nil, errors.New("icon: invalid raster size or stroke")
	}
	key := rasterKey{width, height, c, stroke}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached := s.cache[key]; cached != nil {
		return copyImage(cached), nil
	}
	alpha := float64(c.A) / 255
	c.A = 255
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	doc := *s.doc
	doc.SVGPaths = append([]oksvg.SvgPath(nil), s.doc.SVGPaths...)
	scale := math.Min(float64(width)/doc.ViewBox.W, float64(height)/doc.ViewBox.H)
	doc.SetTarget((float64(width)-doc.ViewBox.W*scale)/2, (float64(height)-doc.ViewBox.H*scale)/2, doc.ViewBox.W*scale, doc.ViewBox.H*scale)
	for i := range doc.SVGPaths {
		path := &doc.SVGPaths[i]
		if path.GetFillColor() == paintMarker {
			path.SetFillColor(c)
		}
		if path.GetLineColor() == paintMarker {
			path.SetLineColor(c)
		}
		if stroke > 0 {
			path.LineWidth = stroke
		}
		path.LineWidth *= scale
	}
	scanner := rasterx.NewScannerGV(width, height, out, out.Bounds())
	doc.Draw(rasterx.NewDasher(width, height, scanner), alpha)
	if width*height <= 65536 {
		if len(s.order) == 8 {
			delete(s.cache, s.order[0])
			s.order = s.order[1:]
		}
		s.cache[key] = out
		s.order = append(s.order, key)
		return copyImage(out), nil
	}
	return out, nil
}
