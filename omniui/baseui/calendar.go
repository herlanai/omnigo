package baseui

import (
	"fmt"
	"time"
)

type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func DateOf(t time.Time) Date        { return Date{t.Year(), t.Month(), t.Day()} }
func (d Date) Time() time.Time       { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC) }
func (d Date) Valid() bool           { return d.Year >= 1 && d.Year <= 9999 && DateOf(d.Time()) == d }
func (d Date) String() string        { return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day) }
func (d Date) AddDays(days int) Date { return DateOf(d.Time().AddDate(0, 0, days)) }
func (d Date) AddMonths(months int) Date {
	first := time.Date(d.Year, d.Month, 1, 0, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	last := first.AddDate(0, 1, -1).Day()
	return Date{first.Year(), first.Month(), min(d.Day, last)}
}
func beforeDate(a, b Date) bool {
	if a.Year != b.Year {
		return a.Year < b.Year
	}
	if a.Month != b.Month {
		return a.Month < b.Month
	}
	return a.Day < b.Day
}

type CalendarMode uint8

const (
	SingleDate CalendarMode = iota
	MultipleDates
	DateRange
)

type CalendarState struct {
	Value, Cursor, Month Date
	Selected             map[Date]bool
	Start, End           Date
}

func NewCalendarState(date Date) *CalendarState {
	if !date.Valid() {
		panic("baseui: invalid initial date")
	}
	return &CalendarState{Value: date, Cursor: date, Month: Date{date.Year, date.Month, 1}, Selected: make(map[Date]bool)}
}

type CalendarOptions struct {
	Mode      CalendarMode
	WeekStart time.Weekday
	Disabled  func(Date) bool
	OnChange  func(Date)
}

func (s *CalendarState) Choose(date Date, o CalendarOptions) bool {
	if o.Mode > DateRange || !date.Valid() || o.Disabled != nil && o.Disabled(date) {
		return false
	}
	s.Value = date
	s.Cursor = date
	s.Month = Date{date.Year, date.Month, 1}
	switch o.Mode {
	case SingleDate:
	case MultipleDates:
		if s.Selected == nil {
			s.Selected = make(map[Date]bool)
		}
		if s.Selected[date] {
			delete(s.Selected, date)
		} else {
			s.Selected[date] = true
		}
	case DateRange:
		if !s.Start.Valid() || s.End.Valid() {
			s.Start = date
			s.End = Date{}
		} else if beforeDate(date, s.Start) {
			s.End = s.Start
			s.Start = date
		} else {
			s.End = date
		}
	}
	if o.OnChange != nil {
		o.OnChange(date)
	}
	return true
}
func (s *CalendarState) isSelected(date Date, mode CalendarMode) bool {
	switch mode {
	case MultipleDates:
		return s.Selected[date]
	case DateRange:
		if !s.Start.Valid() {
			return false
		}
		if !s.End.Valid() {
			return date == s.Start
		}
		return !beforeDate(date, s.Start) && !beforeDate(s.End, date)
	default:
		return date == s.Value
	}
}

func Calendar(id string, state *CalendarState, options CalendarOptions) *Element {
	if state == nil || !state.Month.Valid() || !state.Cursor.Valid() || options.Mode > DateRange || options.WeekStart < time.Sunday || options.WeekStart > time.Saturday {
		panic("baseui: invalid calendar state/options")
	}
	month := Date{state.Month.Year, state.Month.Month, 1}
	first := month.AddDays(-((int(month.Time().Weekday()) - int(options.WeekStart) + 7) % 7))
	shift := func(months int) func() {
		return func() {
			next := state.Cursor.AddMonths(months)
			if next.Valid() {
				state.Cursor = next
				state.Month = Date{next.Year, next.Month, 1}
			}
		}
	}
	rows := []*Element{Row(id+"/header", Button(id+"/previous", "<", shift(-1)).Variant(Ghost).Width(32), Text(id+"/month", fmt.Sprintf("%s %d", month.Month.String(), month.Year)).Grow(1), Button(id+"/next", ">", shift(1)).Variant(Ghost).Width(32))}
	headings := []*Element{}
	for i := 0; i < 7; i++ {
		day := time.Weekday((int(options.WeekStart) + i) % 7)
		headings = append(headings, Text(fmt.Sprintf("%s/weekday/%d", id, i), day.String()[:2]).Width(32))
	}
	rows = append(rows, Row(id+"/weekdays", headings...).Gap(4))
	for week := 0; week < 6; week++ {
		days := []*Element{}
		for day := 0; day < 7; day++ {
			date := first.AddDays(week*7 + day)
			unavailable := !date.Valid() || options.Disabled != nil && options.Disabled(date)
			button := Button(id+"/date/"+date.String(), fmt.Sprint(date.Day), func() { state.Choose(date, options) }).Width(32).Height(32).Variant(Ghost).Disabled(unavailable).Label(date.String())
			button.selected = state.isSelected(date, options.Mode)
			button.role = "gridcell"
			button.OnKey(func(ev KeyEvent) bool {
				next := date
				switch ev.Key {
				case "Left":
					next = date.AddDays(-1)
				case "Right":
					next = date.AddDays(1)
				case "Up":
					next = date.AddDays(-7)
				case "Down":
					next = date.AddDays(7)
				case "Home":
					next = date.AddDays(-((int(date.Time().Weekday()) - int(options.WeekStart) + 7) % 7))
				case "End":
					next = date.AddDays(6 - ((int(date.Time().Weekday()) - int(options.WeekStart) + 7) % 7))
				case "PageUp":
					months := -1
					if ev.Shift {
						months = -12
					}
					next = date.AddMonths(months)
				case "PageDown":
					months := 1
					if ev.Shift {
						months = 12
					}
					next = date.AddMonths(months)
				default:
					return false
				}
				if !next.Valid() {
					return true
				}
				if options.Disabled != nil && options.Disabled(next) {
					return true
				}
				state.Cursor = next
				state.Month = Date{next.Year, next.Month, 1}
				if ev.RequestFocus != nil {
					ev.RequestFocus(id + "/date/" + next.String())
				}
				return true
			})
			days = append(days, button)
		}
		rows = append(rows, Row(fmt.Sprintf("%s/week/%d", id, week), days...).Gap(4))
	}
	e := Card(id, rows...).Width(280).Gap(4)
	e.role = "grid"
	e.label = "Calendar"
	return e
}
