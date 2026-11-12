package baseui

import "testing"

func TestDataTableFilterSortPageSelectionAndVisibility(t *testing.T) {
	state := NewDataTableState(2)
	columns := []TableColumn{{ID: "name", Title: "Name", Width: 200, Sortable: true}}
	rows := []DataRow{{"a", map[string]string{"name": "Charlie"}}, {"b", map[string]string{"name": "Alpha"}}, {"c", map[string]string{"name": "Bravo"}}}
	s, _ := NewSurface(640, 480, NewTheme(false))
	defer s.Close()
	readonly := false
	view := func() *Element { return DataTable("table", state, columns, rows).ReadOnly(readonly) }
	rebuild := func() { mount(t, s, view()) }
	press := func(id string) {
		t.Helper()
		if !s.Focus(id) {
			t.Fatal("not focusable", id)
		}
		if err := s.Key(KeyEvent{Key: "Enter"}); err != nil {
			t.Fatal(err)
		}
		rebuild()
	}
	rebuild()
	press("table/head/name")
	if _, ok := s.Bounds("table/row/a"); ok {
		t.Fatal("sort did not reorder page")
	}
	press("table/select/b")
	press("table/next")
	if !state.Selected["b"] || state.Page != 1 {
		t.Fatal("selection lost across page")
	}
	state.Query.Set("char")
	rebuild()
	if state.Page != 0 {
		t.Fatal("filter did not reset page")
	}
	if _, ok := s.Bounds("table/row/a"); !ok {
		t.Fatal("filtered row missing")
	}
	state.Query.Set("missing")
	rebuild()
	if _, ok := s.Bounds("table/empty"); !ok {
		t.Fatal("empty state missing")
	}
	state.Query.Set("")
	rebuild()
	state.HiddenColumns["name"] = true
	rebuild()
	if _, ok := s.Bounds("table/head/name"); ok {
		t.Fatal("hidden column remains")
	}
	state.HiddenColumns["name"] = false
	readonly = true
	rebuild()
	before := state.Descending
	press("table/head/name")
	if state.Descending != before {
		t.Fatal("read-only sorting changed")
	}
	rows = []DataRow{rows[0], rows[2]}
	rebuild()
	if state.Selected["b"] {
		t.Fatal("removed selection retained")
	}
}
