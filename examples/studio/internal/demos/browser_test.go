package demos

import (
	"context"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"strings"
	"testing"
)

func TestBrowserDemoRunsActualDOMAndScreenshot(t *testing.T) {
	d := New("plugin/omnibrowser")
	defer d.Close()
	s, err := ui.NewSurface(1040, 840, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mount := func() {
		t.Helper()
		if err := s.Mount(d.View()); err != nil {
			t.Fatal(err)
		}
	}
	activate := func(id string) {
		t.Helper()
		if !s.Focus(id) {
			t.Fatal("missing action", id)
		}
		if err := s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
			t.Fatal(err)
		}
	}
	mount()
	if d.browser == nil || !strings.Contains(d.browser.snapshot.Text, "Browse with Go") {
		t.Fatal("missing real page")
	}
	for _, node := range s.Accessibility() {
		if node.Role == "button" && node.Label == "Run page action" {
			activate(node.ID)
			break
		}
	}
	if !strings.Contains(d.browser.snapshot.Text, "Hello, OmniGo!") {
		t.Fatal("page action did not execute", d.browser.snapshot.Text)
	}
	d.browser.tab = "script"
	mount()
	activate("browser/eval")
	if !strings.Contains(d.browser.output, "DOM selectors") {
		t.Fatal(d.browser.output)
	}
	d.browser.tab = "extract"
	mount()
	activate("browser/query")
	if !strings.Contains(d.browser.output, `"tag": "li"`) {
		t.Fatal(d.browser.output)
	}
	d.browser.tab = "screenshot"
	mount()
	activate("browser/capture")
	if d.browser.shot == nil || d.browser.shot.Bounds().Dx() != 800 {
		t.Fatal("missing screenshot")
	}
	d.Close()
	if _, err := d.browser.page.Eval(context.Background(), "1"); err == nil {
		t.Fatal("page not closed")
	}
}
func TestBrowserDemoAppliesWorkerResultOnlyOnDispatch(t *testing.T) {
	d := New("plugin/omnibrowser")
	defer d.Close()
	d.View()
	d.browser.tab = "script"
	d.browser.script.Set(`document.title='Worker title'`)
	var work func() func()
	d.RunTask = func(task func() func()) { work = task }
	s, err := ui.NewSurface(1040, 840, ui.NewTheme(false))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Mount(d.View()); err != nil {
		t.Fatal(err)
	}
	s.Focus("browser/eval")
	if err = s.Key(ui.KeyEvent{Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	if work == nil || !d.browser.busy {
		t.Fatal("task not scheduled")
	}
	done := make(chan func(), 1)
	go func() { done <- work() }()
	apply := <-done
	if d.browser.snapshot.Title == "Worker title" {
		t.Fatal("UI mutated by worker")
	}
	apply()
	if d.browser.busy || d.browser.snapshot.Title != "Worker title" {
		t.Fatal("dispatch result missing")
	}
}
