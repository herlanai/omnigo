package baseui

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"
	"unicode/utf8"
)

type ImageFit uint8

const (
	ImageContain ImageFit = iota
	ImageCover
	ImageStretch
)

type ImageOptions struct {
	Width, Height float64
	Fit           ImageFit
}

// Image copies caller-owned pixels once at construction. There is no hidden IO
// or decoding dependency. SVG, animation and color profiles remain separate work.
func Image(id, label string, source image.Image, o ImageOptions) *Element {
	if source == nil || source.Bounds().Empty() || o.Fit > ImageStretch {
		panic("baseui: invalid image")
	}
	b := source.Bounds()
	if b.Dx() > 4096 || b.Dy() > 4096 || b.Dx()*b.Dy() > 4194304 {
		panic("baseui: image budget exceeded")
	}
	snapshot := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(snapshot, snapshot.Bounds(), source, b.Min, draw.Src)
	if o.Width == 0 {
		o.Width = float64(b.Dx())
	}
	if o.Height == 0 {
		o.Height = float64(b.Dy())
	}
	if !finite(o.Width) || !finite(o.Height) || o.Width <= 0 || o.Height <= 0 {
		panic("baseui: invalid image size")
	}
	return Canvas(id, label, func(p *Painter, r Rect) {
		box := r
		if o.Fit != ImageStretch {
			scale := math.Min(r.W/float64(b.Dx()), r.H/float64(b.Dy()))
			if o.Fit == ImageCover {
				scale = math.Max(r.W/float64(b.Dx()), r.H/float64(b.Dy()))
			}
			box.W, box.H = float64(b.Dx())*scale, float64(b.Dy())*scale
			box.X = r.X + (r.W-box.W)/2
			box.Y = r.Y + (r.H-box.H)/2
		}
		q := *p
		q.clip = intersect(p.clip, r)
		q.Image(box, snapshot, 0)
	}).Width(o.Width).Height(o.Height).Role("img")
}

// Grid lays out equal-width columns without mutating caller-owned children.
// Explicit tracks, spanning, responsive column counts and virtualization are not
// implemented here; this is a finite grid of already materialized elements.
func Grid(id string, columns int, gap float64, children ...*Element) *Element {
	if columns < 1 || columns > 128 || !finite(gap) || gap < 0 {
		panic("baseui: invalid grid")
	}
	rows := []*Element{}
	for start := 0; start < len(children); start += columns {
		cells := []*Element{}
		for col := 0; col < columns; col++ {
			key := fmt.Sprintf("%s/cell/%d", id, start+col)
			if start+col < len(children) {
				if children[start+col] == nil {
					panic("baseui: nil grid child")
				}
				cells = append(cells, Column(key, children[start+col]).Grow(1))
			} else {
				cells = append(cells, Column(key).Grow(1))
			}
		}
		rows = append(rows, Row(fmt.Sprintf("%s/row/%d", id, start/columns), cells...).Gap(gap))
	}
	return Column(id, rows...).Gap(gap).Role("group")
}

// CheckboxGroup includes a mixed-state aggregate checkbox. Disabled options keep
// their values and are excluded from the aggregate's all/none action.
func CheckboxGroup(id, label string, selected map[string]bool, options []Option, onChange func(map[string]bool)) *Element {
	validateOptions(options)
	options = append([]Option(nil), options...)
	values := map[string]bool{}
	for k, v := range selected {
		values[k] = v
	}
	count, total := 0, 0
	for _, o := range options {
		if !o.Disabled {
			total++
			if values[o.ID] {
				count++
			}
		}
	}
	emit := func(next map[string]bool) {
		if onChange != nil {
			onChange(next)
		}
	}
	clone := func() map[string]bool {
		m := map[string]bool{}
		for k, v := range values {
			m[k] = v
		}
		return m
	}
	aggregate := Unchecked
	if count > 0 {
		aggregate = Mixed
	}
	if total > 0 && count == total {
		aggregate = Checked
	}
	all := MixedCheckbox(id+"/all", label, aggregate, func(CheckState) {
		next := clone()
		for _, o := range options {
			if !o.Disabled {
				next[o.ID] = count != total
			}
		}
		emit(next)
	}).Disabled(total == 0)
	children := []*Element{all}
	for _, o := range options {
		o := o
		children = append(children, Checkbox(id+"/"+o.ID, o.Label, values[o.ID], func(v bool) { next := clone(); next[o.ID] = v; emit(next) }).Disabled(o.Disabled))
	}
	return Column(id, children...).Role("group").Label(label)
}

type EditableTextState struct {
	Value   string
	Editing bool
	draft   *TextBuffer
}

// EditableText commits with Enter or Save and cancels with Escape or Cancel.
// Delayed row renaming, double-click and focus-loss commit remain unimplemented.
func EditableText(id, label string, state *EditableTextState, onChange func(string)) *Element {
	if state == nil {
		panic("baseui: nil editable text")
	}
	begin := func() {
		state.Editing = true
		state.draft = NewTextBuffer(state.Value)
		end := utf8.RuneCountInString(state.Value)
		if dot := strings.LastIndex(state.Value, "."); dot > 0 {
			end = utf8.RuneCountInString(state.Value[:dot])
		}
		state.draft.Select(0, end)
	}
	cancel := func() { state.Editing = false; state.draft = nil }
	if !state.Editing {
		return Button(id+"/input", state.Value, begin).Variant(Ghost).Label(label).OnKey(func(ev KeyEvent) bool {
			if ev.Key == "F2" {
				begin()
				return true
			}
			return false
		})
	}
	if state.draft == nil {
		begin()
	}
	commit := func() {
		if state.draft.Composition() != "" {
			return
		}
		next := state.draft.String()
		changed := next != state.Value
		state.Value = next
		cancel()
		if changed && onChange != nil {
			onChange(next)
		}
	}
	input := Input(id+"/input", label, state.draft).Grow(1).OnSubmit(commit)
	input.OnKey(func(ev KeyEvent) bool {
		if ev.Key == "Escape" && state.draft.Composition() == "" {
			cancel()
			return true
		}
		return false
	})
	return Row(id, input, Button(id+"/save", "Save", commit), Button(id+"/cancel", "Cancel", cancel).Variant(Ghost))
}

type FindBarState struct {
	Open    bool
	Query   *TextBuffer
	Current int
}
type FindBarOptions struct {
	Matches  int
	OnSelect func(int)
	OnQuery  func(string)
	OnClose  func()
}

func FindBar(id string, state *FindBarState, o FindBarOptions) *Element {
	if state == nil || o.Matches < 0 {
		panic("baseui: invalid find bar")
	}
	if state.Query == nil {
		state.Query = NewTextBuffer("")
	}
	state.Current = max(0, min(state.Current, max(0, o.Matches-1)))
	close := func() {
		state.Open = false
		if o.OnClose != nil {
			o.OnClose()
		}
	}
	move := func(delta int) {
		if o.Matches == 0 {
			return
		}
		state.Current = (state.Current + delta + o.Matches) % o.Matches
		if o.OnSelect != nil {
			o.OnSelect(state.Current)
		}
	}
	input := Input(id+"/query", "Find", state.Query).Grow(1).Role("searchbox")
	input.onChange = func(value string) {
		state.Current = 0
		if o.OnQuery != nil {
			o.OnQuery(value)
		}
	}
	input.OnKey(func(ev KeyEvent) bool {
		if state.Query.Composition() != "" {
			return false
		}
		if ev.Key == "Escape" {
			close()
			return true
		}
		if ev.Key == "Enter" || ev.Key == "F3" || (ev.Command && strings.EqualFold(ev.Key, "g")) {
			delta := 1
			if ev.Shift {
				delta = -1
			}
			move(delta)
			return true
		}
		return false
	})
	current := 0
	if o.Matches > 0 {
		current = state.Current + 1
	}
	return Row(id, input, Text(id+"/count", fmt.Sprintf("%d / %d", current, o.Matches)).Role("status"), Button(id+"/previous", "Previous", func() { move(-1) }).Disabled(o.Matches == 0), Button(id+"/next", "Next", func() { move(1) }).Disabled(o.Matches == 0), Button(id+"/done", "Done", close)).Hidden(!state.Open).Role("search")
}
