package terminal

import (
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strings"
)

type ViewOptions struct {
	FontSize float64
	OnInput  func([]byte)
}

// View snapshots the screen for native Go painting. Bind OnInput to a writer
// owned by the application; Feed must receive PTY output through UI dispatch.
func View(id string, screen *Screen, o ViewOptions) *ui.Element {
	if screen == nil {
		panic("terminal: nil screen")
	}
	if o.FontSize == 0 {
		o.FontSize = 13
	}
	advance, height, baseline := ui.MonospaceMetrics(o.FontSize)
	if advance == 0 {
		panic("terminal: invalid font size")
	}
	columns, rows := screen.Size()
	cells := screen.Cells()
	cx, cy, cursor := screen.Cursor()
	e := ui.Canvas(id, "Terminal screen", func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, p.Theme().Surface, 0)
		for y := 0; y < rows; y++ {
			for x := 0; x < columns; {
				cell := cells[y*columns+x]
				fg, bg := cell.Foreground, cell.Background
				if fg.A == 0 {
					fg = p.Theme().Text
				}
				if bg.A == 0 {
					bg = p.Theme().Surface
				}
				if cell.Inverse {
					fg, bg = bg, fg
				}
				end := x + 1
				for end < columns {
					next := cells[y*columns+end]
					if next.Foreground != cell.Foreground || next.Background != cell.Background || next.Inverse != cell.Inverse || next.Bold != cell.Bold {
						break
					}
					end++
				}
				var run strings.Builder
				for n := x; n < end; n++ {
					c := cells[y*columns+n].Rune
					if c == 0 {
						c = ' '
					}
					run.WriteRune(c)
				}
				box := ui.Rect{X: r.X + float64(x)*advance, Y: r.Y + float64(y)*height, W: float64(end-x) * advance, H: height}
				p.Fill(box, bg, 0)
				p.StyledText(box.X, box.Y+baseline, run.String(), ui.TextSpan{Monospace: true, Bold: cell.Bold}, o.FontSize, fg)
				x = end
			}
		}
		if cursor {
			p.Stroke(ui.Rect{X: r.X + float64(cx)*advance, Y: r.Y + float64(cy)*height, W: advance, H: height}, p.Theme().Focus, 0, 1)
		}
	}).Width(float64(columns) * advance).Height(float64(rows) * height).Role("terminal").Description(screen.Text())
	if o.OnInput != nil {
		e.OnTextInput(func(value string) { o.OnInput([]byte(value)) })
		e.OnKey(func(ev ui.KeyEvent) bool {
			if ev.Up || ev.Command {
				return false
			}
			codes := map[string]string{"Enter": "\r", "Backspace": "\x7f", "Escape": "\x1b", "Up": "\x1b[A", "Down": "\x1b[B", "Right": "\x1b[C", "Left": "\x1b[D", "Home": "\x1b[H", "End": "\x1b[F", "Delete": "\x1b[3~", "PageUp": "\x1b[5~", "PageDown": "\x1b[6~"}
			if value, ok := codes[ev.Key]; ok {
				o.OnInput([]byte(value))
				return true
			}
			return false
		})
	}
	return e
}
