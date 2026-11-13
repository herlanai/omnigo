package demos

import (
	"context"
	"fmt"
	"strings"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	dshgo "github.com/herlanai/omnigo/omniui/plugins/dsh-go"
	"github.com/herlanai/omnix/pkg/llm"
)

func (d *Demo) panelContent() *ui.Element {
	if d.plugins == nil {
		d.plugins = &pluginState{}
	}
	if d.plugins.panel == nil {
		panel, err := dshgo.NewPanel(dshgo.PanelOptions{
			Title: "OmniGo Assistant", Notice: "Offline model demo · real omnix agent/tool execution. Send any question to test approval; send 'wait' to test Stop.",
			RunTask: d.RunTask, Copy: d.copyText,
			Agent: dshgo.Options{Model: "studio-demo", Adapter: &panelDemoModel{}, Tools: []dshgo.Tool{{Name: "studio_info", Description: "Read Studio demo information", RequiresApproval: true, Execute: func(ctx context.Context, in map[string]any) (any, error) {
				return "OmniGo Studio — native Go chat panel with omnix agent execution", ctx.Err()
			}}}},
		})
		if err != nil {
			return ui.Text("assistant/error", err.Error())
		}
		d.plugins.panel = panel
	}
	return d.plugins.panel.View("assistant")
}

// A deterministic local model exercises the real harness without credentials.
// It performs no provider call and is explicitly labeled as a demo in the panel.
type panelDemoModel struct{ calls int }

func (*panelDemoModel) Name() string { return "studio-offline" }
func (m *panelDemoModel) Chat(ctx context.Context, r llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
	if err := ctx.Err(); err != nil {
		return llm.Usage{}, err
	}
	last := r.Messages[len(r.Messages)-1]
	for _, block := range last.Content {
		if block.ToolResult != nil {
			answer := "Tool returned: " + block.ToolResult.Content
			if block.ToolResult.IsError {
				answer = "The tool was denied or failed. No protected action was performed."
			}
			for _, part := range strings.Fields(answer) {
				if err := ctx.Err(); err != nil {
					return llm.Usage{}, err
				}
				emit(llm.StreamChunk{Kind: llm.ChunkText, Text: part + " "})
			}
			return llm.Usage{PromptTokens: 12, CompletionTokens: 8}, nil
		}
		if strings.TrimSpace(block.Text) == "wait" {
			emit(llm.StreamChunk{Kind: llm.ChunkText, Text: "Waiting for cancellation. Press Stop."})
			<-ctx.Done()
			return llm.Usage{}, ctx.Err()
		}
	}
	m.calls++
	emit(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: fmt.Sprintf("studio-%d", m.calls), Name: "studio_info", Input: map[string]any{}}})
	return llm.Usage{PromptTokens: 8, CompletionTokens: 2}, nil
}
