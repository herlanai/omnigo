package dshgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnix/pkg/llm"
)

// PanelOptions connects the optional Go control to an application's UI host.
type PanelOptions struct {
	Agent Options
	// RunTask must be safe from workers and schedule its returned callback on the
	// UI goroutine with a repaint. Pass omnigo.WindowEnvironment.RunTask.
	// Without a dispatcher the panel is a disabled, renderable preview.
	RunTask func(func() func())
	Copy    func(string)
	// OpenImage opens a provider-returned URL on explicit user interaction.
	OpenImage     func(string)
	Title, Notice string
	MaxMessages   int
}

type panelMessage struct {
	id         int
	role, text string
	images     []GeneratedImage
	feedback   ui.MessageFeedback
}
type panelApproval struct {
	name, arguments string
	reply           chan bool
}

// Panel owns its harness, draft, messages, scrolling and approval interaction.
// Construct once per window. All public methods, including Close, belong to the
// UI goroutine. Worker updates are coalesced through RunTask; no polling is used.
type Panel struct {
	conversation                     *Conversation
	options                          PanelOptions
	draft                            ui.PromptInputState
	scroll, approvalScroll           ui.ScrollState
	messages                         []*panelMessage
	approval                         *panelApproval
	status                           string
	busy, closed, disabled, readOnly bool
	generation, nextID               int
	cancel                           context.CancelFunc
	header                           func() *ui.Element
	pendingMu                        sync.Mutex
	pending                          []func()
	scheduled                        bool
}

func NewPanel(o PanelOptions) (*Panel, error) {
	if o.MaxMessages == 0 {
		o.MaxMessages = 40
	}
	if o.MaxMessages < 2 {
		return nil, errors.New("dshgo: panel needs at least two message slots")
	}
	if o.Title == "" {
		o.Title = "Assistant"
	}
	p := &Panel{options: o}
	if err := p.Configure(o.Agent); err != nil {
		return nil, err
	}
	// Avoid retaining credentials and mutable tool definitions in the view state.
	p.options.Agent = Options{}
	return p, nil
}

// Configure validates a replacement before discarding the idle conversation.
func (p *Panel) Configure(o Options) error {
	if p.closed {
		return ErrClosed
	}
	if p.busy {
		return ErrBusy
	}
	generation := p.generation + 1
	if o.Approve == nil {
		o.Approve = func(ctx context.Context, call llm.ToolCall) (bool, error) {
			raw, err := json.MarshalIndent(call.Input, "", "  ")
			if err != nil {
				return false, err
			}
			request := &panelApproval{name: call.Name, arguments: string(raw), reply: make(chan bool, 1)}
			p.enqueue(func() {
				if p.generation == generation && !p.closed && p.busy {
					p.approval = request
				}
			})
			defer p.enqueue(func() {
				if p.approval == request {
					p.approval = nil
				}
			})
			select {
			case allowed := <-request.reply:
				return allowed, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}
	}
	conversation, err := New(o)
	if err != nil {
		return err
	}
	if p.conversation != nil {
		_ = p.conversation.Close()
	}
	p.conversation = conversation
	p.generation = generation
	p.messages = nil
	p.approval = nil
	p.status = ""
	p.scroll = ui.ScrollState{}
	model := o.Model
	if model == "" {
		model = "deepseek-chat"
	}
	p.draft = ui.PromptInputState{Text: ui.NewTextBuffer(""), Model: ui.ModelSelectorState{Model: model}}
	return nil
}

// enqueue coalesces worker notifications into one pending UI dispatch. Only the
// queue is shared; closures touch panel state after arriving on the UI goroutine.
func (p *Panel) enqueue(apply func()) {
	if p.options.RunTask == nil {
		return
	}
	p.pendingMu.Lock()
	p.pending = append(p.pending, apply)
	if p.scheduled {
		p.pendingMu.Unlock()
		return
	}
	p.scheduled = true
	p.pendingMu.Unlock()
	p.options.RunTask(func() func() {
		return func() {
			p.pendingMu.Lock()
			batch := p.pending
			p.pending = nil
			p.scheduled = false
			p.pendingMu.Unlock()
			if p.closed {
				return
			}
			for _, update := range batch {
				update()
			}
		}
	})
}

func (p *Panel) Busy() bool                                { return p.busy }
func (p *Panel) SetNotice(text string)                     { p.options.Notice = text }
func (p *Panel) SetHeaderActions(build func() *ui.Element) { p.header = build }
func (p *Panel) SetDisabled(disabled bool) {
	p.disabled = disabled
	if disabled {
		p.Stop()
	}
}
func (p *Panel) SetReadOnly(readOnly bool) {
	p.readOnly = readOnly
	if readOnly {
		p.Stop()
	}
}
func (p *Panel) SetDraft(text string) {
	if !p.busy && !p.closed && !p.disabled && !p.readOnly {
		p.draft.Text.Set(text)
	}
}
func (p *Panel) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.conversation.Cancel()
	p.approval = nil
}
func (p *Panel) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	p.generation++
	p.Stop()
	p.busy = false
	p.pendingMu.Lock()
	p.pending = nil
	p.pendingMu.Unlock()
	return p.conversation.Close()
}
func (p *Panel) Reset() error {
	if p.closed {
		return ErrClosed
	}
	if p.busy {
		return ErrBusy
	}
	if err := p.conversation.Reset(); err != nil {
		return err
	}
	p.messages = nil
	p.approval = nil
	p.status = ""
	p.scroll = ui.ScrollState{}
	p.draft.Text.Set("")
	return nil
}

// Send starts an asynchronous turn. The panel handles all streaming updates.
func (p *Panel) Send(text string) error {
	if p.closed {
		return ErrClosed
	}
	if p.busy {
		return ErrBusy
	}
	if p.disabled || p.readOnly || p.options.RunTask == nil {
		return errors.New("dshgo: panel input unavailable")
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("dshgo: input required")
	}
	p.busy = true
	p.status = "Responding…"
	p.draft.Text.Set("")
	p.nextID++
	user := &panelMessage{id: p.nextID, role: "user", text: text}
	p.nextID++
	answer := &panelMessage{id: p.nextID, role: "assistant"}
	p.messages = append(p.messages, user, answer)
	if len(p.messages) > p.options.MaxMessages {
		p.messages = p.messages[len(p.messages)-p.options.MaxMessages:]
	}
	generation := p.generation
	conversation := p.conversation
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go func() {
		defer cancel()
		result, err := conversation.Run(ctx, text, func(event Event) {
			p.enqueue(func() {
				if p.generation != generation {
					return
				}
				switch event.Kind {
				case "text":
					answer.text += event.Text
				case "images":
					answer.images = append(answer.images, event.Images...)
				case "tool_start":
					p.status = "Running tool: " + event.Tool
				case "tool_end":
					p.status = "Tool completed: " + event.Tool
					if event.IsError {
						p.status = "Tool failed or denied: " + event.Tool
					}
				}
			})
		})
		p.enqueue(func() {
			if p.generation != generation {
				return
			}
			p.busy = false
			p.cancel = nil
			p.approval = nil
			p.status = fmt.Sprintf("%d steps · %d input / %d output tokens", result.Steps, result.Usage.PromptTokens, result.Usage.CompletionTokens)
			if err != nil {
				p.status = err.Error()
				if errors.Is(err, context.Canceled) {
					p.status = "Stopped"
				}
			}
		})
	}()
	return nil
}

func (p *Panel) decide(request *panelApproval, allow bool) {
	if p.closed || p.disabled || p.readOnly || p.approval != request {
		return
	}
	select {
	case request.reply <- allow:
	default:
	}
	p.approval = nil
}

// View returns an ordinary Go element tree. Give it the available panel bounds;
// the message viewport grows while the composer and approval remain visible.
func (p *Panel) View(id string) *ui.Element {
	locked := p.closed || p.disabled || p.readOnly
	heading := ui.Row(id+"/header", ui.Heading(id+"/title", p.options.Title, 2).Grow(1))
	if p.header != nil {
		if extra := p.header(); extra != nil {
			heading = ui.Row(id+"/header", ui.Heading(id+"/title", p.options.Title, 2).Grow(1), extra)
		}
	}
	rows := []*ui.Element{}
	for _, message := range p.messages {
		message := message
		actions := ui.AIMessageOptions{Author: message.role, Streaming: p.busy && message.role == "assistant" && message == p.messages[len(p.messages)-1], OnCopy: p.options.Copy}
		if message.role == "assistant" {
			actions.Feedback = &message.feedback
		} else {
			actions.OnEdit = func(text string) { p.SetDraft(text) }
		}
		rows = append(rows, ui.AIMessage(fmt.Sprintf("%s/message/%d", id, message.id), message.role, message.text, actions).Disabled(locked || p.busy && message.role == "user"))
		for i, generated := range message.images {
			imageID := fmt.Sprintf("%s/message/%d/image/%d", id, message.id, i)
			rows = append(rows, ui.Card(imageID,
				ui.Text(imageID+"/title", "Generated image · "+generated.Size),
				ui.Text(imageID+"/url", generated.URL).Wrap(true),
				ui.Row(imageID+"/actions",
					ui.Button(imageID+"/open", "Open image", func() {
						if p.options.OpenImage != nil {
							p.options.OpenImage(generated.URL)
						}
					}).Disabled(locked || p.options.OpenImage == nil),
					ui.Button(imageID+"/copy", "Copy image URL", func() {
						if p.options.Copy != nil {
							p.options.Copy(generated.URL)
						}
					}).Disabled(locked || p.options.Copy == nil),
				).Gap(8),
			))
		}
	}
	if len(rows) == 0 {
		rows = append(rows, ui.Text(id+"/empty", "Start a conversation. Tools run only when explicitly registered.").Wrap(true).Muted(true))
	}
	children := []*ui.Element{heading}
	if p.options.Notice != "" {
		children = append(children, ui.Text(id+"/notice", p.options.Notice).Wrap(true).Muted(true))
	}
	children = append(children, ui.MessageScroller(id+"/chat/messages", &p.scroll, rows...).Height(1).Grow(1))
	_, maxY := p.scroll.Limits()
	children = append(children, ui.Button(id+"/latest", "Jump to latest", func() { _, end := p.scroll.Limits(); p.scroll.Y = end }).Variant(ui.Ghost).Hidden(p.scroll.Y >= maxY-1))
	if request := p.approval; request != nil {
		children = append(children, ui.Card(id+"/approval", ui.Text(id+"/approval/title", "Allow tool: "+request.name+"?").FontWeight(600), ui.ScrollArea(id+"/approval/scroll", &p.approvalScroll, ui.Text(id+"/approval/arguments", request.arguments).Wrap(true)).Height(96), ui.Text(id+"/approval/scope", "This call only"), ui.Row(id+"/approval/actions", ui.Button(id+"/approve", "Allow", func() { p.decide(request, true) }), ui.Button(id+"/deny", "Deny", func() { p.decide(request, false) }).Variant(ui.Outline))).Padding(12).Gap(8).Disabled(locked))
	}
	status := ui.PromptReady
	if p.busy {
		status = ui.PromptStreaming
	}
	children = append(children, ui.Text(id+"/status", p.status).Role("status").Wrap(true), ui.PromptInput(id+"/prompt", &p.draft, ui.PromptInputOptions{Placeholder: "Ask a question… Enter to send, Shift+Enter for a new line", Disabled: locked || p.options.RunTask == nil, Status: status, Models: []ui.ModelOption{{ID: p.draft.Model.Model, Name: p.draft.Model.Model, Provider: "Configured model"}}, OnSubmit: func(s ui.PromptSubmission) {
		if err := p.Send(s.Text); err != nil {
			p.status = err.Error()
		}
	}, OnStop: p.Stop}), ui.Button(id+"/clear", "Clear conversation", func() {
		if err := p.Reset(); err != nil {
			p.status = err.Error()
		}
	}).Variant(ui.Ghost).Disabled(locked || p.busy))
	return ui.Column(id, children...).Gap(10).Padding(16).Role("region").Label(p.options.Title)
}
