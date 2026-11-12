package baseui

// DatePickerState uses an explicit initial visible month, never the wall clock.
type DatePickerState struct {
	Calendar       *CalendarState
	Open, HasValue bool
}

func NewDatePickerState(visibleDate Date) *DatePickerState {
	calendar := NewCalendarState(visibleDate)
	calendar.Value = Date{}
	return &DatePickerState{Calendar: calendar}
}

// DatePicker combines an anchored, dismissible calendar with an accessible
// trigger. Single dates close on selection; ranges close after the second date.
func DatePicker(id, label string, state *DatePickerState, options CalendarOptions) *Element {
	if state == nil || state.Calendar == nil {
		panic("baseui: nil date picker state")
	}
	text := "Pick a date"
	if state.HasValue {
		text = state.Calendar.Value.String()
		if options.Mode == DateRange {
			text = state.Calendar.Start.String()
			if state.Calendar.End.Valid() {
				text += " — " + state.Calendar.End.String()
			}
		}
	}
	trigger := Button(id+"/trigger", text, func() { state.Open = !state.Open }).Variant(Outline).Label(label).Role("combobox")
	trigger.alignStart = true
	trigger.expanded = state.Open
	trigger.controls = id + "/popup"
	trigger.OnKey(func(ev KeyEvent) bool {
		if ev.Key != "Down" {
			return false
		}
		state.Open = true
		return true
	})
	changed := options.OnChange
	options.OnChange = func(date Date) {
		state.HasValue = true
		if options.Mode == SingleDate || options.Mode == DateRange && state.Calendar.End.Valid() {
			state.Open = false
		}
		if changed != nil {
			changed(date)
		}
	}
	cal := Calendar(id+"/calendar", state.Calendar, options).Padding(12)
	cal.flat = true
	popup := Popover(id+"/popup", id+"/trigger", state.Open, func() { state.Open = false }, cal).Width(280).Padding(0).AutoFocus(id + "/calendar/date/" + state.Calendar.Cursor.String())
	return Column(id, trigger, popup).Gap(0)
}
