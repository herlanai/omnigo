package baseui

import "testing"

func TestGoToLineUnicodeValidationAndReadOnly(t *testing.T) {
	s := surface(t)
	document := NewTextBuffer("你好\r\n世界\n")
	state := &GoToLineState{Query: NewTextBuffer("2")}
	line := 0
	build := func() *Element { return GoToLine("goto", state, document, func(v int) { line = v }) }
	mount(t, s, build())
	click(t, s, "goto/go")
	a, b := document.Selection()
	if a != 4 || b != 4 || line != 2 {
		t.Fatal(a, b, line)
	}
	state.Query.Set("4")
	mount(t, s, build())
	click(t, s, "goto/go")
	if state.Error == "" || line != 2 {
		t.Fatal("invalid line accepted")
	}
	state.Query.Set("3")
	mount(t, s, build().ReadOnly(true))
	click(t, s, "goto/go")
	if line != 2 {
		t.Fatal("readonly navigation")
	}
	mount(t, s, build())
	click(t, s, "goto/go")
	a, b = document.Selection()
	if a != 7 || b != 7 || line != 3 || state.Error != "" {
		t.Fatal(a, b, line, state.Error)
	}
	if document.String() != "你好\r\n世界\n" {
		t.Fatal("document modified")
	}
}
func TestCompletionStaleUnicodeUndoAndDisabled(t *testing.T) {
	doc := NewTextBuffer("你.Pr")
	state := &CompletionState{}
	item := CompletionItem{ID: "print", Label: "Print", InsertText: "Print()"}
	if state.Begin(doc, -1, 2) || state.Begin(doc, 1, 99) {
		t.Fatal("bad range")
	}
	if !state.Begin(doc, 2, 4) || !state.Apply(doc, item) || doc.String() != "你.Print()" {
		t.Fatal("unicode insertion", doc.String())
	}
	doc.Undo()
	if doc.String() != "你.Pr" {
		t.Fatal("undo")
	}
	state.Begin(doc, 2, 4)
	doc.Set("changed")
	if state.Apply(doc, item) {
		t.Fatal("stale edit applied")
	}
	state.Begin(doc, 0, 7)
	item.Disabled = true
	if state.Apply(doc, item) {
		t.Fatal("disabled completion")
	}
	item.Disabled = false
	s := surface(t)
	mount(t, s, CompletionMenu("complete", state, doc, []CompletionItem{item}, nil).ReadOnly(true))
	// Every activation route still goes through the control's inherited read-only guard.
	click(t, s, "complete/item/print")
	if doc.String() != "changed" {
		t.Fatal("readonly edit")
	}
}
