package baseui

import (
	"fmt"
	"math"
	"strings"
)

// Rating uses one focus stop. Re-selecting the current star clears the rating.
func Rating(id, label string, value, maximum int, onChange func(int)) *Element {
	if maximum < 1 || maximum > 100 || value < 0 || value > maximum {
		panic("baseui: invalid rating")
	}
	set := func(v int) {
		v = max(0, min(maximum, v))
		if v != value && onChange != nil {
			onChange(v)
		}
	}
	e := Canvas(id, label, func(p *Painter, r Rect) {
		cell := r.W / float64(maximum)
		for n := 0; n < maximum; n++ {
			cx, cy, radius := r.X+(float64(n)+.5)*cell, r.Y+r.H/2, min(cell, r.H)*.43
			points := make([][2]float64, 10)
			for k := range points {
				rad := radius
				if k%2 == 1 {
					rad *= .43
				}
				a := -math.Pi/2 + float64(k)*math.Pi/5
				points[k] = [2]float64{cx + math.Cos(a)*rad, cy + math.Sin(a)*rad}
			}
			c := p.theme.Border
			if n < value {
				c = p.theme.Accent
			}
			p.raster(Rect{cx - radius, cy - radius, 2 * radius, 2 * radius}, c, func(x, y float64) float64 {
				inside := false
				for i, j := 0, len(points)-1; i < len(points); j, i = i, i+1 {
					a, b := points[i], points[j]
					if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
						inside = !inside
					}
				}
				if inside {
					return -1
				}
				return 1
			})
		}
	}).Width(float64(maximum) * 28).Height(32).Role("slider")
	e.value, e.min, e.max, e.step = float64(value), 0, float64(maximum), 1
	e.onPointer = func(ev PointerEvent, r, viewport Rect) {
		n := max(1, min(maximum, int((ev.X-r.X)/max(1, r.W)*float64(maximum))+1))
		switch ev.Kind {
		case "up":
			if r.Contains(ev.X, ev.Y) {
				if n == value {
					set(0)
				} else {
					set(n)
				}
			}
		}
	}
	e.OnKey(func(ev KeyEvent) bool {
		switch ev.Key {
		case "Left", "Down":
			delta := -1
			if ev.Key == "Left" && ev.RTL {
				delta = 1
			}
			set(value + delta)
		case "Right", "Up":
			delta := 1
			if ev.Key == "Right" && ev.RTL {
				delta = -1
			}
			set(value + delta)
		case "Home":
			set(0)
		case "End":
			set(maximum)
		default:
			return false
		}
		return true
	})
	return e
}

type TokenFieldState struct {
	Tokens   []string
	Text     *TextBuffer
	selected int
}
type TokenFieldOptions struct {
	Suggestions []string
	MaxTokens   int
	OnChange    func([]string)
}

func TokenField(id, label string, state *TokenFieldState, o TokenFieldOptions) *Element {
	if state == nil || o.MaxTokens < 0 {
		panic("baseui: invalid token field")
	}
	if state.Text == nil {
		state.Text = NewTextBuffer("")
	}
	notify := func() {
		if o.OnChange != nil {
			o.OnChange(append([]string(nil), state.Tokens...))
		}
	}
	add := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		for _, t := range state.Tokens {
			if t == token {
				return
			}
		}
		if o.MaxTokens > 0 && len(state.Tokens) >= o.MaxTokens {
			return
		}
		state.Tokens = append(append([]string(nil), state.Tokens...), token)
		state.Text.Set("")
		state.selected = 0
		notify()
	}
	remove := func(index int) {
		if index < 0 || index >= len(state.Tokens) {
			return
		}
		next := append([]string(nil), state.Tokens[:index]...)
		state.Tokens = append(next, state.Tokens[index+1:]...)
		notify()
	}
	query := strings.ToLower(strings.TrimSpace(state.Text.String()))
	matches := []string{}
	seen := map[string]bool{}
	for _, t := range state.Tokens {
		seen[t] = true
	}
	for _, t := range o.Suggestions {
		if !seen[t] && strings.Contains(strings.ToLower(t), query) && len(matches) < 20 {
			matches = append(matches, t)
			seen[t] = true
		}
	}
	state.selected = max(0, min(state.selected, len(matches)-1))
	input := Input(id+"/input", label, state.Text).Placeholder("Add a token")
	input.OnKey(func(ev KeyEvent) bool {
		if state.Text.Composition() != "" {
			return false
		}
		switch ev.Key {
		case "Down":
			if len(matches) > 0 {
				state.selected = (state.selected + 1) % len(matches)
			}
		case "Up":
			if len(matches) > 0 {
				state.selected = (state.selected + len(matches) - 1) % len(matches)
			}
		case "Enter":
			if query != "" && len(matches) > 0 {
				add(matches[state.selected])
			} else {
				add(state.Text.String())
			}
		case "Backspace":
			if state.Text.String() != "" {
				return false
			}
			remove(len(state.Tokens) - 1)
		default:
			return false
		}
		return true
	})
	input.onChange = func(s string) {
		if strings.HasSuffix(s, ",") {
			add(strings.TrimSuffix(s, ","))
		}
	}
	children := []*Element{}
	for i, token := range append([]string(nil), state.Tokens...) {
		i := i
		children = append(children, Row(fmt.Sprintf("%s/token/%d", id, i), Badge(fmt.Sprintf("%s/value/%d", id, i), token), Button(fmt.Sprintf("%s/remove/%d", id, i), "Remove", func() { remove(i) }).Variant(Ghost).Label("Remove "+token)))
	}
	children = append(children, input)
	if query != "" {
		for i, match := range matches {
			i, match := i, match
			button := Button(fmt.Sprintf("%s/suggestion/%d", id, i), match, func() { add(match) }).Variant(Ghost)
			button.selected = i == state.selected
			children = append(children, button)
		}
	}
	return Column(id, children...).Role("group").Label(label)
}

type ToolbarItem struct {
	ID, Label                 string
	Disabled, Toggle, Checked bool
	Action                    func()
}

// Toolbar provides roving keyboard focus for actions and toggles. Automatic
// width-based overflow and arbitrary embedded widgets are not implemented yet.
func Toolbar(id, label string, items []ToolbarItem) *Element {
	items = append([]ToolbarItem(nil), items...)
	options := make([]Option, len(items))
	for i, item := range items {
		options[i] = Option{ID: item.ID, Label: item.Label, Disabled: item.Disabled || item.Action == nil}
	}
	validateOptions(options)
	children := []*Element{}
	for i, item := range items {
		i, item := i, item
		b := Button(id+"/"+item.ID, item.Label, item.Action).Variant(Ghost).Disabled(options[i].Disabled)
		b.focusGroup = id
		if item.Toggle {
			b.role = "checkbox"
			b.checked = item.Checked
		}
		b.OnKey(func(ev KeyEvent) bool {
			n := optionKey(ev, i, options)
			if n < 0 {
				return false
			}
			if ev.RequestFocus != nil {
				ev.RequestFocus(id + "/" + items[n].ID)
			}
			return true
		})
		children = append(children, b)
	}
	return Row(id, children...).Gap(4).Role("toolbar").Label(label)
}
