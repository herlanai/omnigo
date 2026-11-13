// Package dshgo embeds the omnix agent runtime for desktop applications.
package dshgo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/herlanai/omnix/pkg/agent"
	"github.com/herlanai/omnix/pkg/brand"
	"github.com/herlanai/omnix/pkg/imagegen/provider_ark"
	"github.com/herlanai/omnix/pkg/llm"
	"github.com/herlanai/omnix/pkg/session"
	"github.com/herlanai/omnix/pkg/sysprompt"
	"github.com/herlanai/omnix/pkg/tools"
	"github.com/herlanai/omnix/pkg/waterfall"
)

var (
	ErrBusy   = errors.New("dshgo: turn already running")
	ErrClosed = errors.New("dshgo: conversation closed")
	ErrLimit  = errors.New("dshgo: conversation budget exceeded")
)

// Tool registers explicit application authority. No built-in shell or file tools
// are enabled. Schema validation and execution belong to the omnix pipeline.
type Tool struct {
	Name, Description string
	Parameters        map[string]any
	Execute           tools.ToolExecuteFunc
	RequiresApproval  bool
}

type Options struct {
	// Images opts into omnix image generation; nil leaves it disabled.
	Images *ImageOptions
	// DeepSeek is used when Adapter is nil.
	DeepSeek                                                         DeepSeekOptions
	Adapter                                                          llm.LLMAdapter
	Model, System                                                    string
	Tools                                                            []Tool
	Approve                                                          func(context.Context, llm.ToolCall) (bool, error)
	MaxSteps, MaxToolCalls, MaxTokens, MaxTurnBytes, MaxHistoryBytes int
	Timeout                                                          time.Duration
}

type Event struct {
	Kind, Text, Tool, CallID string
	Images                   []GeneratedImage
	IsError                  bool
}

type Result struct {
	Images []GeneratedImage
	Text   string
	Usage  llm.Usage
	Steps  int
	Events []session.SessionEvent
}

// Conversation owns successful session history. Run is blocking and belongs on
// a worker. Callbacks run on the omnix worker, never the UI goroutine. Adapters,
// tools and approval callbacks must honor context cancellation.
type Conversation struct {
	run      sync.Mutex
	life     sync.Mutex
	closed   bool
	cancel   context.CancelFunc
	options  Options
	pipeline *tools.Pipeline
	history  []session.SessionEvent
}

// New constructs a harness without starting workers or making network requests.
func New(o Options) (*Conversation, error) {
	if o.Adapter == nil {
		var err error
		o.Adapter, err = NewDeepSeek(o.DeepSeek)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(o.Model) == "" {
		o.Model = "deepseek-chat"
	}
	o.DeepSeek = DeepSeekOptions{}
	if o.MaxSteps == 0 {
		o.MaxSteps = 8
	}
	if o.MaxToolCalls == 0 {
		o.MaxToolCalls = 16
	}
	if o.MaxTokens == 0 {
		o.MaxTokens = 2048
	}
	if o.MaxTurnBytes == 0 {
		o.MaxTurnBytes = 256 << 10
	}
	if o.MaxHistoryBytes == 0 {
		o.MaxHistoryBytes = 4 << 20
	}
	if o.Timeout == 0 {
		o.Timeout = 2 * time.Minute
		if o.Images != nil {
			o.Timeout = 5 * time.Minute
		}
	}
	if o.MaxSteps < 1 || o.MaxToolCalls < 1 || o.MaxTokens < 1 || o.MaxTurnBytes < 1 || o.MaxHistoryBytes < o.MaxTurnBytes || o.Timeout < 0 {
		return nil, errors.New("dshgo: invalid limits")
	}
	p := tools.NewPipeline()
	protected := map[string]bool{}
	if o.Images != nil {
		tool, err := newImageTool(*o.Images)
		if err != nil {
			return nil, err
		}
		p.WithTool(tool)
		protected[tool.Name] = true
		o.System += "\n" + imageInstructions
		o.Images = nil
	}
	for _, definition := range o.Tools {
		if definition.Name == "" || definition.Execute == nil || p.Lookup(definition.Name) != nil {
			return nil, errors.New("dshgo: invalid or duplicate tool")
		}
		parameters := definition.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		raw, err := json.Marshal(parameters)
		if err != nil {
			return nil, err
		}
		schema, err := tools.Compile(raw)
		if err != nil {
			return nil, fmt.Errorf("dshgo: %s schema: %w", definition.Name, err)
		}
		p.WithTool(&tools.Tool{Name: definition.Name, Description: definition.Description, Schema: schema, Execute: definition.Execute})
		protected[definition.Name] = definition.RequiresApproval
	}
	approve := o.Approve
	p.UsePre(func(ec *tools.ExecContext, next waterfall.NextFunc) error {
		if tool := p.Lookup(ec.Request.Tool); tool != nil && tool.Schema != nil {
			if err := tool.Schema.Validate(ec.Request.Input); err != nil {
				return err
			}
		}
		if protected[ec.Request.Tool] {
			if approve == nil {
				return errors.New("tool approval required")
			}
			input, err := copyJSON(ec.Request.Input)
			if err != nil {
				return err
			}
			allowed, err := approve(ec.Ctx, llm.ToolCall{ID: ec.Request.CallID.String(), Name: ec.Request.Tool, Input: input})
			if err != nil {
				return err
			}
			if !allowed {
				return errors.New("tool approval denied")
			}
		}
		if err := ec.Ctx.Err(); err != nil {
			return err
		}
		return next()
	})
	o.Tools = nil
	return &Conversation{options: o, pipeline: p}, nil
}

func (c *Conversation) Cancel() {
	c.life.Lock()
	defer c.life.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

// Close cancels work without blocking the UI. Run unwinds and disposes its agent.
func (c *Conversation) Close() error {
	c.life.Lock()
	defer c.life.Unlock()
	c.closed = true
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
func (c *Conversation) Reset() error {
	if !c.run.TryLock() {
		return ErrBusy
	}
	defer c.run.Unlock()
	c.history = nil
	return nil
}

// History returns an independent omnix event log of successfully completed turns.
// Failed turns remain available in Result.Events, but are not committed here.
func (c *Conversation) History() []session.SessionEvent {
	c.run.Lock()
	defer c.run.Unlock()
	out, _ := copyJSON(c.history)
	return out
}

func copyJSON[T any](value T) (T, error) {
	var result T
	b, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(b, &result)
	}
	return result, err
}
func jsonSize(value any) (int, error) { b, err := json.Marshal(value); return len(b), err }

// Run delegates the entire model/tool loop and history projection to omnix.
// Callbacks may Cancel or Close; they must not call History (which waits for Run).
func (c *Conversation) Run(parent context.Context, input string, emit func(Event)) (out Result, err error) {
	if !c.run.TryLock() {
		return out, ErrBusy
	}
	defer c.run.Unlock()
	c.life.Lock()
	if c.closed {
		c.life.Unlock()
		return out, ErrClosed
	}
	ctx, cancel := context.WithTimeout(parent, c.options.Timeout)
	c.cancel = cancel
	c.life.Unlock()
	defer func() { cancel(); c.life.Lock(); c.cancel = nil; c.life.Unlock() }()
	if strings.TrimSpace(input) == "" {
		return out, errors.New("dshgo: input required")
	}
	if len(input) > c.options.MaxTurnBytes {
		return out, ErrLimit
	}
	if emit == nil {
		emit = func(Event) {}
	}
	log := session.NewSessionLog(brand.NewSessionID("omnigo"))
	for _, event := range c.history {
		if _, err = log.Append(event.Data); err != nil {
			return out, err
		}
	}
	start := log.Len()
	guard := &turnAdapter{source: c.options.Adapter, options: c.options, emit: emit, result: &out, used: len(input), seen: map[string]bool{}}
	prompt := sysprompt.New()
	prompt.Register("application", 0, c.options.System)
	// The pipeline is immutable except for per-turn observation middleware. Build
	// a fresh pipeline around the registered tools so callbacks cannot outlive Run.
	pipeline := tools.NewPipeline()
	for _, tool := range c.pipeline.Tools() {
		pipeline.WithTool(tool)
	}
	pipeline.UseExecute(func(ec *tools.ExecContext, next waterfall.NextFunc) error {
		emit(Event{Kind: "tool_start", Tool: ec.Request.Tool, CallID: ec.Request.CallID.String()})
		ec.Result = c.pipeline.Run(ec.Ctx, ec.Request, c.pipeline.Lookup(ec.Request.Tool))
		n, e := jsonSize(ec.Result)
		if e != nil || n > c.options.MaxTurnBytes-guard.used {
			guard.failure = ErrLimit
			ec.Result = &tools.ToolCallResult{CallID: ec.Request.CallID, IsError: true, Error: ErrLimit.Error()}
			cancel()
		} else {
			guard.used += n
		}
		if ec.Request.Tool == provider_ark.ToolName && !ec.Result.IsError {
			if response, ok := ec.Result.Value.(*provider_ark.Response); ok {
				var images []GeneratedImage
				for _, value := range response.Data {
					if value.Error == nil && value.URL != "" {
						images = append(images, GeneratedImage{URL: value.URL, Size: value.Size})
					}
				}
				out.Images = append(out.Images, images...)
				if len(images) > 0 {
					emit(Event{Kind: "images", Tool: ec.Request.Tool, CallID: ec.Request.CallID.String(), Images: images})
				}
			}
		}
		emit(Event{Kind: "tool_end", Tool: ec.Request.Tool, CallID: ec.Request.CallID.String(), IsError: ec.Result.IsError})
		// Execution already took place in the owned omnix pipeline.
		return nil
	})
	runtime := agent.NewAgent(log.SessionID(), log, prompt, pipeline, guard, agent.AgentOptions{Model: c.options.Model, MaxTokens: c.options.MaxTokens})
	runtime.SetRunContext(ctx, 0)
	defer runtime.Dispose()
	if err = runtime.Run(input); err != nil {
		return out, err
	}
	// Wait for callbacks and tools before reading turn-local state, even on cancel.
	_ = runtime.WhenIdle(context.Background())
	out.Events, _ = copyJSON(log.Events()[start:])
	if guard.failure != nil {
		return out, guard.failure
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	for _, event := range out.Events {
		if failure, ok := event.Data.(session.AgentErrorData); ok {
			return out, errors.New(failure.Message)
		}
	}
	n, e := jsonSize(log.Events())
	if e != nil {
		return out, e
	}
	if n > c.options.MaxHistoryBytes {
		return out, ErrLimit
	}
	c.history, err = copyJSON(log.Events())
	return out, err
}

// turnAdapter observes requests and enforces budgets; agent.Agent owns stepping,
// tool continuation, model history assembly, and session event recording.
type turnAdapter struct {
	source      llm.LLMAdapter
	options     Options
	emit        func(Event)
	result      *Result
	used, calls int
	seen        map[string]bool
	failure     error
}

func (t *turnAdapter) Name() string { return t.source.Name() }
func (t *turnAdapter) Chat(ctx context.Context, r llm.ChatRequest, cb func(llm.StreamChunk)) (llm.Usage, error) {
	if t.result.Steps >= t.options.MaxSteps {
		t.failure = ErrLimit
		return llm.Usage{}, t.failure
	}
	n, err := jsonSize(r)
	if err != nil {
		return llm.Usage{}, err
	}
	if n > t.options.MaxHistoryBytes {
		t.failure = ErrLimit
		return llm.Usage{}, t.failure
	}
	t.result.Steps++
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	meaningful := false
	usage, err := t.source.Chat(streamCtx, r, func(chunk llm.StreamChunk) {
		if t.failure != nil || streamCtx.Err() != nil {
			return
		}
		n, e := jsonSize(chunk)
		if e != nil {
			t.failure = e
			cancel()
			return
		}
		t.used += n
		if t.used > t.options.MaxTurnBytes {
			t.failure = ErrLimit
			cancel()
			return
		}
		if chunk.Kind == llm.ChunkToolCall {
			call := chunk.ToolCall
			if call == nil || call.ID == "" || call.Name == "" || t.seen[call.ID] {
				t.failure = errors.New("dshgo: invalid tool call")
				cancel()
				return
			}
			t.calls++
			if t.calls > t.options.MaxToolCalls {
				t.failure = ErrLimit
				cancel()
				return
			}
			t.seen[call.ID] = true
			copied, e := copyJSON(*call)
			if e != nil {
				t.failure = e
				cancel()
				return
			}
			chunk.ToolCall = &copied
			meaningful = true
		}
		if chunk.Kind == llm.ChunkText && chunk.Text != "" {
			meaningful = true
			t.result.Text += chunk.Text
			t.emit(Event{Kind: "text", Text: chunk.Text})
		}
		if chunk.Kind == llm.ChunkReasoning {
			t.emit(Event{Kind: "reasoning", Text: chunk.Reasoning})
		}
		cb(chunk)
	})
	t.result.Usage.PromptTokens += usage.PromptTokens
	t.result.Usage.CompletionTokens += usage.CompletionTokens
	t.result.Usage.PromptCacheHitTokens += usage.PromptCacheHitTokens
	t.result.Usage.PromptCacheMissTokens += usage.PromptCacheMissTokens
	if t.failure == nil && err != nil {
		t.failure = err
	}
	if t.failure == nil && !meaningful {
		t.failure = errors.New("dshgo: model returned no answer or tool call")
	}
	if t.failure != nil {
		return usage, t.failure
	}
	return usage, ctx.Err()
}
