package demos

import (
	"fmt"
	ui "github.com/herlanai/omnigo/omniui/baseui"
	"time"
)

var advancedIDs = []string{"ai-message", "attachment", "avatar", "breadcrumb", "bubble", "chain-of-thought", "chart", "chat", "checkpoint", "confirmation", "context", "drawer", "inline-citation", "item", "marker", "menubar", "message", "message-scroller", "model-selector", "navigation-menu", "plan", "prompt-input", "questionnaire", "queue", "reasoning", "sidebar", "sonner", "sources", "stack-trace", "subagent", "suggestion", "task", "terminal", "test-results", "toast", "tool", "trace"}

type advancedState struct {
	chartKind     ui.ChartKind
	chartCurve    ui.ChartCurve
	disclosure    ui.DisclosureState
	queue         ui.QueueState
	tests         ui.TestResultsState
	confirmation  ui.ConfirmationState
	question      ui.QuestionnaireState
	answer        string
	prompt        ui.PromptInputState
	model         ui.ModelSelectorState
	feedback      ui.MessageFeedback
	menubar       ui.MenubarState
	chart         ui.ChartState
	drawer        ui.DrawerState
	notifications ui.ToastManager
}

func newAdvancedState() *advancedState {
	s := &advancedState{disclosure: ui.DisclosureState{Open: true}, queue: ui.QueueState{Pending: ui.DisclosureState{Open: true}}, tests: ui.TestResultsState{Open: true}, prompt: ui.PromptInputState{Text: ui.NewTextBuffer("Inspect the workspace and explain the changes.")}, model: ui.ModelSelectorState{Model: "balanced", Effort: "medium"}, chart: ui.ChartState{Selected: 2}}
	s.notifications.Add(ui.ToastOptions{Title: "Workspace saved", Description: "All changes are stored locally.", Duration: 5 * time.Second})
	return s
}
func demoSteps() []ui.AIStep {
	return []ui.AIStep{{ID: "inspect", Title: "Inspect workspace", Description: "Read the project structure and build rules.", Status: ui.AIComplete}, {ID: "implement", Title: "Implement controls", Description: "Add state, events and shared Go painting.", Status: ui.AIRunning}, {ID: "verify", Title: "Verify changes", Description: "Run deterministic tests and render both themes.", Status: ui.AIPending}}
}
func demoModels() []ui.ModelOption {
	return []ui.ModelOption{{ID: "balanced", Name: "Balanced", Provider: "Local", Description: "General purpose reasoning and development", Efforts: []ui.ModelEffort{{ID: "low", Name: "Low"}, {ID: "medium", Name: "Medium"}, {ID: "high", Name: "High"}}}, {ID: "fast", Name: "Fast", Provider: "Local", Description: "Quick responses for smaller tasks", Efforts: []ui.ModelEffort{{ID: "low", Name: "Low"}}}, {ID: "offline", Name: "Unavailable", Disabled: true}}
}
func (d *Demo) advancedContent() *ui.Element {
	a := d.advanced
	steps := demoSteps()
	source := ui.Source{ID: "guide", Title: "OmniGo control guide", URL: "https://example.com/controls", Description: "Reference material supplied by the application."}
	openSource := func(v string) { d.Message = "Open callback: " + v }
	nav := []ui.NavigationItem{{ID: "overview", Label: "Overview", Action: func() { d.Choice = "overview" }}, {ID: "settings", Label: "Settings", Action: func() { d.Choice = "settings" }}, {ID: "unavailable", Label: "Unavailable", Disabled: true}}
	switch d.ID {
	case "plan":
		return ui.Plan("demo", "Control migration", "A visible plan with application-owned progress.", steps)
	case "chain-of-thought":
		return ui.ChainOfThought("demo", "Public progress summaries", &a.disclosure, steps)
	case "task":
		return ui.Task("demo", "Rewrite native controls", ui.AIRunning, steps, &a.disclosure)
	case "queue":
		return ui.Queue("demo", steps, &a.queue)
	case "reasoning":
		return ui.Reasoning("demo", "This is a summary supplied by the application. It is not a view into hidden model reasoning.", false, 8*time.Second, &a.disclosure)
	case "tool":
		return ui.Tool("demo", ui.ToolRun{Name: "Go tests", Input: "go test ./omniui/baseui/...", Output: "All tests passed.", Status: ui.AIComplete}, &a.disclosure)
	case "terminal":
		exit := 0
		output := ui.TerminalOutput{Command: "go test ./...", Output: terminalSampleOutput(), ExitCode: &exit}
		return ui.Terminal("demo", output, d.Scroll, ui.TerminalOptions{Open: &a.disclosure, OnCopy: d.copyText})

	case "subagent":
		return ui.Subagent("demo", ui.SubagentRun{Name: "Review controls", Task: "Inspect keyboard and cancellation behavior", Model: "Application model", Status: ui.AIRunning, Steps: steps[:2], Duration: 12 * time.Second}, &a.disclosure)
	case "stack-trace":
		return ui.StackTrace("demo", "Example validation error", []ui.StackFrame{{Function: "validate", File: "controls.go", Line: 42, Source: "return validateOptions(options)"}, {Function: "main", File: "main.go", Line: 18}}, func(f ui.StackFrame) { d.Message = fmt.Sprintf("Open callback: %s:%d", f.File, f.Line) })
	case "test-results":
		return ui.TestResults("demo", []ui.TestSuite{{ID: "controls", Name: "Controls", Tests: []ui.TestResult{{ID: "focus", Name: "Focus navigation", Status: ui.AIComplete}, {ID: "cancel", Name: "Pointer cancellation", Status: ui.AIError, Error: "Example failure details"}}}}, &a.tests)
	case "trace":
		return ui.Trace("demo", "Build trace", &a.disclosure, ui.Text("trace/one", "Read project → build controls → render"), ui.Button("trace/action", "Inspect result", d.click).Variant(ui.Outline))
	case "confirmation":
		return ui.Confirmation("demo", "Apply changes?", "The application decides what happens after approval.", &a.confirmation, func(v bool) { d.Message = fmt.Sprintf("Decision: %t", v) })
	case "checkpoint":
		return ui.Checkpoint("demo", "Before the control rewrite", func() { d.Message = "Restore callback received" })
	case "suggestion":
		return ui.Suggestion("demo", "Explain the current project", func(v string) { d.Message = "Selected prompt: " + v })
	case "questionnaire":
		return ui.Questionnaire("demo", []ui.Question{{ID: "scope", Title: "Choose the scope", Description: "Select one option to continue.", Choices: options[:3], Answer: &a.answer, Required: true}}, &a.question, func() { d.Message = "Submitted: " + a.answer })
	case "attachment":
		return ui.Column("demo", ui.Attachment("file1", "design.png", "Ready to include", ui.AttachmentReady, d.click), ui.Attachment("file2", "notes.txt", "Processing metadata", ui.AttachmentProcessing, nil), ui.Attachment("file3", "archive.zip", "Example upload error", ui.AttachmentError, d.click))
	case "avatar":
		return ui.Row("demo", ui.Avatar("avatar1", "Alex Morgan", nil), ui.Avatar("avatar2", "Sam", nil).Width(48).Height(48), ui.Text("avatar/label", "Application-owned images or initials"))
	case "item":
		return ui.Item("demo", "Native controls", "Go layout, input and painting.", ui.Button("inspect", "Inspect", d.click).Variant(ui.Outline))
	case "bubble":
		return ui.Bubble("demo", "Can you improve the controls and keep the interaction simple?")
	case "marker":
		return ui.Marker("demo", "Today")
	case "message":
		return ui.Message("demo", "You", true, ui.Bubble("bubble", "Build the next iteration with deterministic tests."))
	case "ai-message":
		return ui.AIMessage("demo", "assistant", "The controls are implemented in Go. Next I will verify keyboard navigation, cancellation and both color themes.", ui.AIMessageOptions{Author: "Assistant", Feedback: &a.feedback, OnCopy: func(v string) { d.Message = "Copy callback: " + v }, OnRetry: d.click})
	case "message-scroller", "chat":
		children := []*ui.Element{}
		for i := range 12 {
			children = append(children, ui.Message(fmt.Sprintf("message/%d", i), "Assistant", false, ui.Bubble(fmt.Sprintf("bubble/%d", i), fmt.Sprintf("Update %d: scroll up to inspect earlier messages.", i+1))))
		}
		if d.ID == "chat" {
			return ui.Chat("demo", d.Scroll, children...)
		}
		return ui.MessageScroller("demo", d.Scroll, children...)
	case "sources":
		return ui.Sources("demo", []ui.Source{source, source, {ID: "invalid", Title: "Unsafe URL is disabled", URL: "javascript:alert(1)"}}, &a.disclosure, openSource)
	case "inline-citation":
		return ui.InlineCitation("demo", "[1] Control guide", source, openSource)
	case "context":
		return ui.Context("demo", ui.ContextUsage{Used: 32000, Limit: 128000, Input: 24000, Output: 8000, Cost: "$0.08"})
	case "model-selector":
		return ui.ModelSelector("demo", &a.model, demoModels(), func(m, e string) { d.Message = m + " / " + e })
	case "prompt-input":
		return ui.PromptInput("demo", &a.prompt, ui.PromptInputOptions{Placeholder: "Describe a change", OnAttach: func() {
			a.prompt.Attachments = append(a.prompt.Attachments, ui.PromptAttachment{ID: fmt.Sprintf("file-%d", d.Count), Name: "notes.txt", Description: "Application metadata"})
			d.Count++
		}, OnSubmit: func(v ui.PromptSubmission) { d.Message = "Submitted: " + v.Text }})
	case "navigation-menu":
		return ui.NavigationMenu("demo", d.Choice, nav)
	case "sidebar":
		return ui.Sidebar("demo", d.Choice, nav)
	case "breadcrumb":
		return ui.Breadcrumb("demo", nav[:2])
	case "menubar":
		return ui.Menubar("demo", &a.menubar, []ui.MenubarMenu{{ID: "file", Label: "File", Items: []ui.MenuItem{{ID: "new", Label: "New workspace", Action: d.click}, {ID: "save", Label: "Save", Action: d.click}}}, {ID: "view", Label: "View", Items: []ui.MenuItem{{ID: "details", Label: "Details", Checked: &d.Enabled}}}})
	case "chart":
		return ui.Column("chart-demo", ui.Row("chart-controls", ui.Button("chart-kind", "Change chart type", func() { a.chartKind = (a.chartKind + 1) % 3 }).Variant(ui.Outline), ui.Button("chart-curve", "Change interpolation", func() { a.chartCurve = (a.chartCurve + 1) % 5 }).Variant(ui.Outline)), ui.Chart("demo", []string{"Mon", "Tue", "Wed", "Thu", "Fri"}, []ui.ChartSeries{{Label: "Completed", Values: []float64{4, 7, 5, 9, 12}}, {Label: "Pending", Values: []float64{8, 5, 6, 3, 2}}}, &a.chart, ui.ChartOptions{Kind: a.chartKind, Curve: a.chartCurve, ShowYAxis: true, Label: "Workspace activity", OnSelect: func(v int) { d.Message = fmt.Sprintf("Selected category %d", v) }}))
	case "drawer":
		return ui.Column("demo", ui.Button("open", "Open drawer", func() { a.drawer.Open = true }), ui.Drawer("drawer", "Workspace settings", &a.drawer, nil, ui.Text("drawer/help", "Drag the handle down to dismiss, or press Escape."), ui.Input("drawer/input", "Workspace name", d.Buffer)))
	case "toast":
		return ui.Toast("demo", ui.ToastOptions{Title: "Workspace saved", Description: "Your changes are stored locally.", ActionLabel: "Undo", Action: d.click}, func() { d.Message = "Dismiss callback received" })
	case "sonner":
		return ui.Column("demo", ui.Button("notify", "Show notification", func() {
			a.notifications.Add(ui.ToastOptions{Title: "Changes saved", Description: "Hover or focus to pause the five-second timeout.", Duration: 5 * time.Second, ActionLabel: "Undo", Action: d.click})
		}), ui.Sonner("notifications", &a.notifications))
	}
	return nil
}

// Match the read-only Forma Terminal demonstration fixture.
func terminalSampleOutput() string {
	return "ok  example/backend\nok  example/ui"
}
