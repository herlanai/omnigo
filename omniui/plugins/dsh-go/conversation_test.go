package dshgo

import (
	"context"
	"errors"
	"fmt"
	"github.com/herlanai/omnix/pkg/llm"
	"github.com/herlanai/omnix/pkg/session"
	"testing"
)

type agentModel func(context.Context, llm.ChatRequest, func(llm.StreamChunk)) (llm.Usage, error)

func (agentModel) Name() string { return "test" }
func (f agentModel) Chat(c context.Context, r llm.ChatRequest, cb func(llm.StreamChunk)) (llm.Usage, error) {
	return f(c, r, cb)
}
func newHarness(t *testing.T, o Options) *Conversation {
	t.Helper()
	o.Model = "test-model"
	c, e := New(o)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestAgentExecutesToolAndContinues(t *testing.T) {
	requests, executions := 0, 0
	c := newHarness(t, Options{System: "test system", Tools: []Tool{{Name: "lookup", Execute: func(context.Context, map[string]any) (any, error) { executions++; return "found", nil }}}, Adapter: agentModel(func(ctx context.Context, r llm.ChatRequest, cb func(llm.StreamChunk)) (llm.Usage, error) {
		requests++
		if r.System != "test system" || len(r.Tools) != 1 {
			t.Errorf("request %+v", r)
		}
		if requests == 1 {
			cb(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "lookup-1", Name: "lookup", Input: map[string]any{}}})
		} else {
			if requests == 2 {
				last := r.Messages[len(r.Messages)-1]
				if len(last.Content) == 0 || last.Content[0].ToolResult == nil || last.Content[0].ToolResult.Content != "found" {
					t.Errorf("tool result missing: %+v", last)
				}
			}
			cb(llm.StreamChunk{Kind: llm.ChunkText, Text: "answer"})
		}
		return llm.Usage{PromptTokens: 4}, nil
	})})
	var events []Event
	result, err := c.Run(context.Background(), "lookup", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "answer" || result.Steps != 2 || executions != 1 || result.Usage.PromptTokens != 8 {
		t.Fatalf("result %+v executions %d", result, executions)
	}
	if len(events) != 3 || events[0].Kind != "tool_start" || events[1].Kind != "tool_end" {
		t.Fatalf("events %+v", events)
	}
	if problems := session.VerifyInvariants(result.Events); len(problems) > 0 {
		t.Fatal(problems)
	}
	history := c.History()
	history[0].Data = session.AgentErrorData{Message: "mutation"}
	if _, ok := c.History()[0].Data.(session.AgentErrorData); ok {
		t.Fatal("history aliased")
	}
	if _, err = c.Run(context.Background(), "follow up", nil); err != nil {
		t.Fatal(err)
	}
	if err = c.Reset(); err != nil || len(c.History()) != 0 {
		t.Fatal("reset failed", err)
	}
}

func TestAgentToolApprovalAndSchema(t *testing.T) {
	for _, mode := range []string{"denied", "approved", "schema", "panic", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			steps, executions, approvals := 0, 0, 0
			c := newHarness(t, Options{Approve: func(context.Context, llm.ToolCall) (bool, error) { approvals++; return mode != "denied", nil }, Tools: []Tool{{Name: "action", RequiresApproval: true, Parameters: map[string]any{"type": "object", "properties": map[string]any{"n": map[string]any{"type": "number"}}, "required": []string{"n"}}, Execute: func(context.Context, map[string]any) (any, error) {
				executions++
				if mode == "panic" {
					panic("test")
				}
				return "ok", nil
			}}}, Adapter: agentModel(func(_ context.Context, r llm.ChatRequest, cb func(llm.StreamChunk)) (llm.Usage, error) {
				steps++
				if steps == 1 {
					input := map[string]any{"n": float64(1)}
					name := "action"
					if mode == "schema" {
						input["n"] = "wrong"
					}
					if mode == "unknown" {
						name = "missing"
					}
					cb(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "id", Name: name, Input: input}})
				} else {
					last := r.Messages[len(r.Messages)-1].Content[0].ToolResult
					if last == nil || last.IsError != (mode != "approved") {
						t.Errorf("result %+v", last)
					}
					cb(llm.StreamChunk{Kind: llm.ChunkText, Text: "done"})
				}
				return llm.Usage{}, nil
			})})
			if _, err := c.Run(context.Background(), "act", nil); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "approved" || mode == "panic" {
				want = 1
			}
			if executions != want {
				t.Fatalf("executions %d", executions)
			}
			if mode == "schema" && approvals != 0 {
				t.Fatal("approved invalid schema")
			}
		})
	}
}

func TestAgentCancellationAndLimits(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan error, 1)
	c := newHarness(t, Options{Adapter: agentModel(func(ctx context.Context, _ llm.ChatRequest, _ func(llm.StreamChunk)) (llm.Usage, error) {
		close(started)
		<-ctx.Done()
		return llm.Usage{}, ctx.Err()
	})})
	go func() { _, err := c.Run(context.Background(), "wait", nil); finished <- err }()
	<-started
	if _, err := c.Run(context.Background(), "busy", nil); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	c.Close()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(c.History()) != 0 {
		t.Fatal("failed turn committed")
	}
	if _, err := c.Run(context.Background(), "closed", nil); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	steps := 0
	loop := newHarness(t, Options{MaxSteps: 2, Adapter: agentModel(func(_ context.Context, _ llm.ChatRequest, cb func(llm.StreamChunk)) (llm.Usage, error) {
		steps++
		cb(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: fmt.Sprint(steps), Name: "missing", Input: map[string]any{}}})
		return llm.Usage{}, nil
	})})
	if _, err := loop.Run(context.Background(), "loop", nil); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if steps != 2 || len(loop.History()) != 0 {
		t.Fatal("limit not enforced")
	}
}
