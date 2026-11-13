package dshgo

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	ui "github.com/herlanai/omnigo/omniui/baseui"
	"github.com/herlanai/omnix/pkg/llm"
)

type panelDriver struct{ queue chan func() }

func (d *panelDriver) run(work func() func()) { d.queue <- work() }
func (d *panelDriver) until(t *testing.T, ready func() bool) {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for !ready() {
		select {
		case f := <-d.queue:
			f()
		case <-timeout.C:
			t.Fatal("UI dispatch did not settle")
		}
	}
}
func panelSurface(t *testing.T, p *Panel, dark bool) *ui.Surface {
	t.Helper()
	s, err := ui.NewSurface(900, 780, ui.NewTheme(dark))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.Mount(p.View("panel")); err != nil {
		t.Fatal(err)
	}
	return s
}
func key(t *testing.T, s *ui.Surface, id, key string) {
	t.Helper()
	if !s.Focus(id) {
		t.Fatalf("cannot focus %s", id)
	}
	if err := s.Key(ui.KeyEvent{Key: key}); err != nil {
		t.Fatal(err)
	}
}

func TestPanelApprovalAndRendering(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(map[bool]string{false: "deny", true: "allow"}[allow], func(t *testing.T) {
			dispatcher := &panelDriver{queue: make(chan func(), 16)}
			steps, executions := 0, 0
			panel, err := NewPanel(PanelOptions{RunTask: dispatcher.run, Title: "Test Assistant", Agent: Options{Adapter: agentModel(func(ctx context.Context, r llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
				steps++
				if steps == 1 {
					emit(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "call", Name: "protected", Input: map[string]any{}}})
				} else {
					last := r.Messages[len(r.Messages)-1].Content[0].ToolResult
					if last == nil || last.IsError == allow {
						t.Error("wrong tool decision")
					}
					emit(llm.StreamChunk{Kind: llm.ChunkText, Text: "Finished."})
				}
				return llm.Usage{PromptTokens: 2, CompletionTokens: 1}, nil
			}), Tools: []Tool{{Name: "protected", RequiresApproval: true, Execute: func(context.Context, map[string]any) (any, error) { executions++; return "ok", nil }}}}})
			if err != nil {
				t.Fatal(err)
			}
			defer panel.Close()
			panel.SetDraft("Please run the tool")
			s := panelSurface(t, panel, false)
			key(t, s, "panel/prompt/text", "Enter")
			if !panel.Busy() || panel.draft.Text.String() != "" {
				t.Fatal("submit did not start")
			}
			dispatcher.until(t, func() bool { return panel.approval != nil })
			for _, dark := range []bool{false, true} {
				if err = s.SetTheme(ui.NewTheme(dark)); err != nil {
					t.Fatal(err)
				}
				if err = s.Mount(panel.View("panel")); err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"panel/approve", "panel/deny", "panel/prompt/stop", "panel/chat/messages"} {
					r, ok := s.Bounds(id)
					if !ok || r.W <= 0 || r.H <= 0 || r.Y < 0 || r.Y+r.H > 780 {
						t.Errorf("invalid bounds %s %+v", id, r)
					}
				}
				img, err := s.Render()
				if err != nil {
					t.Fatal(err)
				}
				if dir := os.Getenv("OMNIGO_PANEL_SHOTS"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					name := "light.png"
					if dark {
						name = "dark.png"
					}
					file, err := os.Create(filepath.Join(dir, name))
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(file, img)
					file.Close()
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			button := "panel/deny"
			if allow {
				button = "panel/approve"
			}
			key(t, s, button, "Enter")
			dispatcher.until(t, func() bool { return !panel.Busy() })
			want := 0
			if allow {
				want = 1
			}
			if executions != want || len(panel.messages) != 2 || panel.messages[1].text != "Finished." {
				t.Fatal("wrong panel result", executions)
			}
			if err = panel.Configure(Options{MaxSteps: -1}); err == nil || len(panel.messages) != 2 {
				t.Fatal("failed configure destroyed messages")
			}
			if err = s.Mount(panel.View("panel")); err != nil {
				t.Fatal(err)
			}
			key(t, s, "panel/message/1/edit", "Enter")
			if panel.draft.Text.String() != "Please run the tool" {
				t.Fatal("edit did not restore draft")
			}
			key(t, s, "panel/clear", "Enter")
			if len(panel.messages) != 0 || len(panel.conversation.History()) != 0 {
				t.Fatal("clear did not reset")
			}
		})
	}
}

func TestPanelIMEReadOnlyAndStop(t *testing.T) {
	dispatcher := &panelDriver{queue: make(chan func(), 16)}
	entered := make(chan struct{})
	panel, err := NewPanel(PanelOptions{RunTask: dispatcher.run, Agent: Options{Adapter: agentModel(func(ctx context.Context, _ llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
		close(entered)
		emit(llm.StreamChunk{Kind: llm.ChunkText, Text: "Working"})
		<-ctx.Done()
		return llm.Usage{}, ctx.Err()
	})}})
	if err != nil {
		t.Fatal(err)
	}
	defer panel.Close()
	panel.SetDraft("question")
	s := panelSurface(t, panel, false)
	s.Focus("panel/prompt/text")
	panel.draft.Text.SetComposition("preedit")
	s.Key(ui.KeyEvent{Key: "Enter"})
	if panel.Busy() {
		t.Fatal("IME submitted")
	}
	panel.draft.Text.CancelComposition()
	s.Key(ui.KeyEvent{Key: "Enter", Shift: true})
	if panel.Busy() {
		t.Fatal("shift enter submitted")
	}
	panel.SetReadOnly(true)
	if err := panel.Send("blocked"); err == nil {
		t.Fatal("read-only submitted")
	}
	panel.SetReadOnly(false)
	panel.SetDisabled(true)
	if err := panel.Send("blocked"); err == nil {
		t.Fatal("disabled submitted")
	}
	panel.SetDisabled(false)
	if err := panel.Send("start"); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := panel.Configure(Options{}); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if err = s.Mount(panel.View("panel")); err != nil {
		t.Fatal(err)
	}
	key(t, s, "panel/prompt/stop", "Enter")
	dispatcher.until(t, func() bool { return !panel.Busy() })
	if panel.status != "Stopped" {
		t.Fatal(panel.status)
	}
	panel.Close()
	before := len(panel.messages)
	for len(dispatcher.queue) > 0 {
		(<-dispatcher.queue)()
	}
	if len(panel.messages) != before {
		t.Fatal("late update changed closed panel")
	}
	if err := panel.Send("closed"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestPanelCloseDuringApproval(t *testing.T) {
	dispatcher := &panelDriver{queue: make(chan func(), 16)}
	panel, err := NewPanel(PanelOptions{RunTask: dispatcher.run, Agent: Options{Adapter: agentModel(func(_ context.Context, _ llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
		emit(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "a", Name: "tool", Input: map[string]any{}}})
		return llm.Usage{}, nil
	}), Tools: []Tool{{Name: "tool", RequiresApproval: true, Execute: func(context.Context, map[string]any) (any, error) {
		t.Error("closed approval executed")
		return nil, nil
	}}}}})
	if err != nil {
		t.Fatal(err)
	}
	panel.Send("run")
	dispatcher.until(t, func() bool { return panel.approval != nil })
	panel.Close()
	// History waits for the canceled worker without blocking the UI in production.
	if len(panel.conversation.History()) != 0 {
		t.Fatal("canceled history committed")
	}
}
