package baseui

import "testing"

func TestFileTreeKeyboardExpansionSelectionAndSnapshot(t *testing.T) {
	s := surface(t)
	state := &FileTreeState{}
	opened := ""
	nodes := []FileNode{{ID: "src", Name: "src", Directory: true, Children: []FileNode{{ID: "main", Name: "main.go", Path: "src/main.go", Change: "m"}, {ID: "hidden", Name: "Disabled", Disabled: true}, {ID: "util", Name: "util.go"}}}, {ID: "readme", Name: "README.md"}}
	view := func() *Element {
		return Column("root", FileTree("files", state, nodes, func(n FileNode) { opened = n.Path }), Button("after", "After", nil))
	}
	mount(t, s, view())
	s.Focus("files/node/src")
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	if !state.Open["src"] {
		t.Fatal("folder did not expand")
	}
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	if s.Focused() != "files/node/main" {
		t.Fatal(s.Focused())
	}
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, view())
	if s.Focused() != "files/node/util" {
		t.Fatal("disabled item not skipped", s.Focused())
	}
	s.Key(KeyEvent{Key: "Left"})
	mount(t, s, view())
	if s.Focused() != "files/node/src" {
		t.Fatal("parent navigation failed")
	}
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	nodes[0].Children[0].Path = "mutated"
	s.Key(KeyEvent{Key: "Enter"})
	if opened != "src/main.go" || state.Selected != "main" {
		t.Fatal("snapshot or selection failed", opened, state.Selected)
	}
	mount(t, s, view())
	s.Key(KeyEvent{Key: "Tab"})
	if s.Focused() != "after" {
		t.Fatal("tree not a single tab stop", s.Focused())
	}
	for _, a := range s.Accessibility() {
		if a.ID == "files/node/main" && a.Level != 2 {
			t.Fatal("wrong tree level", a)
		}
	}
	s.Focus("files/node/src")
	s.Key(KeyEvent{Key: "Left"})
	mount(t, s, view())
	if _, ok := s.Bounds("files/node/main"); ok {
		t.Fatal("collapsed descendants still mounted")
	}
}
func TestFileTreeTypeaheadScrollAndReadOnly(t *testing.T) {
	s := surface(t)
	state := &FileTreeState{}
	nodes := []FileNode{{ID: "a", Name: "Alpha"}, {ID: "b", Name: "Beta"}, {ID: "z", Name: "Zebra"}}
	mount(t, s, FileTree("files", state, nodes, nil).Height(50))
	s.Focus("files/node/a")
	s.TextInput("z")
	if s.Focused() != "files/node/z" || state.scroll.Y == 0 {
		t.Fatal("typeahead did not reveal result")
	}
	mount(t, s, FileTree("files", state, nodes, func(FileNode) { t.Fatal("read-only file opened") }).ReadOnly(true))
	s.Focus("files/node/a")
	s.Key(KeyEvent{Key: "Enter"})
}
func TestFileTreeRejectsDuplicateIDs(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate IDs accepted")
		}
	}()
	FileTree("files", &FileTreeState{}, []FileNode{{ID: "same", Children: []FileNode{{ID: "same"}}}}, nil)
}
