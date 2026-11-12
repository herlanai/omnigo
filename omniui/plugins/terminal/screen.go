package terminal

import (
	"errors"
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Cell is a single-width Unicode code point. Wide/combining cluster layout is
// not implemented yet. Zero-alpha colors mean the viewport's theme defaults.
type Cell struct {
	Rune                   rune
	Foreground, Background color.NRGBA
	Bold, Inverse          bool
}
type Screen struct {
	alternateX, alternateY              int
	columns, rows, x, y, savedX, savedY int
	cells, alternate                    []Cell
	history                             [][]Cell
	HistoryLimit                        int
	style                               Cell
	cursor, wrap, alternateActive       bool
	mode                                byte
	sequence                            []byte
	pending                             []byte
}

func NewScreen(columns, rows int) (*Screen, error) {
	if columns < 1 || rows < 1 || columns > 1000 || rows > 1000 || columns*rows > 262144 {
		return nil, errors.New("terminal: screen budget exceeded")
	}
	s := &Screen{columns: columns, rows: rows, HistoryLimit: 1000, cursor: true}
	s.clear()
	return s, nil
}
func (s *Screen) clear() {
	s.cells = make([]Cell, s.columns*s.rows)
	for i := range s.cells {
		s.cells[i].Rune = ' '
	}
	s.x = 0
	s.y = 0
	s.wrap = false
}
func (s *Screen) Size() (int, int)         { return s.columns, s.rows }
func (s *Screen) Cursor() (int, int, bool) { return s.x, s.y, s.cursor }
func (s *Screen) Cells() []Cell            { return append([]Cell(nil), s.cells...) }
func (s *Screen) History() [][]Cell {
	out := make([][]Cell, len(s.history))
	for i, row := range s.history {
		out[i] = append([]Cell(nil), row...)
	}
	return out
}
func (s *Screen) Text() string {
	var b strings.Builder
	for y := 0; y < s.rows; y++ {
		for x := 0; x < s.columns; x++ {
			r := s.cells[y*s.columns+x].Rune
			if r == 0 {
				r = ' '
			}
			b.WriteRune(r)
		}
		if y+1 < s.rows {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func (s *Screen) Resize(columns, rows int) error {
	fresh, e := NewScreen(columns, rows)
	if e != nil {
		return e
	}
	resize := func(old []Cell) []Cell {
		next := append([]Cell(nil), fresh.cells...)
		for y := 0; y < min(rows, s.rows); y++ {
			copy(next[y*columns:y*columns+min(columns, s.columns)], old[y*s.columns:y*s.columns+min(columns, s.columns)])
		}
		return next
	}
	s.cells = resize(s.cells)
	if s.alternateActive {
		s.alternate = resize(s.alternate)
	}
	s.columns = columns
	s.rows = rows
	s.x = min(s.x, columns-1)
	s.y = min(s.y, rows-1)
	s.wrap = false
	return nil
}

func (s *Screen) blank() Cell { v := s.style; v.Rune = ' '; return v }
func (s *Screen) newline() {
	s.wrap = false
	if s.y+1 < s.rows {
		s.y++
		return
	}
	if !s.alternateActive && s.HistoryLimit > 0 {
		limit := min(s.HistoryLimit, 10000, max(1, 262144/s.columns))
		s.history = append(s.history, append([]Cell(nil), s.cells[:s.columns]...))
		if len(s.history) > limit {
			copy(s.history, s.history[len(s.history)-limit:])
			s.history = s.history[:limit]
		}
	}
	copy(s.cells, s.cells[s.columns:])
	for i := (s.rows - 1) * s.columns; i < len(s.cells); i++ {
		s.cells[i] = s.blank()
	}
}
func (s *Screen) put(r rune) {
	if s.wrap {
		s.x = 0
		s.newline()
	}
	v := s.style
	v.Rune = r
	s.cells[s.y*s.columns+s.x] = v
	if s.x == s.columns-1 {
		s.wrap = true
	} else {
		s.x++
	}
}

// Feed accepts arbitrary byte chunks, including split UTF-8 and escape sequences.
// Call only on the screen's UI goroutine, dispatching PTY reads from workers.
func (s *Screen) Feed(data []byte) error {
	if len(data) > 4<<20 {
		return errors.New("terminal: input chunk too large")
	}
	if len(s.pending) > 0 {
		data = append(append([]byte(nil), s.pending...), data...)
		s.pending = nil
	}
	for i := 0; i < len(data); {
		b := data[i]
		if s.mode == 'o' || s.mode == 'd' {
			i++
			if b == 7 && s.mode == 'o' {
				s.mode = 0
			} else if b == 27 {
				s.mode = 't'
			}
			continue
		}
		if s.mode == 't' {
			i++
			if b == '\\' {
				s.mode = 0
			} else {
				s.mode = 'o'
			}
			continue
		}
		if s.mode == 'e' {
			i++
			s.mode = 0
			switch b {
			case '[':
				s.mode = 'c'
				s.sequence = nil
			case ']':
				s.mode = 'o'
			case 'P', '^', '_':
				s.mode = 'd'
			case '7':
				s.savedX, s.savedY = s.x, s.y
			case '8':
				s.x, s.y = min(s.columns-1, s.savedX), min(s.rows-1, s.savedY)
				s.wrap = false
			case 'c':
				s.style = Cell{}
				s.alternate = nil
				s.alternateActive = false
				s.clear()
				s.history = nil
				s.cursor = true
			case 'D':
				s.newline()
			case 'E':
				s.x = 0
				s.newline()
			}
			continue
		}
		if s.mode == 'c' {
			i++
			if b >= 0x40 && b <= 0x7e {
				s.control(b)
				s.mode = 0
				s.sequence = nil
			} else if len(s.sequence) < 256 {
				s.sequence = append(s.sequence, b)
			} else {
				s.mode = 'x'
			}
			continue
		}
		if s.mode == 'x' {
			i++
			if b >= 0x40 && b <= 0x7e {
				s.mode = 0
			}
			continue
		}
		if b < 32 || b == 127 {
			i++
			switch b {
			case 27:
				s.mode = 'e'
			case '\r':
				s.x = 0
				s.wrap = false
			case '\n', '\v', '\f':
				s.newline()
			case '\b':
				s.x = max(0, s.x-1)
				s.wrap = false
			case '\t':
				s.x = min(s.columns-1, (s.x/8+1)*8)
				s.wrap = false
			}
			continue
		}
		if !utf8.FullRune(data[i:]) {
			s.pending = append([]byte(nil), data[i:]...)
			break
		}
		r, n := utf8.DecodeRune(data[i:])
		s.put(r)
		i += n
	}
	return nil
}
func (s *Screen) control(final byte) {
	raw := string(s.sequence)
	private := strings.HasPrefix(raw, "?")
	raw = strings.TrimPrefix(raw, "?")
	parts := strings.Split(raw, ";")
	params := make([]int, len(parts))
	for i, p := range parts {
		v, e := strconv.Atoi(p)
		if e == nil && v >= 0 {
			params[i] = min(v, 1000000)
		}
	}
	param := func(i, def int) int {
		if i >= len(params) || params[i] == 0 {
			return def
		}
		return params[i]
	}
	n := param(0, 1)
	switch final {
	case 'A':
		s.y = max(0, s.y-n)
		s.wrap = false
	case 'B':
		s.y = min(s.rows-1, s.y+n)
		s.wrap = false
	case 'C':
		s.x = min(s.columns-1, s.x+n)
		s.wrap = false
	case 'D':
		s.x = max(0, s.x-n)
		s.wrap = false
	case 'E':
		s.y = min(s.rows-1, s.y+n)
		s.x = 0
		s.wrap = false
	case 'F':
		s.y = max(0, s.y-n)
		s.x = 0
		s.wrap = false
	case 'G':
		s.x = min(s.columns-1, n-1)
		s.wrap = false
	case 'H', 'f':
		s.y = min(s.rows-1, param(0, 1)-1)
		s.x = min(s.columns-1, param(1, 1)-1)
		s.wrap = false
	case 's':
		s.savedX, s.savedY = s.x, s.y
	case 'u':
		s.x = min(s.columns-1, s.savedX)
		s.y = min(s.rows-1, s.savedY)
		s.wrap = false
	case 'J':
		start, end := s.y*s.columns+s.x, len(s.cells)
		if params[0] == 1 {
			start, end = 0, start+1
		}
		if params[0] == 2 || params[0] == 3 {
			start = 0
		}
		if params[0] == 3 {
			s.history = nil
		}
		for i := start; i < end; i++ {
			s.cells[i] = s.blank()
		}
	case 'K':
		start, end := s.y*s.columns+s.x, (s.y+1)*s.columns
		if params[0] == 1 {
			start, end = s.y*s.columns, start+1
		}
		if params[0] == 2 {
			start = s.y * s.columns
		}
		for i := start; i < end; i++ {
			s.cells[i] = s.blank()
		}
	case 'm':
		s.sgr(params)
	case 'h', 'l':
		if private {
			for _, p := range params {
				if p == 25 {
					s.cursor = final == 'h'
				}
				if p == 1049 {
					if final == 'h' && !s.alternateActive {
						s.alternate = append([]Cell(nil), s.cells...)
						s.alternateX, s.alternateY = s.x, s.y
						s.clear()
						s.alternateActive = true
					} else if final == 'l' && s.alternateActive {
						s.cells = s.alternate
						s.alternate = nil
						s.alternateActive = false
						s.x = min(s.columns-1, s.alternateX)
						s.y = min(s.rows-1, s.alternateY)
						s.wrap = false
					}
				}
			}
		}
	}
}

var ansiColors = []color.NRGBA{{0, 0, 0, 255}, {205, 49, 49, 255}, {13, 188, 121, 255}, {229, 229, 16, 255}, {36, 114, 200, 255}, {188, 63, 188, 255}, {17, 168, 205, 255}, {229, 229, 229, 255}, {102, 102, 102, 255}, {241, 76, 76, 255}, {35, 209, 139, 255}, {245, 245, 67, 255}, {59, 142, 234, 255}, {214, 112, 214, 255}, {41, 184, 219, 255}, {255, 255, 255, 255}}

func indexedColor(n int) color.NRGBA {
	n = max(0, min(255, n))
	if n < 16 {
		return ansiColors[n]
	}
	if n >= 232 {
		v := uint8(8 + 10*(n-232))
		return color.NRGBA{v, v, v, 255}
	}
	n -= 16
	levels := []uint8{0, 95, 135, 175, 215, 255}
	return color.NRGBA{levels[n/36], levels[(n/6)%6], levels[n%6], 255}
}
func (s *Screen) sgr(values []int) {
	for i := 0; i < len(values); i++ {
		n := values[i]
		switch {
		case n == 0:
			s.style = Cell{}
		case n == 1:
			s.style.Bold = true
		case n == 22:
			s.style.Bold = false
		case n == 7:
			s.style.Inverse = true
		case n == 27:
			s.style.Inverse = false
		case n == 39:
			s.style.Foreground = color.NRGBA{}
		case n == 49:
			s.style.Background = color.NRGBA{}
		case n >= 30 && n <= 37:
			s.style.Foreground = indexedColor(n - 30)
		case n >= 40 && n <= 47:
			s.style.Background = indexedColor(n - 40)
		case n >= 90 && n <= 97:
			s.style.Foreground = indexedColor(n - 90 + 8)
		case n >= 100 && n <= 107:
			s.style.Background = indexedColor(n - 100 + 8)
		case n == 38 || n == 48:
			var c color.NRGBA
			if i+2 < len(values) && values[i+1] == 5 {
				c = indexedColor(values[i+2])
				i += 2
			} else if i+4 < len(values) && values[i+1] == 2 {
				c = color.NRGBA{uint8(min(255, values[i+2])), uint8(min(255, values[i+3])), uint8(min(255, values[i+4])), 255}
				i += 4
			} else {
				continue
			}
			if n == 38 {
				s.style.Foreground = c
			} else {
				s.style.Background = c
			}
		}
	}
}
