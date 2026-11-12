package baseui

import (
	"strconv"
	"strings"
)

type ModelEffort struct {
	ID, Name string
	Disabled bool
}
type ModelOption struct {
	ID, Name, Provider, Description string
	Disabled                        bool
	Efforts                         []ModelEffort
}
type ModelSelectorState struct {
	Model, Effort string
	model, effort SelectState
}

func modelOptions(models []ModelOption) []Option {
	options := make([]Option, len(models))
	for i, m := range models {
		label := m.Name
		if label == "" {
			label = m.ID
		}
		options[i] = Option{ID: m.ID, Label: label, Disabled: m.Disabled}
		efforts := []Option{}
		for _, e := range m.Efforts {
			name := e.Name
			if name == "" {
				name = e.ID
			}
			efforts = append(efforts, Option{ID: e.ID, Label: name, Disabled: e.Disabled})
		}
		validateOptions(efforts)
	}
	validateOptions(options)
	return options
}
func modelByID(models []ModelOption, id string) (ModelOption, bool) {
	for _, m := range models {
		if m.ID == id && !m.Disabled {
			return m, true
		}
	}
	return ModelOption{}, false
}
func supportedEffort(model ModelOption, effort string) string {
	for _, e := range model.Efforts {
		if e.ID == effort && !e.Disabled {
			return effort
		}
	}
	for _, e := range model.Efforts {
		if !e.Disabled {
			return e.ID
		}
	}
	return ""
}

type ModelSelectorOptions struct {
	Placeholder string
	Disabled    bool
}

func ModelSelector(id string, state *ModelSelectorState, models []ModelOption, change func(string, string), optionsOverride ...ModelSelectorOptions) *Element {
	config := ModelSelectorOptions{Placeholder: "Select model"}
	if len(optionsOverride) > 0 {
		config = optionsOverride[0]
		if config.Placeholder == "" {
			config.Placeholder = "Select model"
		}
	}
	if state == nil {
		panic("baseui: nil model selector state")
	}
	models = append([]ModelOption(nil), models...)
	for i := range models {
		models[i].Efforts = append([]ModelEffort(nil), models[i].Efforts...)
	}
	// Providers retain first-occurrence order, even with interleaved input.
	providers := []string{}
	seenProviders := map[string]bool{}
	for _, m := range models {
		if !seenProviders[m.Provider] {
			providers = append(providers, m.Provider)
			seenProviders[m.Provider] = true
		}
	}
	ordered := make([]ModelOption, 0, len(models))
	for _, provider := range providers {
		for _, m := range models {
			if m.Provider == provider {
				ordered = append(ordered, m)
			}
		}
	}
	models = ordered
	options := modelOptions(models)
	state.model.Value = state.Model
	pick := Select(id+"/model", "Model", &state.model, options, func(v string) {
		m, ok := modelByID(models, v)
		if !ok || state.Model == v {
			return
		}
		state.effort.Open = false
		state.Model = v
		state.Effort = supportedEffort(m, state.Effort)
		if change != nil {
			change(state.Model, state.Effort)
		}
	})
	known := false
	for _, m := range models {
		if m.ID == state.Model {
			known = true
			break
		}
	}
	if !known {
		pick.children[0].text = config.Placeholder
		pick.children[0].label = "Model: " + config.Placeholder
	}
	list := pick.children[1].children[0]
	rows := make([]*Element, 0, len(models)+len(providers))
	provider := ""
	contentHeight := 0.
	if len(models) > 0 {
		for i, m := range models {
			if m.Provider != "" && (i == 0 || m.Provider != provider) {
				rows = append(rows, Text(id+"/model/group/"+strconv.Itoa(i), m.Provider).Role("note").Height(28))
				contentHeight += 30
			}
			provider = m.Provider
			row := list.children[i]
			row.description = m.Description
			row.detail = m.Description
			if m.Description != "" {
				row.Height(56)
				contentHeight += 58
			} else {
				contentHeight += 38
			}
			rows = append(rows, row)
		}
		list.children = rows
		list.Height(min(320, contentHeight))
		pick.children[1].Width(320)
	} else {
		pick.children[0].Disabled(true)
	}
	children := []*Element{pick}
	var current *ModelOption
	for i := range models {
		if models[i].ID == state.Model {
			current = &models[i]
			break
		}
	}
	if current != nil && len(current.Efforts) > 0 {
		m := *current
		efforts := []Option{}
		for _, e := range m.Efforts {
			name := e.Name
			if name == "" {
				name = e.ID
			}
			efforts = append(efforts, Option{ID: e.ID, Label: name, Disabled: e.Disabled})
		}
		state.effort.Value = state.Effort
		children = append(children, Select(id+"/effort", "Reasoning effort", &state.effort, efforts, func(v string) {
			if state.Effort == v {
				return
			}
			state.Effort = v
			if change != nil {
				change(state.Model, v)
			}
		}).Disabled(m.Disabled))
	}
	return Row(id, children...).Role("group").Label("Model and effort").Disabled(config.Disabled)
}

type PromptAttachment struct{ ID, Name, Path, Description string }
type PromptSubmission struct {
	Text, Model, Effort string
	Attachments         []PromptAttachment
}
type PromptStatus string

const (
	PromptReady     PromptStatus = ""
	PromptSubmitted PromptStatus = "submitted"
	PromptStreaming PromptStatus = "streaming"
)

type PromptInputState struct {
	Text        *TextBuffer
	Model       ModelSelectorState
	Attachments []PromptAttachment
}
type PromptInputOptions struct {
	Placeholder, Error string
	Disabled           bool
	Status             PromptStatus
	Models             []ModelOption
	OnAttach           func()
	OnRemove           func(PromptAttachment)
	OnSubmit           func(PromptSubmission)
	OnStop             func()
	OnModelChange      func(string, string)
}

// PromptInput submits immutable metadata snapshots. Clearing text, uploading,
// running models and reporting failures remain explicit application actions.
func PromptInput(id string, state *PromptInputState, o PromptInputOptions) *Element {
	if state == nil || state.Text == nil {
		panic("baseui: nil prompt input state or text")
	}
	busy := o.Status == PromptSubmitted || o.Status == PromptStreaming
	canSend := func() bool {
		if o.Disabled || busy || o.OnSubmit == nil || state.Text.Composition() != "" {
			return false
		}
		if strings.TrimSpace(state.Text.String()) == "" && len(state.Attachments) == 0 {
			return false
		}
		if len(o.Models) > 0 {
			m, ok := modelByID(o.Models, state.Model.Model)
			if !ok {
				return false
			}
			if len(m.Efforts) > 0 && supportedEffort(m, state.Model.Effort) != state.Model.Effort {
				return false
			}
		}
		return true
	}
	submit := func() {
		if canSend() {
			o.OnSubmit(PromptSubmission{Text: state.Text.String(), Model: state.Model.Model, Effort: state.Model.Effort, Attachments: append([]PromptAttachment(nil), state.Attachments...)})
		}
	}
	children := []*Element{}
	seen := map[string]bool{}
	for _, a := range state.Attachments {
		a := a
		if a.ID == "" || seen[a.ID] {
			panic("baseui: duplicate or empty attachment ID")
		}
		seen[a.ID] = true
		children = append(children, Attachment(id+"/attachment/"+a.ID, a.Name, a.Description, AttachmentReady, func() {
			for i, current := range state.Attachments {
				if current.ID == a.ID {
					state.Attachments = append(state.Attachments[:i:i], state.Attachments[i+1:]...)
					if o.OnRemove != nil {
						o.OnRemove(current)
					}
					break
				}
			}
		}).Disabled(o.Disabled || busy))
	}
	editor := Textarea(id+"/text", "Prompt text", state.Text).Placeholder(o.Placeholder).Disabled(o.Disabled || busy).OnKey(func(ev KeyEvent) bool {
		if ev.Key != "Enter" || ev.Shift || ev.Command {
			return false
		}
		if state.Text.Composition() != "" {
			return true
		}
		if !ev.Repeat && !ev.Up {
			submit()
		}
		return true
	})
	children = append(children, editor)
	actions := []*Element{Button(id+"/attach", "Attach", o.OnAttach).Variant(Ghost).Disabled(o.Disabled || busy || o.OnAttach == nil), Column(id + "/spacer").Grow(1)}
	if len(o.Models) > 0 {
		actions = append(actions, ModelSelector(id+"/model", &state.Model, o.Models, o.OnModelChange).Disabled(o.Disabled || busy))
	}
	if busy {
		actions = append(actions, Button(id+"/stop", "Stop", o.OnStop).Variant(Outline).Disabled(o.Disabled || o.OnStop == nil))
	} else {
		actions = append(actions, Button(id+"/send", "Send", submit).Disabled(!canSend()))
	}
	children = append(children, Row(id+"/actions", actions...))
	if o.Error != "" {
		children = append(children, paragraph(id+"/error", o.Error).Role("alert"))
	}
	return Card(id, children...).Role("group").Label("Prompt input").Invalid(o.Error != "")
}
