package baseui

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/herlanai/omnigo/omniui/baseui/internal/diff"
)

// DiffHunk uses zero-based, half-open line ranges. Line endings are preserved.
type DiffHunk struct{ OldStart, OldEnd, NewStart, NewEnd int }

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
	rows               []diff.Row
	unifiedRows        []diff.Row
	undo               []string
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
	result := diff.Compare(s.Old, s.New)
	s.oldLines, s.newLines = result.OldLines, result.NewLines
	s.hunks = make([]DiffHunk, len(result.Hunks))
	for i, h := range result.Hunks {
		s.hunks[i] = DiffHunk{h.OldStart, h.OldEnd, h.NewStart, h.NewEnd}
	}
	s.rows = result.Rows
	s.unifiedRows = nil
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
		if r.Old == h.OldStart || r.New == h.NewStart {
			break
		}
		line++
		if s.Unified && r.Old >= 0 && r.New >= 0 && s.oldLines[r.Old] != s.newLines[r.New] {
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
	rows := s.rows
	a, b := s.oldLines, s.newLines
	unified := s.Unified
	if unified {
		if s.unifiedRows == nil {
			mapped := make([]diff.Row, 0, len(s.rows))
			for _, r := range s.rows {
				if r.Old >= 0 && r.New >= 0 && a[r.Old] != b[r.New] {
					mapped = append(mapped, diff.Row{Old: r.Old, New: -1}, diff.Row{Old: -1, New: r.New})
				} else {
					mapped = append(mapped, r)
				}
			}
			s.unifiedRows = mapped
		}
		rows = s.unifiedRows
	}
	canvas := Canvas(id+"/lines", "Line differences", func(p *Painter, r Rect) {
		theme := p.Theme()
		start := max(0, int(s.Scroll.Y/24))
		end := min(len(rows), start+int(height/24)+3)
		for i := start; i < end; i++ {
			row := rows[i]
			y := r.Y + float64(i)*24
			different := row.Old < 0 || row.New < 0 || (a[row.Old] != b[row.New])
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
				if row.New >= 0 {
					sign := " "
					if different {
						sign = "+"
					}
					cell(r.X, r.W, row.New, b, sign)
				} else {
					cell(r.X, r.W, row.Old, a, "-")
				}
			} else {
				w := (r.W - 12) / 2
				left, right := " ", " "
				if different {
					left, right = "-", "+"
				}
				cell(r.X, w, row.Old, a, left)
				cell(r.X+w+12, w, row.New, b, right)
			}
		}
	}).Height(max(24, float64(len(rows))*24))

	return Column(id, toolbar, Text(id+"/summary", fmt.Sprintf("%d changes · old / new", len(s.hunks))).Muted(true), ScrollArea(id+"/scroll", &s.Scroll, canvas).HorizontalScroll(true).Height(height))
}
