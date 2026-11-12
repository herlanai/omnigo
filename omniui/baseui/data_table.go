package baseui

import (
	"fmt"
	"sort"
	"strings"
)

// DataRow cells are plain text keyed by column ID. DataTable snapshots them.
type DataRow struct {
	Key   string
	Cells map[string]string
}
type DataTableState struct {
	Query          *TextBuffer
	Page, PageSize int
	SortColumn     string
	Descending     bool
	Selected       map[string]bool
	HiddenColumns  map[string]bool
	columns        MenuState
	lastQuery      string
}

func NewDataTableState(pageSize int) *DataTableState {
	if pageSize < 1 || pageSize > 100 {
		panic("baseui: data table page size outside 1..100")
	}
	return &DataTableState{Query: NewTextBuffer(""), PageSize: pageSize, Selected: map[string]bool{}, HiddenColumns: map[string]bool{}}
}

// DataTable is a bounded client-side table: filtering, stable text sorting,
// paging, visible-page selection and column visibility. For large/server data,
// use Table with an explicit window. Numeric/custom sorting is not inferred.
func DataTable(id string, state *DataTableState, columns []TableColumn, source []DataRow) *Element {
	if state == nil || state.Query == nil || state.PageSize < 1 || state.PageSize > 100 || len(source) > 10000 {
		panic("baseui: invalid data table or more than 10000 client rows")
	}
	if state.Selected == nil {
		state.Selected = map[string]bool{}
	}
	if state.HiddenColumns == nil {
		state.HiddenColumns = map[string]bool{}
	}
	columns = append([]TableColumn(nil), columns...)
	seen := map[string]bool{}
	visible := []TableColumn{}
	menus := []MenuItem{}
	for _, c := range columns {
		if c.ID == "" || seen[c.ID] || !finite(c.Width) || c.Width <= 0 {
			panic("baseui: invalid data table column")
		}
		seen[c.ID] = true
		shown := !state.HiddenColumns[c.ID]
		menus = append(menus, MenuItem{ID: c.ID, Label: c.Title, Checked: &shown, Action: func() { state.HiddenColumns[c.ID] = !shown }})
		if shown {
			visible = append(visible, c)
		}
	}
	query := strings.ToLower(strings.TrimSpace(state.Query.String()))
	if query != state.lastQuery {
		state.Page = 0
		state.lastQuery = query
	}
	rows := []DataRow{}
	seen = map[string]bool{}
	for _, r := range source {
		if r.Key == "" || seen[r.Key] {
			panic("baseui: data table requires unique row keys")
		}
		seen[r.Key] = true
		copyRow := DataRow{Key: r.Key, Cells: map[string]string{}}
		matches := query == ""
		for _, c := range columns {
			value := r.Cells[c.ID]
			copyRow.Cells[c.ID] = value
			if strings.Contains(strings.ToLower(value), query) {
				matches = true
			}
		}
		if matches {
			rows = append(rows, copyRow)
		}
	}
	for key := range state.Selected {
		if !seen[key] {
			delete(state.Selected, key)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := strings.ToLower(rows[i].Cells[state.SortColumn]), strings.ToLower(rows[j].Cells[state.SortColumn])
		if state.Descending {
			return a > b
		}
		return a < b
	})
	pages := max(1, (len(rows)+state.PageSize-1)/state.PageSize)
	state.Page = max(0, min(state.Page, pages-1))
	start := min(len(rows), state.Page*state.PageSize)
	end := min(len(rows), start+state.PageSize)
	page := rows[start:end]
	selected := 0
	for _, r := range page {
		if state.Selected[r.Key] {
			selected++
		}
	}
	check := Unchecked
	if selected > 0 {
		check = Mixed
	}
	if len(page) > 0 && selected == len(page) {
		check = Checked
	}
	headers := []*Element{MixedCheckbox(id+"/select-page", "", check, func(CheckState) {
		set := selected != len(page)
		for _, r := range page {
			state.Selected[r.Key] = set
		}
	}).Width(36).Disabled(len(page) == 0).Label("Select current page")}
	for _, c := range visible {
		label := c.Title
		if state.SortColumn == c.ID {
			if state.Descending {
				label += " ↓"
			} else {
				label += " ↑"
			}
		}
		h := Button(id+"/head/"+c.ID, label, func() {
			if state.SortColumn == c.ID {
				state.Descending = !state.Descending
			} else {
				state.SortColumn = c.ID
				state.Descending = false
			}
			state.Page = 0
		}).Variant(Ghost).Width(c.Width).Disabled(!c.Sortable)
		h.alignStart = true
		h.role = "columnheader"
		headers = append(headers, h)
	}
	body := []*Element{Row(id+"/header", headers...).Gap(0), Separator(id + "/header-rule")}
	for _, r := range page {
		cells := []*Element{Checkbox(id+"/select/"+r.Key, "", state.Selected[r.Key], func(v bool) { state.Selected[r.Key] = v }).Width(36).Label("Select " + r.Key)}
		for _, c := range visible {
			cell := Text(id+"/cell/"+r.Key+"/"+c.ID, r.Cells[c.ID]).Width(c.Width).Height(40)
			cell.textIndent = 12
			cells = append(cells, cell)
		}
		line := Row(id+"/row/"+r.Key, cells...).Gap(0)
		line.role = "row"
		line.selected = state.Selected[r.Key]
		body = append(body, line, Separator(id+"/rule/"+r.Key))
	}
	if len(page) == 0 {
		body = append(body, Text(id+"/empty", "No results.").Height(64).Muted(true))
	}
	count := 0
	for _, r := range rows {
		if state.Selected[r.Key] {
			count++
		}
	}
	toolbar := Row(id+"/toolbar", Input(id+"/filter", "Filter rows", state.Query).Placeholder("Filter rows…").Grow(1), DropdownMenu(id+"/columns", "Columns", &state.columns, menus)).Gap(12)
	footer := Row(id+"/footer", Text(id+"/count", fmt.Sprintf("%d of %d rows selected", count, len(rows))).FontSize(13).Muted(true).Grow(1), Text(id+"/page", fmt.Sprintf("Page %d of %d", state.Page+1, pages)).FontSize(13), Button(id+"/previous", "Previous", func() { state.Page = max(0, state.Page-1) }).Variant(Outline).Disabled(state.Page == 0), Button(id+"/next", "Next", func() { state.Page = min(pages-1, state.Page+1) }).Variant(Outline).Disabled(state.Page == pages-1)).Gap(12)
	return Column(id, toolbar, Column(id+"/table", body...).Gap(0).Role("table"), footer).Gap(16)
}
