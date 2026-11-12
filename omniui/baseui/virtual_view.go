package baseui

import (
	"fmt"
	"math"
	"strconv"
)

// VirtualListState is UI-goroutine owned. Selection follows the application's
// stable Key, while Scroll is an explicit pixel offset. Rebuild after events.
type VirtualListState struct {
	Scroll   ScrollState
	Selected string
}
type VirtualListOptions struct {
	Count             int
	RowHeight, Height float64
	Overscan          int
	Key               func(int) string
	Label             func(int) string
	Disabled          func(int) bool
	OnSelect          func(int)
	OnActivate        func(int)
}
type GridViewOptions struct {
	VirtualListOptions
	Width, MinWidth, Gap float64
}

// VirtualList builds only the fixed-height rows intersecting its viewport plus
// overscan. Item receives a stable, unique ID for its subtree. Variable heights,
// anchor correction after insertion and multi-selection are not implemented.
func VirtualList(id string, state *VirtualListState, o VirtualListOptions, item func(string, int) *Element) *Element {
	return virtualView(id, state, o, 1, 0, 0, item)
}

// GridView virtualizes complete rows of fixed-height cells. Width is the actual
// viewport width; rebuild with the new Width when the host resizes the viewport.
func GridView(id string, state *VirtualListState, o GridViewOptions, item func(string, int) *Element) *Element {
	if !finite(o.Width) || !finite(o.MinWidth) || !finite(o.Gap) || o.Width <= 10 || o.MinWidth <= 0 || o.Gap < 0 {
		panic("baseui: invalid virtual grid geometry")
	}
	columns := max(1, int(math.Floor((o.Width-10+o.Gap)/(o.MinWidth+o.Gap))))
	if columns > 128 {
		panic("baseui: virtual grid exceeds 128 columns")
	}
	return virtualView(id, state, o.VirtualListOptions, columns, o.Width, o.Gap, item)
}
func virtualView(id string, state *VirtualListState, o VirtualListOptions, columns int, width, gap float64, item func(string, int) *Element) *Element {
	if state == nil || item == nil || o.Count < 0 || o.Count > 100000000 || !finite(o.RowHeight) || o.RowHeight < 32 || !finite(o.Height) || o.Height <= 0 || o.Height > 16384 || o.Overscan < 0 || o.Overscan > 32 || !finite(state.Scroll.Y) {
		panic("baseui: invalid virtual list")
	}
	rows := (o.Count + columns - 1) / columns
	stride := o.RowHeight + gap
	total := max(0, float64(rows)*stride-gap)
	state.Scroll.Y = max(0, min(state.Scroll.Y, max(0, total-o.Height)))
	start := max(0, int(math.Floor(state.Scroll.Y/stride))-o.Overscan)
	end := min(rows, int(math.Ceil((state.Scroll.Y+o.Height)/stride))+o.Overscan)
	if (end-start)*columns > 4096 {
		panic("baseui: virtual viewport exceeds 4096 items")
	}
	key := func(i int) string {
		if o.Key != nil {
			return o.Key(i)
		}
		return strconv.Itoa(i)
	}
	label := func(i int) string {
		if o.Label != nil {
			return o.Label(i)
		}
		return fmt.Sprintf("Item %d", i+1)
	}
	disabled := func(i int) bool { return o.Disabled != nil && o.Disabled(i) }
	selectItem := func(i int) {
		k := key(i)
		changed := state.Selected != k
		state.Selected = k
		if changed && o.OnSelect != nil {
			o.OnSelect(i)
		}
	}
	reveal := func(i int) {
		top := float64(i/columns) * stride
		if top < state.Scroll.Y {
			state.Scroll.Y = top
		}
		if top+o.RowHeight > state.Scroll.Y+o.Height {
			state.Scroll.Y = top + o.RowHeight - o.Height
		}
	}
	children := []*Element{}
	if start > 0 {
		children = append(children, Column(id+"/before").Height(float64(start)*stride))
	}
	seen := map[string]bool{}
	for row := start; row < end; row++ {
		cells := []*Element{}
		for col := 0; col < columns; col++ {
			i := row*columns + col
			if i >= o.Count {
				cells = append(cells, Column(fmt.Sprintf("%s/empty/%d", id, col)).Grow(1))
				continue
			}
			k := key(i)
			if k == "" || seen[k] {
				panic("baseui: empty or duplicate virtual item key")
			}
			seen[k] = true
			prefix := id + "/item/" + k
			choose := Button(prefix+"/select", label(i), func() { selectItem(i) }).Variant(Ghost).Height(32).Role("option")
			choose.selected = state.Selected == k
			choose.alignStart = true
			choose.focusGroup = id
			choose.onKey = func(ev KeyEvent) bool {
				target := i
				direction := 1
				switch ev.Key {
				case "Up":
					target -= columns
					direction = -1
				case "Down":
					target += columns
				case "Left":
					if columns == 1 {
						return false
					}
					target--
					direction = -1
				case "Right":
					if columns == 1 {
						return false
					}
					target++
				case "Home":
					target = 0
				case "End":
					target = o.Count - 1
					direction = -1
				case "PageUp":
					target -= max(1, int(o.Height/stride)) * columns
					direction = -1
				case "PageDown":
					target += max(1, int(o.Height/stride)) * columns
				case "Enter":
					selectItem(i)
					if !ev.Repeat && o.OnActivate != nil {
						o.OnActivate(i)
					}
					return true
				default:
					return false
				}
				target = max(0, min(o.Count-1, target))
				for target >= 0 && target < o.Count && disabled(target) {
					target += direction
				}
				if target >= 0 && target < o.Count {
					selectItem(target)
					reveal(target)
					ev.RequestFocus(id + "/item/" + key(target) + "/select")
				}
				return true
			}
			content := item(prefix+"/content", i)
			if content == nil {
				panic("baseui: nil virtual item")
			}
			cells = append(cells, Column(prefix, choose, content).Gap(0).Height(o.RowHeight).Grow(1).Disabled(disabled(i)))
		}
		children = append(children, Row(fmt.Sprintf("%s/row/%d", id, row), cells...).Gap(gap).Height(o.RowHeight))
		if row < rows-1 {
			children = append(children, Column(fmt.Sprintf("%s/gap/%d", id, row)).Height(gap))
		}
	}
	if end < rows {
		children = append(children, Column(id+"/after").Height(float64(rows-end)*stride-gap))
	}
	root := ScrollArea(id, &state.Scroll, children...).Gap(0).Height(o.Height).Role("listbox")
	root.skipTab = o.Count > 0
	if width > 0 {
		root.Width(width)
	}
	return root
}
