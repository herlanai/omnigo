package baseui

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// NumberInputState keeps incomplete numeric text separate from the committed value.
// Mutate it only on the UI goroutine. An external Value change replaces the draft.
type NumberInputState struct {
	Value float64
	Text  *TextBuffer
	last  float64
}
type NumberInputOptions struct {
	Min, Max, Step float64
	OnChange       func(float64)
}

func NumberInput(id, label string, state *NumberInputState, o NumberInputOptions) *Element {
	if state == nil || !finite(state.Value) || !finite(o.Min) || !finite(o.Max) || !finite(o.Step) || o.Min > o.Max || o.Step <= 0 {
		panic("baseui: invalid number input")
	}
	format := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	if state.Text == nil || state.Value != state.last {
		state.Text = NewTextBuffer(format(state.Value))
		state.last = state.Value
	}
	set := func(v float64, normalize bool) {
		v = math.Max(o.Min, math.Min(o.Max, v))
		if !finite(v) {
			return
		}
		changed := v != state.Value
		state.Value = v
		state.last = v
		if normalize {
			state.Text.Set(format(v))
		}
		if changed && o.OnChange != nil {
			o.OnChange(v)
		}
	}
	step := func(delta float64) {
		v := state.Value + delta
		// Stabilize ordinary decimal steps without rounding arbitrary typed precision.
		scale := math.Pow10(min(12, max(0, -int(math.Floor(math.Log10(o.Step)))+6)))
		if finite(v * scale) {
			v = math.Round(v*scale) / scale
		}
		set(v, true)
	}
	input := Input(id+"/input", label, state.Text).Grow(1).Role("spinbutton")
	input.value, input.min, input.max, input.step = state.Value, o.Min, o.Max, o.Step
	input.onChange = func(s string) {
		if v, e := strconv.ParseFloat(strings.TrimSpace(s), 64); e == nil && finite(v) && v >= o.Min && v <= o.Max {
			set(v, false)
		}
	}
	input.onKey = func(ev KeyEvent) bool {
		if state.Text.Composition() != "" {
			return false
		}
		switch ev.Key {
		case "Up":
			step(o.Step)
		case "Down":
			step(-o.Step)
		case "Enter", "Escape":
			state.Text.Set(format(state.Value))
		default:
			return false
		}
		return true
	}
	_, err := strconv.ParseFloat(strings.TrimSpace(state.Text.String()), 64)
	v, _ := strconv.ParseFloat(strings.TrimSpace(state.Text.String()), 64)
	input.Invalid(err != nil || !finite(v) || v < o.Min || v > o.Max)
	root := Row(id, input, Button(id+"/decrease", "−", func() { step(-o.Step) }).Label(label+" decrease").Variant(Outline).Size(ControlSmall).Width(32).Disabled(state.Value <= o.Min), Button(id+"/increase", "+", func() { step(o.Step) }).Label(label+" increase").Variant(Outline).Size(ControlSmall).Width(32).Disabled(state.Value >= o.Max)).Role("group").Label(label).Gap(4)
	root.centerItems = true
	return root
}

type DateInputState struct {
	Value    time.Time
	Open     bool
	calendar *CalendarState
}
type DateInputOptions struct {
	Layout    string
	WeekStart time.Weekday
	Disabled  func(Date) bool
	OnChange  func(time.Time)
}

func DateInput(id, label string, state *DateInputState, o DateInputOptions) *Element {
	if state == nil || !DateOf(state.Value).Valid() {
		panic("baseui: invalid date input")
	}
	layout := o.Layout
	if layout == "" {
		layout = "2006-01-02"
	}
	if state.calendar == nil || !state.Open {
		state.calendar = NewCalendarState(DateOf(state.Value))
	}
	anchor := Button(id+"/trigger", state.Value.Format(layout), func() { state.Open = !state.Open }).Label(label).Variant(Outline).Role("combobox")
	anchor.expanded = state.Open
	anchor.controls = id + "/popup"
	calendar := Calendar(id+"/calendar", state.calendar, CalendarOptions{WeekStart: o.WeekStart, Disabled: o.Disabled, OnChange: func(d Date) {
		old := state.Value
		state.Value = time.Date(d.Year, d.Month, d.Day, old.Hour(), old.Minute(), old.Second(), old.Nanosecond(), old.Location())
		state.Open = false
		if !state.Value.Equal(old) && o.OnChange != nil {
			o.OnChange(state.Value)
		}
	}})
	return Column(id, anchor, Popover(id+"/popup", id+"/trigger", state.Open, func() { state.Open = false }, calendar).Width(304).AutoFocus(id+"/calendar/date/"+state.calendar.Cursor.String())).Gap(0)
}

type TimeInputState struct {
	Value          time.Time
	hours, minutes NumberInputState
}
type TimeInputOptions struct {
	TwelveHour bool
	OnChange   func(time.Time)
}

func TimeInput(id, label string, state *TimeInputState, o TimeInputOptions) *Element {
	if state == nil {
		panic("baseui: nil time input")
	}
	set := func(h, m int) {
		old := state.Value
		y, mo, d := old.Date()
		next := time.Date(y, mo, d, h, m, old.Second(), old.Nanosecond(), old.Location())
		state.Value = next
		if !next.Equal(old) && o.OnChange != nil {
			o.OnChange(next)
		}
	}
	h := state.Value.Hour()
	lo, hi := 0., 23.
	if o.TwelveHour {
		h %= 12
		if h == 0 {
			h = 12
		}
		lo, hi = 1, 12
	}
	state.hours.Value = float64(h)
	state.minutes.Value = float64(state.Value.Minute())
	hours := NumberInput(id+"/hours", label+" hours", &state.hours, NumberInputOptions{Min: lo, Max: hi, Step: 1, OnChange: func(v float64) {
		hour := int(v)
		if o.TwelveHour {
			hour = hour%12 + state.Value.Hour()/12*12
		}
		set(hour, state.Value.Minute())
	}})
	minutes := NumberInput(id+"/minutes", label+" minutes", &state.minutes, NumberInputOptions{Min: 0, Max: 59, Step: 1, OnChange: func(v float64) { set(state.Value.Hour(), int(v)) }})
	// Time segments use only the text fields; keyboard arrows step them.
	hours.children = hours.children[:1]
	minutes.children = minutes.children[:1]
	// Segments wrap without carrying into the other segment or date.
	for index, segment := range []*Element{hours, minutes} {
		index := index
		input := segment.children[0]
		previous := input.onKey
		input.onKey = func(ev KeyEvent) bool {
			if input.buffer.Composition() != "" {
				return false
			}
			delta := 0
			if ev.Key == "Up" {
				delta = 1
			}
			if ev.Key == "Down" {
				delta = -1
			}
			if delta != 0 {
				if index == 0 {
					set((state.Value.Hour()+delta+24)%24, state.Value.Minute())
				} else {
					set(state.Value.Hour(), (state.Value.Minute()+delta+60)%60)
				}
				return true
			}
			if ev.Key == "Left" || ev.Key == "Right" {
				target := id + "/hours/input"
				if ev.Key == "Right" {
					target = id + "/minutes/input"
				}
				if ev.RequestFocus != nil {
					ev.RequestFocus(target)
				}
				return true
			}
			return previous(ev)
		}
		// Typed fractions are invalid for time segments.
		previousChange := input.onChange
		input.onChange = func(s string) {
			if v, e := strconv.Atoi(strings.TrimSpace(s)); e == nil {
				previousChange(strconv.Itoa(v))
			}
		}
	}
	children := []*Element{hours.Grow(1), Text(id+"/colon", ":"), minutes.Grow(1)}
	if o.TwelveHour {
		period := "AM"
		if state.Value.Hour() >= 12 {
			period = "PM"
		}
		children = append(children, Button(id+"/period", period, func() { set((state.Value.Hour()+12)%24, state.Value.Minute()) }).Label(label+" AM/PM").Variant(Secondary))
	}
	root := Row(id, children...).Role("group").Label(label).Width(240)
	root.centerItems = true
	return root
}

// SearchField preserves TextBuffer editing, IME and history while providing clear
// and explicit submit actions. Filtering remains the application's responsibility.
func SearchField(id, label string, text *TextBuffer, onSearch func(string)) *Element {
	if text == nil {
		panic("baseui: nil search buffer")
	}
	input := Input(id+"/input", label, text).Role("searchbox").Grow(1).Placeholder("Search")
	input.OnSubmit(func() {
		if text.Composition() == "" && onSearch != nil {
			onSearch(text.String())
		}
	})
	input.OnKey(func(ev KeyEvent) bool {
		if ev.Key == "Escape" && text.Composition() == "" {
			text.Set("")
			return true
		}
		return false
	})
	icon := Canvas(id+"/icon", "", func(p *Painter, r Rect) {
		p.Stroke(Rect{X: r.X + 2, Y: r.Y + 2, W: 10, H: 10}, p.Theme().MutedText, 5, 1.5)
		p.Line(r.X+11, r.Y+11, r.X+17, r.Y+17, 1.5, p.Theme().MutedText)
	}).Width(20).Height(20)
	return InputGroup(id, icon, input, Button(id+"/clear", "Clear", func() { text.Set("") }).Variant(Ghost).Disabled(text.String() == "")).Role("search")
}
