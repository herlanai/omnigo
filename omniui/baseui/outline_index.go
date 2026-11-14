package baseui

type outlineIndex[K comparable] struct {
	revision uint64
	rows     []outlineEntry[K]
	open     map[K]bool
}

func copyExpansion[K comparable](open map[K]bool) map[K]bool {
	result := make(map[K]bool)
	for k, v := range open {
		if v {
			result[k] = true
		}
	}
	return result
}
func sameExpansion[K comparable](a, b map[K]bool) bool {
	for k, v := range a {
		if v != b[k] {
			return false
		}
	}
	for k, v := range b {
		if v != a[k] {
			return false
		}
	}
	return true
}

func (state *OutlineState[K]) indexRows(roots []K, o OutlineOptions[K]) []outlineEntry[K] {
	if !o.Incremental {
		state.dirty = nil
		state.index = nil
		return state.fullRows(roots, o)
	}
	old := state.index
	if old == nil || old.revision != o.Revision {
		rows := state.fullRows(roots, o)
		state.dirty = nil
		state.index = &outlineIndex[K]{o.Revision, rows, copyExpansion(state.Open)}
		return rows
	}
	if sameExpansion(old.open, state.Open) && len(state.dirty) == 0 {
		return old.rows
	}
	rows := make([]outlineEntry[K], 0, len(old.rows))
	seen := make(map[K]bool)
	ids := make(map[string]bool)
	stack := []int{}
	appendRow := func(row outlineEntry[K]) {
		if row.depth > 64 || row.depth > len(stack) || len(rows) >= 100000 || seen[row.key] || row.id == "" || ids[row.id] {
			panic("baseui: invalid incremental outline")
		}
		seen[row.key] = true
		ids[row.id] = true
		row.parent = -1
		if row.depth > 0 {
			row.parent = stack[row.depth-1]
		}
		stack = append(stack[:row.depth], len(rows))
		rows = append(rows, row)
	}
	var expand func([]K, int, bool)
	expand = func(keys []K, depth int, disabled bool) {
		for _, key := range keys {
			children := append([]K(nil), o.Children(key)...)
			off := disabled || (o.Disabled != nil && o.Disabled(key))
			appendRow(outlineEntry[K]{key: key, id: o.ID(key), depth: depth, branch: len(children) > 0, disabled: off})
			if state.Open[key] {
				expand(children, depth+1, off)
			}
		}
	}
	for i := 0; i < len(old.rows); i++ {
		row := old.rows[i]
		if state.dirty[row.key] {
			inherited := false
			if row.depth > 0 {
				inherited = rows[stack[row.depth-1]].disabled
			}
			children := append([]K(nil), o.Children(row.key)...)
			row.id = o.ID(row.key)
			row.branch = len(children) > 0
			row.disabled = inherited || (o.Disabled != nil && o.Disabled(row.key))
			appendRow(row)
			if state.Open[row.key] {
				expand(children, row.depth+1, row.disabled)
			}
			for i+1 < len(old.rows) && old.rows[i+1].depth > row.depth {
				i++
			}
			continue
		}
		appendRow(row)
		if !state.Open[row.key] {
			for i+1 < len(old.rows) && old.rows[i+1].depth > row.depth {
				i++
			}
			continue
		}
		if !old.open[row.key] && row.branch {
			expand(append([]K(nil), o.Children(row.key)...), row.depth+1, row.disabled)
		}
	}
	state.index = &outlineIndex[K]{o.Revision, rows, copyExpansion(state.Open)}
	state.dirty = nil
	return rows
}

func (state *OutlineState[K]) fullRows(roots []K, o OutlineOptions[K]) []outlineEntry[K] {
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
	return rows

}

// Invalidate refreshes specific model subtrees on the next incremental build.
// Invalidate the parent for child insertion/removal/reorder. A call without keys
// invalidates the entire model (also required when the roots change). Call on UI.
func (state *OutlineState[K]) Invalidate(keys ...K) {
	if len(keys) == 0 {
		state.index = nil
		state.dirty = nil
		return
	}
	if state.dirty == nil {
		state.dirty = make(map[K]bool)
	}
	for _, key := range keys {
		state.dirty[key] = true
	}
}
