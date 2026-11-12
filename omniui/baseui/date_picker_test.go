package baseui

import "testing"

func TestDatePickerSelectionDismissalAndReadOnly(t *testing.T) {
	state := NewDatePickerState(Date{2026, 11, 12})
	s, _ := NewSurface(500, 480, NewTheme(false))
	defer s.Close()
	count := 0
	readonly := false
	view := func() *Element {
		return Column("root", DatePicker("date", "Date", state, CalendarOptions{Disabled: func(d Date) bool { return d.Day == 13 }, OnChange: func(Date) { count++ }}).ReadOnly(readonly)).Padding(20)
	}
	mount(t, s, view())
	s.Focus("date/trigger")
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if !state.Open || s.focus != "date/calendar/date/2026-11-12" {
		t.Fatal("calendar did not take focus", s.focus)
	}
	if s.Focus("date/calendar/date/2026-11-13") {
		t.Fatal("disabled date focusable")
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	if state.Open || s.focus != "date/trigger" || state.HasValue {
		t.Fatal("dismissal changed value/focus")
	}
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, view())
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if state.Open || !state.HasValue || state.Calendar.Value.Day != 12 || count != 1 {
		t.Fatal("selection did not commit once")
	}
	readonly = true
	mount(t, s, view())
	s.Focus("date/trigger")
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if state.Open {
		t.Fatal("read-only date picker opened")
	}
}
func TestDatePickerRangeClosesAfterEnd(t *testing.T) {
	state := NewDatePickerState(Date{2026, 11, 12})
	state.Open = true
	s, _ := NewSurface(500, 480, NewTheme(true))
	defer s.Close()
	view := func() *Element {
		return Column("root", DatePicker("date", "Range", state, CalendarOptions{Mode: DateRange})).Padding(20)
	}
	mount(t, s, view())
	s.Focus("date/calendar/date/2026-11-12")
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if !state.Open {
		t.Fatal("range closed after start")
	}
	s.Focus("date/calendar/date/2026-11-15")
	s.Key(KeyEvent{Key: "Enter"})
	mount(t, s, view())
	if state.Open || state.Calendar.Start.Day != 12 || state.Calendar.End.Day != 15 {
		t.Fatal("range not committed")
	}
}
