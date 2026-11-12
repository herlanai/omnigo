package baseui

import (
	"fmt"
	"math"
	"sort"
)

type TableColumn struct {
	ID, Title string
	Width     float64
	Sortable  bool
}
type TableState struct {
	Offset, Limit int
	SortColumn    string
	Descending    bool
	Selected      map[string]bool
}
type TableData interface {
	Len() int
	Key(row int) string
	Cell(row int, column string) string
}

// Table materializes at most Limit rows. The data source owns sorting/paging;
// clicking a sortable header updates state and calls onSort. This avoids silently
// scanning a million rows on each frame. Column resize/cell selection are pending.
func Table(id string, state *TableState, columns []TableColumn, data TableData, onSort func(string, bool)) *Element {
	if state == nil || data == nil || state.Offset < 0 || state.Limit < 1 || state.Limit > 10000 || data.Len() < 0 {
		panic("baseui: invalid table window")
	}
	seen := map[string]bool{}
	header := []*Element{}
	for _, column := range columns {
		column := column
		if column.ID == "" || seen[column.ID] || !finite(column.Width) || column.Width <= 0 {
			panic("baseui: invalid table columns")
		}
		seen[column.ID] = true
		title := column.Title
		if state.SortColumn == column.ID {
			if state.Descending {
				title += " v"
			} else {
				title += " ^"
			}
		}
		cell := Button(id+"/head/"+column.ID, title, func() {
			if state.SortColumn == column.ID {
				state.Descending = !state.Descending
			} else {
				state.SortColumn = column.ID
				state.Descending = false
			}
			if onSort != nil {
				onSort(state.SortColumn, state.Descending)
			}
		}).Width(column.Width).Variant(Ghost).Disabled(!column.Sortable)
		if !column.Sortable {
			cell = Text(id+"/head/"+column.ID, title).Width(column.Width).Height(36)
		}
		cell.role = "columnheader"
		cell.fontSize = 13
		cell.fontWeight = 600
		cell.alignStart = true
		cell.mutedText = true
		if cell.kind == TextKind {
			cell.textIndent = 12
		}
		header = append(header, cell)
	}
	rows := []*Element{Row(id+"/header", header...).Gap(0), Separator(id + "/rule")}
	keys := map[string]bool{}
	end := state.Offset + min(state.Limit, max(0, data.Len()-state.Offset))
	for row := state.Offset; row < end; row++ {
		key := data.Key(row)
		if key == "" || keys[key] {
			panic("baseui: table requires unique visible row keys")
		}
		keys[key] = true
		cells := []*Element{}
		for _, column := range columns {
			cell := Text(id+"/row/"+key+"/"+column.ID, data.Cell(row, column.ID)).Width(column.Width).Height(36).FontSize(13)
			cell.textIndent = 12
			cells = append(cells, cell)
		}
		line := Row(id+"/row/"+key, cells...).Gap(0)
		line.role = "row"
		if row%2 == 1 {
			line.variant = Secondary
		}
		line.selected = state.Selected[key]
		rows = append(rows, line)
	}
	if data.Len() == 0 {
		rows = append(rows, Text(id+"/empty", "No data"))
	}
	e := Column(id, rows...).Gap(0)
	e.role = "table"
	e.label = fmt.Sprintf("Table, %d rows", data.Len())
	return e
}

// Window computes fixed-height virtualization without visiting data items.
func Window(count int, rowHeight, scroll, viewport float64, overscan int) (start, end int, spacer float64) {
	if count <= 0 || rowHeight <= 0 || viewport <= 0 || !finite(rowHeight) || !finite(scroll) || !finite(viewport) {
		return
	}
	overscan = max(0, overscan)
	scroll = max(0, scroll)
	index := func(n float64) int {
		if n <= 0 {
			return 0
		}
		if n >= float64(count) {
			return count
		}
		return int(n)
	}
	start = index(math.Floor(scroll/rowHeight) - float64(overscan))
	end = index(math.Ceil((scroll+viewport)/rowHeight) + float64(overscan))
	return start, end, float64(start) * rowHeight
}

// StableOrder sorts an explicit data snapshot outside rendering. Equal elements
// retain their input order. It never changes the caller's slice.
func StableOrder[T any](rows []T, less func(T, T) bool) []T {
	copy := append([]T(nil), rows...)
	sort.SliceStable(copy, func(i, j int) bool { return less(copy[i], copy[j]) })
	return copy
}
