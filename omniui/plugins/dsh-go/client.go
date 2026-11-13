// The legacy raw-stream client is independent of the omnix-backed harness.
// Its callbacks run on the calling goroutine, not the UI goroutine.
package dshgo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Function struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type ToolCall struct {
	Index    int      `json:"index,omitempty"`
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	Function Function `json:"function"`
}
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Reasoning  string     `json:"reasoning_content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
type Request struct {
	MaxTokens       int               `json:"max_tokens,omitempty"`
	Temperature     float64           `json:"temperature,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
	Model           string            `json:"model"`
	Messages        []Message         `json:"messages"`
	Tools           []json.RawMessage `json:"tools,omitempty"`
}
type Delta struct {
	Reasoning string     `json:"reasoning_content"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
}
type Usage struct {
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	PromptTokens          int `json:"prompt_tokens"`
	CompletionTokens      int `json:"completion_tokens"`
	TotalTokens           int `json:"total_tokens"`
}
type ClientOptions struct {
	BaseURL, APIKey string
	Client          *http.Client
	MaxStreamBytes  int64
}
type Client struct {
	endpoint, key string
	http          *http.Client
	limit         int64
}

func NewClient(o ClientOptions) (*Client, error) {
	if o.BaseURL == "" {
		o.BaseURL = "https://api.deepseek.com"
	}
	u, e := url.Parse(o.BaseURL)
	if e != nil {
		return nil, e
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && local)) {
		return nil, errors.New("dshgo: HTTPS or loopback endpoint required")
	}
	client := http.Client{Timeout: 2 * time.Minute}
	if o.Client != nil {
		client = *o.Client
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if o.MaxStreamBytes <= 0 {
		o.MaxStreamBytes = 32 << 20
	}
	return &Client{strings.TrimRight(o.BaseURL, "/") + "/chat/completions", o.APIKey, &client, o.MaxStreamBytes}, nil
}
func (c *Client) Chat(ctx context.Context, r Request, onDelta func(Delta) error) (Usage, error) {
	var usage Usage
	if r.Model == "" || len(r.Messages) == 0 || onDelta == nil {
		return usage, errors.New("dshgo: model, messages and callback required")
	}
	payload := struct {
		Request
		Stream        bool            `json:"stream"`
		StreamOptions map[string]bool `json:"stream_options"`
	}{r, true, map[string]bool{"include_usage": true}}
	b, e := json.Marshal(payload)
	if e != nil {
		return usage, e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(b))
	if e != nil {
		return usage, e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return usage, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return usage, fmt.Errorf("dshgo: provider HTTP status %d", resp.StatusCode)
	}
	limited := &io.LimitedReader{R: resp.Body, N: c.limit + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var data []string
	consume := func() (bool, error) {
		if len(data) == 0 {
			return false, nil
		}
		body := strings.Join(data, "\n")
		data = nil
		if body == "[DONE]" {
			return true, nil
		}
		var event struct {
			Choices []struct {
				Delta Delta `json:"delta"`
			} `json:"choices"`
			Usage *Usage          `json:"usage"`
			Error json.RawMessage `json:"error"`
		}
		if e := json.Unmarshal([]byte(body), &event); e != nil {
			return false, e
		}
		if len(event.Error) > 0 && string(event.Error) != "null" {
			return false, errors.New("dshgo: provider stream error")
		}
		if event.Usage != nil {
			usage = *event.Usage
		}
		for _, choice := range event.Choices {
			if e := onDelta(choice.Delta); e != nil {
				return false, e
			}
		}
		return false, nil
	}
	for scanner.Scan() {
		if limited.N <= 0 {
			return usage, errors.New("dshgo: stream limit exceeded")
		}
		line := scanner.Text()
		if line == "" {
			done, e := consume()
			if e != nil || done {
				return usage, e
			}
		} else if strings.HasPrefix(line, "data:") {
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			data = append(data, value)
		}
	}
	if e = scanner.Err(); e != nil {
		return usage, e
	}
	if limited.N <= 0 {
		return usage, errors.New("dshgo: stream limit exceeded")
	}
	if done, e := consume(); e != nil || done {
		return usage, e
	}
	return usage, io.ErrUnexpectedEOF
}
