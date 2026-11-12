package baseui

import (
	"strings"
	"time"
)

// FileNode is metadata; FileTree never reads the filesystem. IDs are unique
// across the whole tree, and caller callbacks decide how files are opened.
type FileNode struct {
	ID, Name, Path, Change string
	Directory, Disabled    bool
	Children               []FileNode
}
type FileTreeState struct {
	Open     map[string]bool
	Selected string
	scroll   ScrollState
	search   typeSearch
}
type treeRow struct {
	node     FileNode
	parent   string
	depth    int
	disabled bool
}

func FileTree(id string, state *FileTreeState, nodes []FileNode, onOpen func(FileNode)) *Element {
	if state == nil {
		panic("baseui: nil file tree state")
	}
	if state.Open == nil {
		state.Open = map[string]bool{}
	}
	seen := map[string]bool{}
	count := 0
	var snapshot func([]FileNode, int) []FileNode
	snapshot = func(items []FileNode, depth int) []FileNode {
		if depth > 64 {
			panic("baseui: file tree exceeds 64 levels")
		}
		copyItems := make([]FileNode, len(items))
		for i, n := range items {
			count++
			if count > 100000 || n.ID == "" || seen[n.ID] {
				panic("baseui: invalid file tree IDs or size")
			}
			seen[n.ID] = true
			n.Children = snapshot(n.Children, depth+1)
			copyItems[i] = n
		}
		return copyItems
	}
	nodes = snapshot(nodes, 0)
	rows := []treeRow{}
	var flatten func([]FileNode, string, int, bool)
	flatten = func(items []FileNode, parent string, depth int, disabled bool) {
		for _, n := range items {
			off := disabled || n.Disabled
			rows = append(rows, treeRow{n, parent, depth, off})
			if state.Open[n.ID] {
				flatten(n.Children, n.ID, depth+1, off)
			}
		}
	}
	flatten(nodes, "", 0, false)
	options := make([]Option, len(rows))
	for i, r := range rows {
		options[i] = Option{ID: r.node.ID, Label: r.node.Name, Disabled: r.disabled}
	}
	children := []*Element{}
	for i, row := range rows {
		n := row.node
		directory := n.Directory || len(n.Children) > 0
		label := n.Name

		if n.Change != "" {
			label += "  · " + strings.ToUpper(n.Change)
		}
		activate := func() {
			if directory {
				state.Open[n.ID] = !state.Open[n.ID]
				return
			}
			state.Selected = n.ID
			if onOpen != nil {
				// Give the callback an independent metadata subtree.
				out := n
				out.Children = nil
				onOpen(out)
			}
		}
		item := Button(id+"/node/"+n.ID, label, activate).Variant(Ghost).Disabled(row.disabled).Role("treeitem")
		item.alignStart = true
		item.textIndent = float64(row.depth)*18 + 14
		item.branch = directory
		item.selected = state.Selected == n.ID
		item.focusGroup = id
		item.expanded = directory && state.Open[n.ID]
		item.treeLevel = row.depth + 1
		item.OnKey(func(ev KeyEvent) bool {
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
				if directory {
					if !state.Open[n.ID] {
						state.Open[n.ID] = true
					} else if i+1 < len(rows) && rows[i+1].parent == n.ID && !rows[i+1].disabled {
						target = i + 1
					}
				}
			case "Left":
				if directory && state.Open[n.ID] {
					state.Open[n.ID] = false
				} else if row.parent != "" {
					ev.RequestFocus(id + "/node/" + row.parent)
				}
			default:
				return false
			}
			if target >= 0 {
				ev.RequestFocus(id + "/node/" + rows[target].node.ID)
			}
			return true
		})
		item.onText = func(text string, now time.Duration) string {
			target := state.search.find(text, now, i, options)
			if target >= 0 {
				return id + "/node/" + rows[target].node.ID
			}
			return ""
		}
		children = append(children, item)
	}
	if len(children) == 0 {
		children = append(children, Text(id+"/empty", "No files"))
	}
	e := ScrollArea(id, &state.scroll, children...).Height(280).Gap(2).Role("tree").Label("Files")
	e.skipTab = true
	return e
}
