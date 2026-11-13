package dshgo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/herlanai/omnix/pkg/imagegen/provider_ark"
	"github.com/herlanai/omnix/pkg/llm"
)

func TestImageToolConfiguration(t *testing.T) {
	t.Setenv("ARK_API_KEY", "")
	adapter := agentModel(func(context.Context, llm.ChatRequest, func(llm.StreamChunk)) (llm.Usage, error) {
		return llm.Usage{}, nil
	})
	if _, err := New(Options{Adapter: adapter, Images: &ImageOptions{}}); err == nil || !strings.Contains(err.Error(), "ARK_API_KEY") {
		t.Fatal("missing key should fail clearly", err)
	}
	t.Setenv("ARK_API_KEY", "test-image-key")
	c := newHarness(t, Options{Adapter: adapter})
	if c.pipeline.Lookup(provider_ark.ToolName) != nil {
		t.Fatal("environment alone enabled tool")
	}
	enabled := newHarness(t, Options{Adapter: adapter, Images: &ImageOptions{}})
	if enabled.options.Images != nil || enabled.options.Timeout != 5*time.Minute {
		t.Fatal("key retained or insufficient default timeout")
	}
	if _, err := New(Options{Adapter: adapter, Images: &ImageOptions{}, Tools: []Tool{{Name: provider_ark.ToolName, Execute: func(context.Context, map[string]any) (any, error) { return nil, nil }}}}); err == nil {
		t.Fatal("duplicate builtin accepted")
	}
}

func TestImageToolAgentAndPanel(t *testing.T) {
	for _, allow := range []bool{false, true} {
		t.Run(fmt.Sprint(allow), func(t *testing.T) {
			t.Setenv("ARK_API_KEY", "fixture-ark-key")
			t.Setenv("ARK_IMAGE_MODEL", "fixture-image-model")
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != "Bearer fixture-ark-key" {
					t.Error("wrong key")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["model"] != "fixture-image-model" || body["prompt"] != "Draw a bird" || body["response_format"] != "url" || body["background"] != "transparent" || body["stream"] != false {
					t.Errorf("unexpected payload: %+v", body)
				}
				fmt.Fprint(w, `{"data":[{"url":"https://example.com/generated.png","size":"2K"}]}`)
			}))
			defer server.Close()
			steps := 0
			adapter := agentModel(func(_ context.Context, r llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
				steps++
				raw, _ := json.Marshal(r)
				if strings.Contains(string(raw), "fixture-ark-key") {
					t.Error("key leaked to model")
				}
				if steps == 1 {
					if len(r.Tools) != 1 || !strings.Contains(r.System, provider_ark.ToolName) {
						t.Error("image tool not advertised")
					}
					emit(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "image-1", Name: provider_ark.ToolName, Input: map[string]any{"prompt": "Draw a bird", "background": "transparent"}}})
				} else {
					result := r.Messages[len(r.Messages)-1].Content[0].ToolResult
					if result == nil || result.IsError == allow {
						t.Error("wrong approval result", result)
					}
					if allow && !strings.Contains(result.Content, "generated.png") {
						t.Error("image result not fed back to agent")
					}
					emit(llm.StreamChunk{Kind: llm.ChunkText, Text: "Finished."})
				}
				return llm.Usage{}, nil
			})
			dispatcher := &panelDriver{queue: make(chan func(), 16)}
			var opened, copied string
			panel, err := NewPanel(PanelOptions{RunTask: dispatcher.run, OpenImage: func(u string) { opened = u }, Copy: func(u string) { copied = u }, Agent: Options{Adapter: adapter, Images: &ImageOptions{Endpoint: server.URL}}})
			if err != nil {
				t.Fatal(err)
			}
			defer panel.Close()
			if err := panel.Send("Draw a bird"); err != nil {
				t.Fatal(err)
			}
			dispatcher.until(t, func() bool { return panel.approval != nil })
			if calls.Load() != 0 {
				t.Fatal("HTTP request before approval")
			}
			panel.decide(panel.approval, allow)
			dispatcher.until(t, func() bool { return !panel.Busy() })
			answer := panel.messages[len(panel.messages)-1]
			if allow {
				if calls.Load() != 1 || len(answer.images) != 1 {
					t.Fatal("image missing", calls.Load(), answer.images, panel.status)
				}
				surface := panelSurface(t, panel, false)
				key(t, surface, "panel/message/2/image/0/open", "Enter")
				key(t, surface, "panel/message/2/image/0/copy", "Enter")
				if opened != "https://example.com/generated.png" || copied != opened {
					t.Fatal("image actions did not use provider URL")
				}
			} else if calls.Load() != 0 || len(answer.images) != 0 {
				t.Fatal("denied tool ran")
			}
			history, _ := json.Marshal(panel.conversation.History())
			if strings.Contains(string(history), "fixture-ark-key") {
				t.Fatal("credential persisted in history")
			}
		})
	}
}

func TestImageToolCancel(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	c := newHarness(t, Options{Adapter: agentModel(func(_ context.Context, _ llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
		emit(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "cancel", Name: provider_ark.ToolName, Input: map[string]any{"prompt": "test"}}})
		return llm.Usage{}, nil
	}), Images: &ImageOptions{APIKey: "test", Endpoint: server.URL}, Approve: func(context.Context, llm.ToolCall) (bool, error) { return true, nil }})
	done := make(chan error, 1)
	go func() { _, err := c.Run(context.Background(), "generate", nil); done <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request never started")
	}
	c.Cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request did not cancel")
	}
}

func TestImageToolFailureHasNoImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":"bad_request","message":"fixture-secret rejected"}}`)
	}))
	defer server.Close()
	step := 0
	c := newHarness(t, Options{Images: &ImageOptions{APIKey: "fixture-secret", Endpoint: server.URL}, Approve: func(context.Context, llm.ToolCall) (bool, error) { return true, nil }, Adapter: agentModel(func(_ context.Context, r llm.ChatRequest, emit func(llm.StreamChunk)) (llm.Usage, error) {
		step++
		if step == 1 {
			emit(llm.StreamChunk{Kind: llm.ChunkToolCall, ToolCall: &llm.ToolCall{ID: "failure", Name: provider_ark.ToolName, Input: map[string]any{"prompt": "test"}}})
		} else {
			result := r.Messages[len(r.Messages)-1].Content[0].ToolResult
			if result == nil || !result.IsError || strings.Contains(result.Content, "fixture-secret") {
				t.Error("provider failure not safely returned", result)
			}
			emit(llm.StreamChunk{Kind: llm.ChunkText, Text: "Image generation failed."})
		}
		return llm.Usage{}, nil
	})})
	result, err := c.Run(context.Background(), "generate", func(e Event) {
		if e.Kind == "images" {
			t.Error("failure emitted images")
		}
	})
	if err != nil || len(result.Images) != 0 {
		t.Fatal(result, err)
	}
}
