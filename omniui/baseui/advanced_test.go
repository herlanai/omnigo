package baseui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"
)

func TestMessageTailFollowingAndWrappedContent(t *testing.T) {
	s := surface(t)
	state := &ScrollState{}
	text := strings.Repeat("A message with words that wrap across the viewport.\n", 20)
	view := func() *Element {
		return MessageScroller("messages", state, paragraph("text", text)).Width(200).Height(100)
	}
	mount(t, s, view())
	if state.Y != state.maxY || state.Y == 0 {
		t.Fatal("did not start at tail", state)
	}
	text += "Appended\n"
	mount(t, s, view())
	if state.Y != state.maxY {
		t.Fatal("did not follow")
	}
	s.ScrollTo("messages", 0, 20)
	text += "Another appended line\n"
	mount(t, s, view())
	if state.Y != 20 {
		t.Fatal("yanked reader", state.Y)
	}
	lines := s.textLines(paragraph("wrap", "hello world\n\nlongwordlongword"), 50)
	if strings.Join(lines, "") == "" || len(lines) < 5 {
		t.Fatal(lines)
	}
	text = "short"
	mount(t, s, view())
	if state.Y != 0 {
		t.Fatal("shrink offset")
	}
}
func TestConfirmationIsOneShotAndReadOnly(t *testing.T) {
	s := surface(t)
	state := ConfirmationPending
	calls := 0
	view := func() *Element {
		return Confirmation("confirm", "Apply?", "Change files", &state, func(v bool) {
			calls++
			if !v {
				t.Fatal("wrong decision")
			}
		})
	}
	mount(t, s, view().ReadOnly(true))
	click(t, s, "confirm/approve")
	if calls != 0 {
		t.Fatal("read-only decision")
	}
	mount(t, s, view())
	click(t, s, "confirm/approve")
	click(t, s, "confirm/deny")
	if calls != 1 || state != ConfirmationApproved {
		t.Fatal(state, calls)
	}
	mount(t, s, view())
	if s.Focus("confirm/approve") {
		t.Fatal("decided controls focusable")
	}
}
func TestQuestionnaireRequiredChoicesAndSubmission(t *testing.T) {
	s := surface(t)
	state := &QuestionnaireState{Index: -10}
	a, b := "", ""
	calls := 0
	questions := []Question{{ID: "one", Title: "One", Required: true, Answer: &a, Choices: []Option{{ID: "yes", Label: "Yes"}, {ID: "no", Label: "Unavailable", Disabled: true}}}, {ID: "two", Title: "Two", Required: true, Answer: &b, Choices: []Option{{ID: "ok", Label: "OK"}}}}
	view := func() *Element { return Questionnaire("q", questions, state, func() { calls++ }) }
	mount(t, s, view())
	click(t, s, "q/next")
	if state.Index != 0 {
		t.Fatal("required skipped")
	}
	click(t, s, "q/answers/yes")
	mount(t, s, view())
	click(t, s, "q/next")
	mount(t, s, view())
	click(t, s, "q/answers/ok")
	mount(t, s, view())
	click(t, s, "q/next")
	click(t, s, "q/next")
	if calls != 1 || !state.Submitted {
		t.Fatal(calls, state)
	}
}
func TestPromptCompositionSnapshotBusyAndFailure(t *testing.T) {
	s := surface(t)
	state := &PromptInputState{Text: NewTextBuffer("hello"), Attachments: []PromptAttachment{{ID: "a", Name: "before"}}}
	var got PromptSubmission
	calls, stops := 0, 0
	options := PromptInputOptions{OnSubmit: func(v PromptSubmission) { calls++; got = v }, OnStop: func() { stops++ }}
	view := func() *Element { return PromptInput("prompt", state, options) }
	mount(t, s, view())
	s.Focus("prompt/text")
	s.SetComposition("preedit")
	s.Key(KeyEvent{Key: "Enter"})
	if calls != 0 || state.Text.Composition() == "" {
		t.Fatal("IME submitted")
	}
	s.CommitComposition()
	s.Key(KeyEvent{Key: "Enter", Shift: true})
	if !strings.Contains(state.Text.String(), "\n") {
		t.Fatal("shift enter")
	}
	s.Key(KeyEvent{Key: "Enter"})
	if calls != 1 {
		t.Fatal(calls)
	}
	state.Attachments[0].Name = "after"
	if got.Attachments[0].Name != "before" {
		t.Fatal("mutable submission")
	}
	options.Status = PromptStreaming
	mount(t, s, view())
	if s.Focus("prompt/text") {
		t.Fatal("busy editor focusable")
	}
	click(t, s, "prompt/attachment/a/remove")
	if len(state.Attachments) != 1 {
		t.Fatal("busy removed attachment")
	}
	click(t, s, "prompt/stop")
	if stops != 1 {
		t.Fatal(stops)
	}
	options.Status = PromptReady
	options.Error = "Retry later"
	mount(t, s, view())
	if state.Text.String() == "" {
		t.Fatal("failure cleared prompt")
	}
	click(t, s, "prompt/attachment/a/remove")
	if len(state.Attachments) != 0 {
		t.Fatal("remove failed")
	}
}
func TestModelSelectorRetainsOnlySupportedEffort(t *testing.T) {
	s := surface(t)
	state := &ModelSelectorState{Model: "a", Effort: "high"}
	calls := 0
	models := []ModelOption{{ID: "a", Name: "A", Efforts: []ModelEffort{{ID: "high", Name: "High"}}}, {ID: "b", Name: "B", Efforts: []ModelEffort{{ID: "off", Name: "Disabled", Disabled: true}, {ID: "low", Name: "Low"}}}}
	view := func() *Element { return ModelSelector("models", state, models, func(m, e string) { calls++ }) }
	mount(t, s, view())
	click(t, s, "models/model/trigger")
	mount(t, s, view())
	click(t, s, "models/model/option/b")
	if state.Model != "b" || state.Effort != "low" || calls != 1 {
		t.Fatal(state, calls)
	}
}
func TestSourcesRejectUnsafeURLsAndDeduplicate(t *testing.T) {
	s := surface(t)
	calls := 0
	state := &DisclosureState{Open: true}
	mount(t, s, Sources("sources", []Source{{Title: "Good", URL: "https://example.com/doc"}, {Title: "Duplicate", URL: "https://example.com/doc"}, {Title: "Bad", URL: "javascript:alert(1)"}}, state, func(string) { calls++ }))
	click(t, s, "sources/source/0/open")
	click(t, s, "sources/source/1/open")
	if calls != 1 {
		t.Fatal(calls)
	}
	if _, ok := s.Bounds("sources/source/2"); ok {
		t.Fatal("not deduplicated")
	}
	for _, url := range []string{"file:///tmp/foo", "https:///nohost", "https://user:password@example.com", "data:text/plain,x"} {
		if sourceURL(url) {
			t.Fatal(url)
		}
	}
}
func TestWorkflowGroupsAndSuiteDefaults(t *testing.T) {
	s := surface(t)
	steps := []AIStep{{ID: "a", Title: "Done", Status: AIComplete}, {ID: "b", Title: "Work", Status: AIRunning}, {ID: "c", Title: "Cancelled", Status: AICancelled}}
	state := &QueueState{Pending: DisclosureState{Open: true}, Completed: DisclosureState{Open: true}}
	mount(t, s, Queue("queue", steps, state))
	if _, ok := s.Bounds("queue/completed/items/c"); !ok {
		t.Fatal("cancelled missing")
	}
	click(t, s, "queue/pending/trigger")
	mount(t, s, Queue("queue", steps, state))
	if !state.Completed.Open || state.Pending.Open {
		t.Fatal("sections not independent")
	}
	tests := &TestResultsState{Open: true}
	suites := []TestSuite{{ID: "suite", Name: "Suite", Tests: []TestResult{{ID: "one", Name: "One", Status: AIError, Error: "failure"}}}}
	mount(t, s, TestResults("tests", suites, tests))
	if _, ok := s.Bounds("tests/suite/suite/tests/one"); !ok {
		t.Fatal("failed suite not expanded")
	}
	click(t, s, "tests/suite/suite/trigger")
	mount(t, s, TestResults("tests", suites, tests))
	if _, ok := s.Bounds("tests/suite/suite/tests/one"); ok {
		t.Fatal("collapse ignored")
	}
}
func TestChartSelectionGapsSnapshotAndExtremeBounds(t *testing.T) {
	s := surface(t)
	state := &ChartState{Selected: -1}
	series := []ChartSeries{{Label: "Values", Values: []float64{-math.MaxFloat64, math.NaN(), math.MaxFloat64}}}
	calls := 0
	view := Chart("chart", []string{"A", "B", "C"}, series, state, ChartOptions{Kind: ChartLine, Grid: true, OnSelect: func(int) { calls++ }}).Width(400)
	mount(t, s, view)
	series[0].Values[0] = 42
	s.Focus("chart/plot")
	s.Key(KeyEvent{Key: "End"})
	if state.Selected != 2 {
		t.Fatal(state)
	}
	s.Key(KeyEvent{Key: "Escape"})
	if state.Selected != -1 {
		t.Fatal("clear")
	}
	r, _ := s.Bounds("chart/plot")
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 65, Y: r.Y + 40})
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 65, Y: r.Y + 40})
	if state.Selected != 0 || calls != 3 {
		t.Fatal(state, calls)
	}
	if _, err := s.RenderScaled(1.25, nil); err != nil {
		t.Fatal(err)
	}
	mount(t, s, view.ReadOnly(true))
	s.Focus("chart/plot")
	s.Key(KeyEvent{Key: "End"})
	if state.Selected != 0 {
		t.Fatal("read-only selection")
	}
}
func TestAvatarImageSnapshotAndAlpha(t *testing.T) {
	s := surface(t)
	source := image.NewNRGBA(image.Rect(5, 7, 13, 15))
	for y := 7; y < 15; y++ {
		for x := 5; x < 13; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 255, A: 128})
		}
	}
	e := Avatar("avatar", "Test", source)
	source.SetNRGBA(9, 11, color.NRGBA{B: 255, A: 255})
	mount(t, s, Column("root", e))
	img, err := s.Render()
	if err != nil {
		t.Fatal(err)
	}
	c := img.RGBAAt(16, 16)
	if c.R <= c.B {
		t.Fatal("source mutated snapshot", c)
	}
}
func TestDrawerDragCancelDismissAndFocusReturn(t *testing.T) {
	s := surface(t)
	state := &DrawerState{Open: true}
	calls := 0
	view := func() *Element {
		return Column("root", Button("open", "Open", nil), Drawer("drawer", "Settings", state, func() { calls++ }, Text("body", "Content")))
	}
	mount(t, s, view())
	r, _ := s.Bounds("drawer/handle")
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 20, Y: r.Y + 10})
	s.Pointer(PointerEvent{Kind: "move", X: r.X + 20, Y: r.Y + 80})
	if state.offset != 70 {
		t.Fatal(state.offset)
	}
	s.Pointer(PointerEvent{Kind: "cancel"})
	if state.offset != 0 || !state.Open {
		t.Fatal("cancel")
	}
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 20, Y: r.Y + 10})
	s.Pointer(PointerEvent{Kind: "up", X: r.X + 20, Y: r.Y + 150})
	if state.Open || calls != 1 {
		t.Fatal(state, calls)
	}
	mount(t, s, view())
	if s.modal != "" {
		t.Fatal("retained modal")
	}
}
func TestToastDeadlinePauseActionAndUnmount(t *testing.T) {
	s := surface(t)
	m := &ToastManager{}
	calls := 0
	m.Add(ToastOptions{ID: "one", Title: "Saved", Duration: time.Second, ActionLabel: "Undo", Action: func() { calls++ }})
	view := func() *Element { return Column("root", Button("outside", "Outside", nil), Sonner("notifications", m)) }
	mount(t, s, view())
	if s.modal != "" {
		t.Fatal("notifications trapped focus")
	}
	if d, ok := s.NextWakeup(); !ok || d != time.Second {
		t.Fatal(d, ok)
	}
	s.Advance(400 * time.Millisecond)
	r, _ := s.Bounds("notifications/one/dismiss")
	s.Pointer(PointerEvent{Kind: "move", X: r.X + 5, Y: r.Y + 5})
	s.Advance(3 * time.Second)
	if m.Count() != 1 {
		t.Fatal("hover did not pause")
	}
	s.Pointer(PointerEvent{Kind: "leave"})
	if d, ok := s.NextWakeup(); !ok || d != 600*time.Millisecond {
		t.Fatal(d, ok)
	}
	s.Advance(3600 * time.Millisecond)
	if m.Count() != 0 {
		t.Fatal("deadline did not expire")
	}
	if _, ok := s.Bounds("notifications/one"); ok {
		t.Fatal("expired remained mounted")
	}
	m.Add(ToastOptions{ID: "two", Title: "New", Duration: time.Second, ActionLabel: "Undo", Action: func() { calls++ }})
	mount(t, s, view())
	click(t, s, "notifications/two/action")
	click(t, s, "notifications/two/action")
	if calls != 1 {
		t.Fatal("repeated action", calls)
	}
	mount(t, s, Button("alone", "Alone", nil))
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("unmounted timer")
	}
}
func TestNavigationRovingAndDisabled(t *testing.T) {
	s := surface(t)
	calls := 0
	items := []NavigationItem{{ID: "one", Label: "One", Action: func() { calls++ }}, {ID: "off", Label: "Off", Disabled: true}, {ID: "two", Label: "Two", Action: func() { calls++ }}}
	view := func() *Element { return Sidebar("nav", "one", items) }
	mount(t, s, view())
	s.Focus("nav/one")
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, view())
	if s.focus != "nav/two" {
		t.Fatal(s.focus)
	}
	if calls != 0 {
		t.Fatal("navigation activated on focus")
	}
	s.Key(KeyEvent{Key: "Enter"})
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestMenubarSwitchesOpenMenusAndSkipsDisabled(t *testing.T) {
	s := surface(t)
	state := &MenubarState{}
	menus := []MenubarMenu{{ID: "file", Label: "File", Items: []MenuItem{{ID: "new", Label: "New"}}}, {ID: "disabled", Label: "Disabled", Disabled: true}, {ID: "view", Label: "View", Items: []MenuItem{{ID: "details", Label: "Details"}}}}
	view := func() *Element { return Menubar("bar", state, menus) }
	mount(t, s, view())
	s.Focus("bar/file/trigger")
	s.Key(KeyEvent{Key: "Down"})
	mount(t, s, view())
	if s.focus != "bar/file/item/new" {
		t.Fatal(s.focus)
	}
	s.Key(KeyEvent{Key: "Right"})
	mount(t, s, view())
	if s.focus != "bar/view/item/details" || state.menus["file"].Open {
		t.Fatal("menu did not switch", s.focus)
	}
	s.Key(KeyEvent{Key: "Escape"})
	mount(t, s, view())
	if s.modal != "" {
		t.Fatal("escape failed")
	}
}
func TestPointerCaptureCancelOnUnmountAndDisable(t *testing.T) {
	s := surface(t)
	state := &DrawerState{Open: true}
	view := func() *Element { return Drawer("drawer", "Drag", state, nil, Text("body", "Content")) }
	mount(t, s, view())
	r, _ := s.Bounds("drawer/handle")
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 10, Y: r.Y + 10})
	s.Pointer(PointerEvent{Kind: "move", X: r.X + 10, Y: r.Y + 60})
	mount(t, s, Button("other", "Other", nil))
	if state.dragging || state.offset != 0 {
		t.Fatal("unmounted capture retained")
	}
	mount(t, s, view())
	r, _ = s.Bounds("drawer/handle")
	s.Pointer(PointerEvent{Kind: "down", X: r.X + 10, Y: r.Y + 10})
	mount(t, s, view().ReadOnly(true))
	if state.dragging {
		t.Fatal("read-only capture retained")
	}
}
func TestToastBackgroundPauseUpdateAndPersistent(t *testing.T) {
	s := surface(t)
	m := &ToastManager{}
	m.Add(ToastOptions{ID: "toast-1", Title: "Persistent"})
	id := m.Add(ToastOptions{Title: "Temporary", Duration: time.Second})
	if id == "toast-1" {
		t.Fatal("generated ID collision")
	}
	view := func() *Element { return Sonner("notifications", m) }
	mount(t, s, view())
	s.Advance(250 * time.Millisecond)
	s.SetActive(false)
	s.Advance(10 * time.Second)
	s.SetActive(true)
	if d, ok := s.NextWakeup(); !ok || d != 750*time.Millisecond {
		t.Fatal(d, ok)
	}
	s.Advance(10750 * time.Millisecond)
	if m.Count() != 1 {
		t.Fatal(m.Count())
	}
	if _, ok := s.NextWakeup(); ok {
		t.Fatal("persistent toast polling")
	}
	m.Add(ToastOptions{ID: "toast-1", Title: "Updated", Duration: time.Second})
	mount(t, s, view())
	s.Advance(11750 * time.Millisecond)
	if m.Count() != 0 {
		t.Fatal("update did not expire")
	}
}
func TestRowWrappingReservesActionsAtNarrowWidths(t *testing.T) {
	s := surface(t)
	mount(t, s, Item("item", "Title", strings.Repeat("Long text ", 12), Button("action", "Open", nil)).Width(260))
	root, _ := s.Bounds("item")
	action, _ := s.Bounds("action")
	body, _ := s.Bounds("item/body")
	if action.X+action.W > root.X+root.W || body.X+body.W > action.X {
		t.Fatal(root, action, body)
	}
	if body.H < 80 {
		t.Fatal("text was not remeasured after width assignment", body)
	}
}
