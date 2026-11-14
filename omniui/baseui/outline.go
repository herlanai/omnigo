package baseui

import (
	"fmt"
	"math"
	"time"
)

// OutlineState stores expansion and selection by business key, independent of
// row positions. It belongs to the UI goroutine. Zero values are ready to use.
type OutlineState[K comparable] struct {
	Open         map[K]bool
	Selected     K
	HasSelection bool
	Scroll       ScrollState
	search       typeSearch
}
type OutlineOptions[K comparable] struct {
	// ID must return a nonempty, stable, unique identifier for each item.
	ID         func(K) string
	Label      func(K) string
	Children   func(K) []K
	Disabled   func(K) bool
	OnSelect   func(K)
	OnActivate func(K)
	Indent     float64
	// RowHeight enables fixed-height viewport virtualization when positive.
	// Custom cells must fit this height; zero preserves intrinsic layout.
	RowHeight float64
	Height    float64
}

// OutlineColumn builds a cell using the supplied unique element ID. Width zero
// shares available space; positive widths are fixed logical pixels.
type OutlineColumn[K comparable] struct {
	Title string
	Width float64
	Cell  func(id string, item K) *Element
}
type outlineEntry[K comparable] struct {
	key              K
	id               string
	parent, depth    int
	branch, disabled bool
}

// TreeOutline is the general-purpose keyed tree. The name avoids the existing
// Outline button variant. Unlike FileTree, keys need not represent files.
func TreeOutline[K comparable](id string, state *OutlineState[K], roots []K, o OutlineOptions[K]) *Element {
	return outlineView(id, state, roots, o, nil)
}

// OutlineTable renders a tree with a fixed header and arbitrary Go cell controls.
// The first column contains disclosure and selection; subsequent cells are built
// by the application. Column sorting/resizing are pending. RowHeight enables virtualization.
func OutlineTable[K comparable](id string, state *OutlineState[K], roots []K, o OutlineOptions[K], columns []OutlineColumn[K]) *Element {
	if len(columns) == 0 {
		panic("baseui: outline table requires columns")
	}
	columns = append([]OutlineColumn[K](nil), columns...)
	for i, c := range columns {
		if !finite(c.Width) || c.Width < 0 || (i > 0 && c.Cell == nil) {
			panic("baseui: invalid outline column")
		}
	}
	return outlineView(id, state, roots, o, columns)
}
func outlineView[K comparable](id string, state *OutlineState[K], roots []K, o OutlineOptions[K], columns []OutlineColumn[K]) *Element {
	if state == nil || o.ID == nil || o.Label == nil || o.Children == nil || !finite(o.Indent) || o.Indent < 0 {
		panic("baseui: invalid outline")
	}
	if !finite(o.RowHeight) || o.RowHeight < 0 || (o.RowHeight > 0 && o.RowHeight < 32) || !finite(o.Height) || o.Height < 0 || o.Height > 16384 || !finite(state.Scroll.Y) {
		panic("baseui: invalid outline viewport")
	}
	if o.Height == 0 {
		o.Height = 280
	}
	if state.Open == nil {
		state.Open = make(map[K]bool)
	}
	if o.Indent == 0 {
		o.Indent = 18
	}
	rows := []outlineEntry[K]{}
	seen := map[K]bool{}
	ids := map[string]bool{}
	var visit func([]K, int, int, bool)
	visit = func(items []K, parent, depth int, disabled bool) {
		if depth > 64 {
			panic("baseui: outline depth exceeds 64")
		}
		for _, key := range items {
			itemID := o.ID(key)
			if seen[key] || itemID == "" || ids[itemID] || len(rows) >= 100000 {
				panic("baseui: duplicate/cyclic outline or row budget exceeded")
			}
			seen[key], ids[itemID] = true, true
			children := append([]K(nil), o.Children(key)...)
			off := disabled || (o.Disabled != nil && o.Disabled(key))
			index := len(rows)
			rows = append(rows, outlineEntry[K]{key, itemID, parent, depth, len(children) > 0, off})
			if state.Open[key] {
				visit(children, index, depth+1, off)
			}
		}
	}
	visit(roots, -1, 0, false)
	options := make([]Option, len(rows))
	for i, row := range rows {
		options[i] = Option{ID: row.id, Disabled: row.disabled}
	}
	labelsReady := false
	fillLabels := func() {
		if labelsReady {
			return
		}
		for i, row := range rows {
			options[i].Label = o.Label(row.key)
		}
		labelsReady = true
	}
	rowID := func(i int) string { return id + "/item/" + rows[i].id + "/select" }
	selectKey := func(key K) {
		changed := !state.HasSelection || state.Selected != key
		state.Selected, state.HasSelection = key, true
		if changed && o.OnSelect != nil {
			o.OnSelect(key)
		}
	}
	sizeCell := func(e *Element, col int) *Element {
		if len(columns) > 0 && columns[col].Width > 0 {
			return e.Width(columns[col].Width)
		}
		return e.Grow(1)
	}
	start, end := 0, len(rows)
	stride := o.RowHeight + 2
	reveal := func(i int) {
		if o.RowHeight == 0 {
			return
		}
		top := float64(i) * stride
		if top < state.Scroll.Y {
			state.Scroll.Y = top
		}
		if top+o.RowHeight > state.Scroll.Y+o.Height {
			state.Scroll.Y = top + o.RowHeight - o.Height
		}
	}
	if o.RowHeight > 0 {
		total := math.Max(0, float64(len(rows))*stride-2)
		state.Scroll.Y = math.Max(0, math.Min(state.Scroll.Y, total-o.Height))
		start = max(0, int(state.Scroll.Y/stride)-2)
		end = min(len(rows), int(math.Ceil((state.Scroll.Y+o.Height)/stride))+2)
	}
	elements := make([]*Element, 0, end-start+2)
	if start > 0 {
		elements = append(elements, Column(id+"/before").Height(float64(start)*stride-2))
	}
	for i := start; i < end; i++ {
		row := rows[i]
		prefix := id + "/item/" + row.id
		label := o.Label(row.key)
		item := Button(rowID(i), label, func() { selectKey(row.key) }).Variant(Ghost).Grow(1).Height(32).Role("treeitem")
		item.alignStart = true
		item.selected = state.HasSelection && state.Selected == row.key
		item.treeLevel, item.expanded = row.depth+1, row.branch && state.Open[row.key]
		item.focusGroup = id
		item.onKey = func(ev KeyEvent) bool {
			key := ev.Key
			if ev.RTL {
				if key == "Left" {
					key = "Right"
				} else if key == "Right" {
					key = "Left"
				}
			}
			target := -1
			switch key {
			case "Up", "Down", "Home", "End":
				target = optionKey(ev, i, options)
			case "Right":
				if row.branch {
					if !state.Open[row.key] {
						state.Open[row.key] = true
					} else if i+1 < len(rows) && !rows[i+1].disabled {
						target = i + 1
					}
				}
			case "Left":
				if row.branch && state.Open[row.key] {
					state.Open[row.key] = false
				} else {
					target = row.parent
				}
			case "Enter":
				selectKey(row.key)
				if o.OnActivate != nil {
					o.OnActivate(row.key)
				}
			default:
				return false
			}
			if target >= 0 {
				reveal(target)
				selectKey(rows[target].key)
				ev.RequestFocus(rowID(target))
			}
			return true
		}
		item.onText = func(text string, now time.Duration) string {
			fillLabels()
			target := state.search.find(text, now, i, options)
			if target >= 0 {
				reveal(target)
				selectKey(rows[target].key)
				return rowID(target)
			}
			return ""
		}
		arrow := Button(prefix+"/toggle", "", func() { state.Open[row.key] = !state.Open[row.key] }).Variant(Ghost).Width(24).Height(32).Label("Toggle " + label).Disabled(!row.branch)
		arrow.branch, arrow.expanded, arrow.textIndent, arrow.skipTab = row.branch, state.Open[row.key], 14, true
		first := Row(prefix+"/first", Column(prefix+"/indent").Width(float64(row.depth)*o.Indent+.01), arrow, item).Gap(0)
		cells := []*Element{sizeCell(first, 0)}
		for col := 1; col < len(columns); col++ {
			cellID := fmt.Sprintf("%s/cell/%d", prefix, col)
			cell := columns[col].Cell(cellID+"/content", row.key)
			if cell == nil {
				panic("baseui: nil outline cell")
			}
			cells = append(cells, sizeCell(Column(cellID, cell).Padding(4), col))
		}
		line := Row(prefix, cells...).Gap(8).Disabled(row.disabled).Role("row")
		line.centerItems = true
		if o.RowHeight > 0 {
			line.Height(o.RowHeight)
		}
		elements = append(elements, line)
	}
	if end < len(rows) {
		elements = append(elements, Column(id+"/after").Height(float64(len(rows)-end)*stride-2))
	}
	viewport := ScrollArea(id+"/rows", &state.Scroll, elements...).Height(o.Height).Gap(2).Role("tree").Label("Outline")
	viewport.skipTab = true
	if len(columns) == 0 {
		return Column(id, viewport).Gap(0)
	}
	headers := make([]*Element, len(columns))
	for i, col := range columns {
		headers[i] = sizeCell(Text(fmt.Sprintf("%s/header/%d", id, i), col.Title).FontWeight(600).Muted(true).Role("columnheader"), i)
	}
	return Column(id, Row(id+"/header", headers...).Gap(8).Padding(4), viewport).Gap(4).Role("treegrid")
}
