package baseui

import (
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"strings"
	"unicode"
)

// TextSpan is immutable display data. Link callbacks belong to the application.
type TextSpan struct {
	Text, Link                 string
	Bold, Italic, Code, Strike bool
	Monospace                  bool
}
type RichTextState struct {
	Link    int
	pressed int
	armed   bool
}
type RichTextOptions struct {
	Size   float64
	State  *RichTextState
	OnLink func(string)
}
type richFragment struct {
	text string
	span TextSpan
	box  Rect
	link int
}

var richFonts = func() []*opentype.Font {
	var out []*opentype.Font
	for _, data := range [][]byte{goregular.TTF, gobold.TTF, goitalic.TTF, gobolditalic.TTF, gomono.TTF, gomonobold.TTF} {
		f, e := opentype.Parse(data)
		if e != nil {
			panic(e)
		}
		out = append(out, f)
	}
	return platformFonts(out)
}()

func richStyle(s TextSpan) int {
	if s.Code || s.Monospace {
		if s.Bold {
			return 5
		}
		return 4
	}
	n := 0
	if s.Bold {
		n++
	}
	if s.Italic {
		n += 2
	}
	return n
}
func richFace(style int, size float64) font.Face {
	f, e := opentype.NewFace(richFonts[style], &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if e != nil {
		panic(e)
	}
	return f
}

// RichText lays out formatted runs in a single wrapping paragraph. Links share
// one Tab stop per paragraph; Left/Right choose a link and Enter activates it.
// Selection, per-link platform accessibility and complex shaping remain separate.
func RichText(id string, spans []TextSpan, o RichTextOptions) *Element {
	if o.Size == 0 {
		o.Size = 14
	}
	if !finite(o.Size) || o.Size < 6 || o.Size > 96 {
		panic("baseui: invalid rich text size")
	}
	spans = append([]TextSpan(nil), spans...)
	state := o.State
	if state == nil {
		state = &RichTextState{}
	}
	var plain strings.Builder
	var links []string
	indices := make([]int, len(spans))
	for i, s := range spans {
		plain.WriteString(s.Text)
		indices[i] = -1
		if s.Link != "" {
			found := -1
			for j, v := range links {
				if v == s.Link {
					found = j
					break
				}
			}
			if found < 0 {
				found = len(links)
				links = append(links, s.Link)
			}
			indices[i] = found
		}
	}
	lineHeight := o.Size * 1.55
	cachedWidth := -1.
	var fragments []richFragment
	var measuredW, measuredH float64
	layout := func(width float64) {}
	layout = func(width float64) {
		if width == cachedWidth {
			return
		}
		cachedWidth = width
		fragments = nil
		measuredW = 0
		x, y := 0., 0.
		faces := map[int]font.Face{}
		defer func() {
			for _, f := range faces {
				f.Close()
			}
		}()
		newline := func() { measuredW = max(measuredW, x); x = 0; y += lineHeight }
		for i, span := range spans {
			style := richStyle(span)
			face := faces[style]
			if face == nil {
				face = richFace(style, o.Size)
				faces[style] = face
			}
			runes := []rune(strings.ReplaceAll(span.Text, "\t", "    "))
			for start := 0; start < len(runes); {
				if runes[start] == '\n' {
					newline()
					start++
					continue
				}
				end := start + 1
				space := unicode.IsSpace(runes[start])
				for end < len(runes) && runes[end] != '\n' && unicode.IsSpace(runes[end]) == space {
					end++
				}
				token := runes[start:end]
				tw := float64(font.MeasureString(face, string(token))) / 64
				if !space && x > 0 && x+tw > width {
					newline()
				}
				remainingWidth := tw
				for len(token) > 0 {
					n := len(token)
					advance := remainingWidth
					if advance > width-x {
						n = 0
						advance = 0
						for n < len(token) {
							w := float64(font.MeasureString(face, string(token[n]))) / 64
							if n > 0 && x+advance+w > width {
								break
							}
							advance += w
							n++
							if x+advance > width {
								break
							}
						}
					}
					if space && x == 0 && y > 0 {
						token = nil
						break
					}
					fragments = append(fragments, richFragment{string(token[:n]), span, Rect{x, y, advance, lineHeight}, indices[i]})
					x += advance
					token = token[n:]
					remainingWidth -= advance
					if len(token) > 0 {
						newline()
					}
				}
				start = end
			}
		}
		measuredW = min(width, max(measuredW, x))
		measuredH = y + lineHeight
	}
	e := Canvas(id, plain.String(), func(p *Painter, r Rect) {
		layout(max(1, r.W))
		faces := map[int]font.Face{}
		defer func() {
			for _, f := range faces {
				f.Close()
			}
		}()
		scale := p.scale
		if scale == 0 {
			scale = 1
		}
		for _, fragment := range fragments {
			b := fragment.box
			b.X += r.X
			b.Y += r.Y
			if intersect(b, p.clip).H <= 0 {
				continue
			}
			style := richStyle(fragment.span)
			f := faces[style]
			if f == nil {
				f = richFace(style, o.Size*scale)
				faces[style] = f
			}
			q := *p
			q.face = f
			fg := p.theme.Text
			if fragment.span.Code {
				p.Fill(b, p.theme.Muted, 3)
			}
			if fragment.link >= 0 {
				fg = p.theme.Accent
			}
			baseline := b.Y + o.Size*1.15
			q.Text(b.X, baseline, fragment.text, fg)
			if fragment.link >= 0 {
				p.Line(b.X, baseline+2, b.X+b.W, baseline+2, 1, fg)
			}
			if fragment.span.Strike {
				p.Line(b.X, baseline-o.Size*.3, b.X+b.W, baseline-o.Size*.3, 1, fg)
			}
		}
	}).Role("text")
	e.measureContent = func(width float64) (float64, float64) { layout(width); return measuredW, measuredH }
	if len(links) > 0 && o.OnLink != nil {
		e.description = "Links: " + strings.Join(links, ", ") + ". Left/Right selects; Enter opens."
		e.onKey = func(ev KeyEvent) bool {
			if ev.Up {
				return false
			}
			state.Link = max(0, min(len(links)-1, state.Link))
			switch ev.Key {
			case "Left":
				state.Link = (state.Link + len(links) - 1) % len(links)
			case "Right":
				state.Link = (state.Link + 1) % len(links)
			case "Home":
				state.Link = 0
			case "End":
				state.Link = len(links) - 1
			case "Enter":
				o.OnLink(links[state.Link])
			default:
				return false
			}
			return true
		}
		e.onPointer = func(ev PointerEvent, r, _ Rect) {
			layout(max(1, r.W))
			hit := -1
			for _, f := range fragments {
				if f.link >= 0 && f.box.Contains(ev.X-r.X, ev.Y-r.Y) {
					hit = f.link
					break
				}
			}
			switch ev.Kind {
			case "down":
				state.pressed = hit
				state.armed = true
			case "cancel":
				state.armed = false
				state.pressed = -1
			case "up":
				pressed := state.pressed
				armed := state.armed
				state.armed = false
				state.pressed = -1
				if armed && hit >= 0 && hit == pressed {
					state.Link = hit
					o.OnLink(links[hit])
				}
			}
		}
	}
	return e
}
