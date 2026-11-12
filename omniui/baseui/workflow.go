package baseui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AIStatus describes application-owned work. These controls never execute jobs.
type AIStatus string

const (
	AIPending   AIStatus = "pending"
	AIRunning   AIStatus = "running"
	AIComplete  AIStatus = "complete"
	AIError     AIStatus = "error"
	AICancelled AIStatus = "cancelled"
)

type AIStep struct {
	ID, Title, Description string
	Status                 AIStatus
}
type DisclosureState struct{ Open bool }

func statusLabel(s AIStatus) string {
	switch s {
	case AIPending, AIRunning, AIComplete, AIError, AICancelled:
		return string(s)
	default:
		return "pending"
	}
}
func stepList(id string, steps []AIStep) *Element {
	children := make([]*Element, 0, len(steps))
	seen := map[string]bool{}
	for _, step := range steps {
		if step.ID == "" || seen[step.ID] {
			panic("baseui: step IDs must be unique and nonempty")
		}
		seen[step.ID] = true
		p := id + "/" + step.ID
		label := Badge(p+"/status", statusLabel(step.Status)).Variant(Secondary)
		if step.Status == AIError {
			label.Variant(Destructive)
		}
		title := Row(p+"/header", Text(p+"/title", step.Title).FontWeight(600).Grow(1), label)
		children = append(children, Column(p, title, paragraph(p+"/description", step.Description).Role("note")).Gap(4))
	}
	return Column(id, children...).Role("list")
}
func disclosure(id, title string, state *DisclosureState, children ...*Element) *Element {
	if state == nil {
		panic("baseui: nil disclosure state")
	}
	e := Collapsible(id, title, state.Open, func(v bool) { state.Open = v }, children...)
	e.children[0].expanded = state.Open
	e.children[0].controls = id + "/content"
	return e
}

// ChainOfThought displays only summaries explicitly supplied by the application.
func ChainOfThought(id, title string, state *DisclosureState, steps []AIStep) *Element {
	done := 0
	for _, s := range steps {
		if s.Status == AIComplete {
			done++
		}
	}
	return disclosure(id, fmt.Sprintf("%s (%d/%d)", title, done, len(steps)), state, stepList(id+"/steps", steps))
}
func Reasoning(id, text string, running bool, duration time.Duration, state *DisclosureState) *Element {
	label := "Reasoning"
	if running {
		label = "Reasoning in progress"
	} else if duration > 0 {
		label += " · " + duration.Round(time.Second).String()
	}
	return disclosure(id, label, state, paragraph(id+"/text", text))
}
func Plan(id, title, description string, steps []AIStep) *Element {
	done := 0
	for _, s := range steps {
		if s.Status == AIComplete {
			done++
		}
	}
	fraction := 0.
	if len(steps) > 0 {
		fraction = float64(done) / float64(len(steps))
	}
	return Card(id, Heading(id+"/title", title, 4), paragraph(id+"/description", description).Role("note"), Progress(id+"/progress", fmt.Sprintf("%d of %d completed", done, len(steps)), fraction), stepList(id+"/steps", steps)).Role("group")
}
func Task(id, title string, status AIStatus, steps []AIStep, state *DisclosureState) *Element {
	return disclosure(id, title+" · "+statusLabel(status), state, stepList(id+"/steps", steps))
}

type QueueState struct{ Pending, Completed DisclosureState }

func Queue(id string, steps []AIStep, state *QueueState) *Element {
	if state == nil {
		panic("baseui: nil queue state")
	}
	pending, completed := []AIStep{}, []AIStep{}
	for _, s := range steps {
		if s.Status == AIComplete || s.Status == AICancelled {
			completed = append(completed, s)
		} else {
			pending = append(pending, s)
		}
	}
	// Validate globally, including duplicate IDs across both groups.
	_ = stepList(id+"/validate", steps)
	return Column(id, disclosure(id+"/pending", fmt.Sprintf("Pending (%d)", len(pending)), &state.Pending, stepList(id+"/pending/items", pending)), disclosure(id+"/completed", fmt.Sprintf("Completed (%d)", len(completed)), &state.Completed, stepList(id+"/completed/items", completed)))
}

type ToolRun struct {
	Name, Input, Output, Error string
	Status                     AIStatus
}

func Tool(id string, run ToolRun, state *DisclosureState) *Element {
	status := run.Status
	if run.Error != "" {
		status = AIError
	}
	body := []*Element{Text(id+"/input", run.Input), Text(id+"/output", run.Output)}
	if run.Error != "" {
		body = append(body, Text(id+"/error", run.Error).Role("alert"))
	}
	return disclosure(id, run.Name+" · "+statusLabel(status), state, body...)
}

type TerminalOutput struct {
	Command, Output string
	Running         bool
	ExitCode        *int
}

// TerminalOptions binds disclosure and copy actions to application-owned state.
// A nil Open keeps the output visible for existing callers.
type TerminalOptions struct {
	Open      *DisclosureState
	OnCopy    func(string)
	MaxHeight float64
}

// Terminal displays agent command output. It does not implement a PTY or VT emulator.
func Terminal(id string, output TerminalOutput, scroll *ScrollState, options ...TerminalOptions) *Element {
	if scroll == nil {
		panic("baseui: nil terminal scroll state")
	}
	var o TerminalOptions
	if len(options) > 0 {
		o = options[0]
	}
	status := AIPending
	title := output.Command
	if output.Running {
		status = AIRunning
	} else if output.ExitCode != nil {
		status = AIComplete
		if *output.ExitCode != 0 {
			status = AIError
		}
		title += fmt.Sprintf(" · exit %d", *output.ExitCode)
	}
	header := Text(id+"/command", title).Grow(1)
	if o.Open != nil {
		header = Button(id+"/toggle", title, func() { o.Open.Open = !o.Open.Open }).Variant(Ghost).Grow(1)
		header.alignStart = true
		header.branch = true
		header.textIndent = 16
		header.expanded = o.Open.Open
		header.controls = id + "/output"
	}
	badge := Badge(id+"/status", statusLabel(status)).Variant(Secondary)
	if status == AIError {
		badge.Variant(Destructive)
	}
	controls := []*Element{Text(id+"/title", "Terminal").FontSize(13).FontWeight(600).Grow(1)}
	if o.OnCopy != nil {
		controls = append(controls, Button(id+"/copy", "Copy", func() { o.OnCopy(output.Output) }).Variant(Ghost))
	}
	height := o.MaxHeight
	if height <= 0 {
		height = 320
	}
	text := codeText(id+"/text", output.Output, status == AIError)
	viewport := ScrollArea(id+"/viewport", scroll, text).HorizontalScroll(true).Height(min(height, text.height+10))
	viewport.followTail = true
	actions := Row(id+"/actions", controls...)
	actions.centerItems = true
	body := Column(id+"/output", actions, viewport).Gap(8)
	if o.Open != nil {
		body.Hidden(!o.Open.Open)
	}
	heading := Row(id+"/header", header, badge)
	heading.centerItems = true
	return Card(id, heading, body).Gap(8).Role("log")
}

type SubagentRun struct {
	Name, Task, Model, Result string
	Status                    AIStatus
	Steps                     []AIStep
	Duration                  time.Duration
}

func Subagent(id string, run SubagentRun, state *DisclosureState) *Element {
	return disclosure(id, run.Name+" · "+statusLabel(run.Status), state, paragraph(id+"/task", run.Task), Text(id+"/model", run.Model+" · "+max(time.Duration(0), run.Duration).Round(time.Second).String()).Role("note"), stepList(id+"/steps", run.Steps), paragraph(id+"/result", run.Result))
}

type StackFrame struct {
	Function, File, Source string
	Line                   int
	External               bool
}

func StackTrace(id, message string, frames []StackFrame, open func(StackFrame)) *Element {
	children := []*Element{Text(id+"/message", message).Role("alert")}
	for i, f := range frames {
		f := f
		p := id + "/" + strconv.Itoa(i)
		label := fmt.Sprintf("%s — %s:%d", f.Function, f.File, f.Line)
		if f.External {
			label += " (external)"
		}
		children = append(children, Column(p, Button(p+"/open", label, func() {
			if open != nil {
				open(f)
			}
		}).Variant(Ghost).Disabled(open == nil || f.File == ""), Text(p+"/source", f.Source)))
	}
	return Card(id, children...)
}

type TestResult struct {
	ID, Name, Error string
	Status          AIStatus
}
type TestSuite struct {
	ID, Name string
	Tests    []TestResult
}
type TestResultsState struct {
	Open   bool
	Suites map[string]bool
}

func TestResults(id string, suites []TestSuite, state *TestResultsState) *Element {
	if state == nil {
		panic("baseui: nil test results state")
	}
	if state.Suites == nil {
		state.Suites = map[string]bool{}
	}
	passed, failed, total := 0, 0, 0
	children := []*Element{}
	seen := map[string]bool{}
	for _, suite := range suites {
		if suite.ID == "" || seen[suite.ID] {
			panic("baseui: duplicate or empty suite ID")
		}
		seen[suite.ID] = true
		steps := []AIStep{}
		hasError := false
		for _, test := range suite.Tests {
			total++
			if test.Status == AIComplete {
				passed++
			}
			if test.Status == AIError {
				failed++
				hasError = true
			}
			steps = append(steps, AIStep{ID: test.ID, Title: test.Name, Description: test.Error, Status: test.Status})
		}
		expanded, known := state.Suites[suite.ID]
		if !known {
			expanded = hasError
		}
		sid := suite.ID
		children = append(children, Collapsible(id+"/suite/"+sid, suite.Name, expanded, func(v bool) { state.Suites[sid] = v }, stepList(id+"/suite/"+sid+"/tests", steps)))
	}
	return Collapsible(id, fmt.Sprintf("Tests: %d passed, %d failed, %d total", passed, failed, total), state.Open, func(v bool) { state.Open = v }, children...)
}
func Trace(id, title string, state *DisclosureState, children ...*Element) *Element {
	return disclosure(id, title, state, children...)
}

type ConfirmationState string

const (
	ConfirmationPending  ConfirmationState = ""
	ConfirmationApproved ConfirmationState = "approved"
	ConfirmationDenied   ConfirmationState = "denied"
)

// Confirmation commits one decision until the application explicitly resets it.
func Confirmation(id, title, description string, state *ConfirmationState, decide func(bool)) *Element {
	if state == nil {
		panic("baseui: nil confirmation state")
	}
	action := func(approve bool) func() {
		return func() {
			if *state != ConfirmationPending || decide == nil {
				return
			}
			*state = ConfirmationDenied
			if approve {
				*state = ConfirmationApproved
			}
			decide(approve)
		}
	}
	return Card(id, Heading(id+"/title", title, 4), paragraph(id+"/description", description), Row(id+"/actions", Button(id+"/deny", "Deny", action(false)).Variant(Outline), Button(id+"/approve", "Approve", action(true))).Disabled(*state != ConfirmationPending || decide == nil), Text(id+"/status", string(*state)).Role("status"))
}
func Checkpoint(id, label string, restore func()) *Element {
	return Row(id, Text(id+"/label", label).Grow(1), Button(id+"/restore", "Restore", restore).Variant(Outline).Disabled(restore == nil)).Role("group")
}
func Suggestion(id, prompt string, choose func(string)) *Element {
	return Button(id, prompt, func() {
		if choose != nil {
			choose(prompt)
		}
	}).Variant(Outline).Disabled(choose == nil)
}

type Question struct {
	ID, Title, Description string
	Choices                []Option
	Answer                 *string
	Required               bool
}
type QuestionnaireState struct {
	Index     int
	Submitted bool
}

func Questionnaire(id string, questions []Question, state *QuestionnaireState, submit func()) *Element {
	if state == nil {
		panic("baseui: nil questionnaire state")
	}
	seen := map[string]bool{}
	for _, q := range questions {
		if q.ID == "" || seen[q.ID] || q.Answer == nil {
			panic("baseui: invalid question")
		}
		seen[q.ID] = true
		validateOptions(q.Choices)
	}
	if len(questions) == 0 {
		return Empty(id, "No questions", "")
	}
	state.Index = max(0, min(state.Index, len(questions)-1))
	q := questions[state.Index]
	valid := func(q Question) bool {
		if !q.Required && strings.TrimSpace(*q.Answer) == "" {
			return true
		}
		for _, c := range q.Choices {
			if c.ID == *q.Answer && !c.Disabled {
				return true
			}
		}
		return false
	}
	index := state.Index
	last := state.Index == len(questions)-1
	canNext := valid(q)
	if last {
		for _, q := range questions {
			canNext = canNext && valid(q)
		}
	}
	label := "Next"
	if last {
		label = "Submit"
	}
	next := func() {
		if state.Submitted || state.Index != index || !canNext {
			return
		}
		if !last {
			state.Index++
			return
		}
		if submit != nil {
			state.Submitted = true
			submit()
		}
	}
	return Card(id, Heading(id+"/title", q.Title, 4), Text(id+"/description", q.Description).Role("note"), RadioGroup(id+"/answers", q.Title, *q.Answer, q.Choices, func(v string) { *q.Answer = v }).Disabled(state.Submitted), Row(id+"/navigation", Button(id+"/previous", "Previous", func() {
		if !state.Submitted && state.Index == index && state.Index > 0 {
			state.Index--
		}
	}).Variant(Outline).Disabled(state.Index == 0 || state.Submitted), Text(id+"/position", fmt.Sprintf("%d / %d", state.Index+1, len(questions))).Grow(1), Button(id+"/next", label, next).Disabled(!canNext || state.Submitted || (last && submit == nil))))
}
