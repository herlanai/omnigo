package baseui

import (
	"fmt"
	"image/color"
	"strings"
)

// DiffHunk uses zero-based, half-open line ranges. Line endings are preserved.
type DiffHunk struct{ OldStart, OldEnd, NewStart, NewEnd int }
type diffRow struct{ old, new int }

// DiffState is UI-goroutine confined. SetText invalidates the computed diff.
// Large comparisons use a bounded coarse replacement instead of quadratic memory.
type DiffState struct {
	Old, New           string
	Unified            bool
	Current            int
	Scroll             ScrollState
	oldText, newText   string
	ready              bool
	oldLines, newLines []string
	hunks              []DiffHunk
	rows               []diffRow
	undo               []string
}

func diffLines(s string) []string {
	if s == "" {
		return nil
	}
	a := strings.SplitAfter(s, "\n")
	if a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	return a
}
func (s *DiffState) SetText(old, next string) {
	s.Old, s.New = old, next
	s.undo = nil
	s.Current = 0
	s.Scroll = ScrollState{}
	s.ready = false
}
func (s *DiffState) compute() {
	if s.ready && s.oldText == s.Old && s.newText == s.New {
		return
	}
	s.oldText, s.newText, s.ready = s.Old, s.New, true
	a, b := diffLines(s.Old), diffLines(s.New)
	s.oldLines, s.newLines = a, b
	s.hunks = nil
	s.rows = nil
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	ae, be := len(a), len(b)
	for ae > prefix && be > prefix && a[ae-1] == b[be-1] {
		ae--
		be--
	}
	for i := 0; i < prefix; i++ {
		s.rows = append(s.rows, diffRow{i, i})
	}
	add := func(os, oe, ns, ne int) {
		if os == oe && ns == ne {
			return
		}
		s.hunks = append(s.hunks, DiffHunk{os, oe, ns, ne})
		for k := 0; k < max(oe-os, ne-ns); k++ {
			r := diffRow{-1, -1}
			if os+k < oe {
				r.old = os + k
			}
			if ns+k < ne {
				r.new = ns + k
			}
			s.rows = append(s.rows, r)
		}
	}
	n, m := ae-prefix, be-prefix
	if n > 0 && m > 0 && n <= 1000000/m {
		width := m + 1
		table := make([]uint32, (n+1)*width)
		for i := n - 1; i >= 0; i-- {
			for j := m - 1; j >= 0; j-- {
				at := i*width + j
				if a[prefix+i] == b[prefix+j] {
					table[at] = 1 + table[at+width+1]
				} else {
					table[at] = max(table[at+width], table[at+1])
				}
			}
		}
		i, j, os, ns := 0, 0, prefix, prefix
		for i < n && j < m {
			if a[prefix+i] == b[prefix+j] {
				add(os, prefix+i, ns, prefix+j)
				s.rows = append(s.rows, diffRow{prefix + i, prefix + j})
				i++
				j++
				os, ns = prefix+i, prefix+j
			} else if table[(i+1)*width+j] >= table[i*width+j+1] {
				i++
			} else {
				j++
			}
		}
		add(os, ae, ns, be)
	} else {
		add(prefix, ae, prefix, be)
	}
	for ae < len(a) && be < len(b) {
		s.rows = append(s.rows, diffRow{ae, be})
		ae++
		be++
	}
	s.Current = max(0, min(s.Current, len(s.hunks)-1))
}
func (s *DiffState) Hunks() []DiffHunk { s.compute(); return append([]DiffHunk(nil), s.hunks...) }

// Revert restores one new-side hunk from the old text, retaining exact line endings.
func (s *DiffState) Revert(index int) bool {
	s.compute()
	if index < 0 || index >= len(s.hunks) {
		return false
	}
	h := s.hunks[index]
	s.undo = append(s.undo, s.New)
	if len(s.undo) > 100 {
		s.undo = s.undo[1:]
	}
	s.New = strings.Join(s.newLines[:h.NewStart], "") + strings.Join(s.oldLines[h.OldStart:h.OldEnd], "") + strings.Join(s.newLines[h.NewEnd:], "")
	s.compute()
	return true
}
func (s *DiffState) Undo() bool {
	if len(s.undo) == 0 {
		return false
	}
	s.New = s.undo[len(s.undo)-1]
	s.undo = s.undo[:len(s.undo)-1]
	s.compute()
	return true
}
func (s *DiffState) move(delta int) {
	s.compute()
	if len(s.hunks) == 0 {
		return
	}
	s.Current = (s.Current + delta + len(s.hunks)) % len(s.hunks)
	h := s.hunks[s.Current]
	line := 0
	for _, r := range s.rows {
		if r.old == h.OldStart || r.new == h.NewStart {
			break
		}
		line++
		if s.Unified && r.old >= 0 && r.new >= 0 && s.oldLines[r.old] != s.newLines[r.new] {
			line++
		}
	}
	s.Scroll.Y = float64(line) * 24
}

// DiffView paints visible lines only. Both sides share a scroll viewport and line grid.
// It is a read-only comparison; changes are made through explicit revert actions.
func DiffView(id string, s *DiffState, height float64, onChange func(string)) *Element {
	if s == nil {
		panic("baseui: nil diff state")
	}
	if !finite(height) || height <= 0 {
		height = 280
	}
	s.compute()
	changed := func() {
		if onChange != nil {
			onChange(s.New)
		}
	}
	toolbar := Row(id+"/actions",
		Button(id+"/previous", "Previous", func() { s.move(-1) }).Variant(Outline).Disabled(len(s.hunks) == 0),
		Button(id+"/next", "Next", func() { s.move(1) }).Variant(Outline).Disabled(len(s.hunks) == 0),
		Button(id+"/layout", "Unified / split", func() { s.Unified = !s.Unified; s.Scroll.Y = 0 }).Variant(Ghost),
		Button(id+"/revert", "Revert hunk", func() {
			if s.Revert(s.Current) {
				changed()
			}
		}).Variant(Outline).Disabled(len(s.hunks) == 0),
		Button(id+"/undo", "Undo", func() {
			if s.Undo() {
				changed()
			}
		}).Variant(Ghost).Disabled(len(s.undo) == 0))
	// Wrap the actions so narrow native windows preserve every action.
	toolbar.Wrap(true).Gap(6)
	rows := append([]diffRow(nil), s.rows...)
	a, b := s.oldLines, s.newLines
	unified := s.Unified
	if unified {
		rows = nil
		for _, r := range s.rows {
			if r.old >= 0 && r.new >= 0 && a[r.old] != b[r.new] {
				rows = append(rows, diffRow{r.old, -1}, diffRow{-1, r.new})
			} else {
				rows = append(rows, r)
			}
		}
	}
	canvas := Canvas(id+"/lines", "Line differences", func(p *Painter, r Rect) {
		theme := p.Theme()
		start := max(0, int(s.Scroll.Y/24))
		end := min(len(rows), start+int(height/24)+3)
		for i := start; i < end; i++ {
			row := rows[i]
			y := r.Y + float64(i)*24
			different := row.old < 0 || row.new < 0 || (a[row.old] != b[row.new])
			cell := func(x, w float64, number int, lines []string, sign string) {
				box := Rect{X: x, Y: y, W: w, H: 24}
				if different {
					c := color.NRGBA{R: 254, G: 226, B: 226, A: 255}
					if sign == "+" {
						c = color.NRGBA{R: 220, G: 252, B: 231, A: 255}
					}
					if theme.Background.R < 128 {
						c = color.NRGBA{R: 69, G: 26, B: 26, A: 255}
						if sign == "+" {
							c = color.NRGBA{R: 20, G: 55, B: 38, A: 255}
						}
					}
					p.Fill(box, c, 0)
				}
				if number < 0 {
					return
				}
				text := strings.TrimRight(lines[number], "\r\n")
				c := theme.Text

				q := *p
				q.clip = intersect(p.clip, box)
				q.StyledText(x+8, y+17, fmt.Sprintf("%s %4d  %s", sign, number+1, text), TextSpan{Monospace: true}, 12, c)
			}
			if unified {
				if row.new >= 0 {
					sign := " "
					if different {
						sign = "+"
					}
					cell(r.X, r.W, row.new, b, sign)
				} else {
					cell(r.X, r.W, row.old, a, "-")
				}
			} else {
				w := (r.W - 12) / 2
				left, right := " ", " "
				if different {
					left, right = "-", "+"
				}
				cell(r.X, w, row.old, a, left)
				cell(r.X+w+12, w, row.new, b, right)
			}
		}
	}).Height(max(24, float64(len(rows))*24))

	return Column(id, toolbar, Text(id+"/summary", fmt.Sprintf("%d changes · old / new", len(s.hunks))).Muted(true), ScrollArea(id+"/scroll", &s.Scroll, canvas).HorizontalScroll(true).Height(height))
}
