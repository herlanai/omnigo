package baseui

import (
	"fmt"
	"testing"
	"time"
	"unicode/utf8"
)

func TestUnicodeHistoryAndLimit(t *testing.T) {
	b := NewTextBuffer("你好🙂")
	b.Limit = 4
	b.Select(1, 2)
	b.Replace("世界🦀")
	if b.String() != "你世界🙂" {
		t.Fatal(b.String())
	}
	if !b.Undo() || b.String() != "你好🙂" {
		t.Fatal("undo")
	}
	if !b.Redo() || b.String() != "你世界🙂" {
		t.Fatal("redo")
	}
	b.Select(0, 99)
	if b.Selected() != b.String() {
		t.Fatal("selection")
	}
	for range 150 {
		b.Limit = 0
		b.Select(0, 0)
		b.Replace("x")
	}
	if len(b.undo) > 100 {
		t.Fatal("unbounded undo")
	}
}
func FuzzTextEdits(f *testing.F) {
	f.Add("你好🙂", 3, 1, "text")
	f.Fuzz(func(t *testing.T, initial string, a, b int, insert string) {
		if len(initial)+len(insert) > 4096 {
			return
		}
		buffer := NewTextBuffer(initial)
		buffer.Select(a, b)
		before := buffer.String()
		buffer.Replace(insert)
		if !utf8.ValidString(buffer.String()) {
			t.Fatal("invalid UTF8")
		}
		anchor, caret := buffer.Selection()
		n := utf8.RuneCountInString(buffer.String())
		if anchor < 0 || caret < 0 || anchor > n || caret > n {
			t.Fatal("selection out of bounds")
		}
		if buffer.Undo() && buffer.String() != before {
			t.Fatal("undo mismatch")
		}
	})
}
func TestCalendarModesAndLeapMonth(t *testing.T) {
	jan := Date{2024, time.January, 31}
	if got := jan.AddMonths(1); got != (Date{2024, time.February, 29}) {
		t.Fatal(got)
	}
	state := NewCalendarState(jan)
	options := CalendarOptions{Mode: MultipleDates}
	state.Choose(jan, options)
	state.Choose(jan, options)
	if state.Selected[jan] {
		t.Fatal("multiple did not toggle")
	}
	options.Mode = DateRange
	state.Choose(Date{2024, time.March, 10}, options)
	state.Choose(Date{2024, time.March, 5}, options)
	if state.Start.Day != 5 || state.End.Day != 10 {
		t.Fatal(state.Start, state.End)
	}
	options.Disabled = func(d Date) bool { return d.Day == 20 }
	if state.Choose(Date{2024, time.March, 20}, options) {
		t.Fatal("disabled date selected")
	}
}
func TestCalendarKeyboardAndTabsSkipDisabled(t *testing.T) {
	s := surface(t)
	state := NewCalendarState(Date{2024, time.January, 31})
	view := func() *Element { return Calendar("cal", state, CalendarOptions{WeekStart: time.Monday}) }
	mount(t, s, view())
	s.Focus("cal/date/2024-01-31")
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	if s.Focused() != "cal/date/2024-02-01" {
		t.Fatal(s.Focused())
	}
	value := "a"
	options := []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B", Disabled: true}, {ID: "c", Label: "C"}}
	tabs := func() *Element { return Tabs("tabs", "Views", value, options, func(v string) { value = v }) }
	mount(t, s, tabs())
	s.Focus("tabs/a")
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, tabs())
	if value != "c" || s.Focused() != "tabs/c" {
		t.Fatal(value, s.Focused())
	}
}

type countedData struct{ calls int }

func (d *countedData) Len() int                         { return 1_000_000 }
func (d *countedData) Key(i int) string                 { return fmt.Sprint(i) }
func (d *countedData) Cell(i int, column string) string { d.calls++; return fmt.Sprint(i) }
func TestTableOnlyMaterializesVisibleRows(t *testing.T) {
	s := surface(t)
	d := &countedData{}
	state := &TableState{Offset: 50000, Limit: 10}
	mount(t, s, Table("table", state, []TableColumn{{ID: "name", Title: "Name", Width: 100}}, d, nil))
	if d.calls != 10 {
		t.Fatal("visited offscreen rows", d.calls)
	}
	start, end, offset := Window(1000000, 24, 2400, 240, 2)
	if start != 98 || end != 112 || offset != 2352 {
		t.Fatal(start, end, offset)
	}
}
