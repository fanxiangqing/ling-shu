package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAliyunProviderChat(t *testing.T) {
	provider := NewAliyunProvider(AliyunConfig{
		APIKey:    "test-key",
		BaseURL:   "https://example.com",
		ChatModel: "qwen-plus",
	})
	provider.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", got)
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["model"] != "qwen-plus" {
			t.Fatalf("expected default model qwen-plus, got %v", req["model"])
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
			"id":"chatcmpl-test",
			"model":"qwen-plus",
			"choices":[{"index":0,"message":{"role":"assistant","content":"你好"}}],
			"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}
		}`)),
		}, nil
	})}

	resp, err := provider.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "你好"}},
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Content != "你好" {
		t.Fatalf("expected content 你好, got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("expected total tokens 5, got %d", resp.Usage.TotalTokens)
	}
}

func TestAliyunProviderStreamChat(t *testing.T) {
	provider := NewAliyunProvider(AliyunConfig{
		APIKey:    "test-key",
		BaseURL:   "https://example.com",
		ChatModel: "qwen-plus",
	})
	provider.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["stream"] != true {
			t.Fatalf("expected stream=true, got %v", req["stream"])
		}

		body := strings.Join([]string{
			`data: {"model":"qwen-plus","choices":[{"index":0,"delta":{"role":"assistant","content":"你"}}]}`,
			`data: {"model":"qwen-plus","choices":[{"index":0,"delta":{"content":"好"}}]}`,
			`data: {"model":"qwen-plus","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
			``,
		}, "\n\n")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}

	var content strings.Builder
	var doneCount int
	err := provider.StreamChat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "你好"}},
	}, func(event ChatStreamEvent) error {
		content.WriteString(event.Delta)
		if event.Done {
			doneCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream chat: %v", err)
	}
	if content.String() != "你好" {
		t.Fatalf("expected streamed content 你好, got %q", content.String())
	}
	if doneCount == 0 {
		t.Fatal("expected at least one done event")
	}
}

func TestAliyunProviderRequiresAPIKey(t *testing.T) {
	provider := NewAliyunProvider(AliyunConfig{})
	_, err := provider.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

type roundTripFunc func(r *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
