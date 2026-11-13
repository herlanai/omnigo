package dshgo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHarnessUsesOmnixDeepSeek(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request %s headers %v", r.URL, r.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["model"] != "deepseek-chat" {
			t.Errorf("model %+v", payload)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	conversation, err := New(Options{DeepSeek: DeepSeekOptions{APIKey: "test-key", BaseURL: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer conversation.Close()
	result, err := conversation.Run(context.Background(), "hi", nil)
	if err != nil || result.Text != "hello" || result.Usage.PromptTokens != 3 {
		t.Fatalf("result %+v err %v", result, err)
	}
}
