package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("provider api key is not configured")

type AliyunConfig struct {
	APIKey         string
	BaseURL        string
	ChatModel      string
	EmbeddingModel string
	Timeout        time.Duration
}

type AliyunProvider struct {
	apiKey         string
	baseURL        string
	chatModel      string
	embeddingModel string
	client         *http.Client
}

func NewAliyunProvider(cfg AliyunConfig) *AliyunProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 180 * time.Second
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	chatModel := cfg.ChatModel
	if chatModel == "" {
		chatModel = "qwen-plus"
	}
	embeddingModel := cfg.EmbeddingModel
	if embeddingModel == "" {
		embeddingModel = "text-embedding-v4"
	}

	return &AliyunProvider{
		apiKey:         cfg.APIKey,
		baseURL:        baseURL,
		chatModel:      chatModel,
		embeddingModel: embeddingModel,
		client:         &http.Client{Timeout: timeout},
	}
}

func (p *AliyunProvider) Name() string {
	return ProviderAliyun
}

func (p *AliyunProvider) Configured() bool {
	return strings.TrimSpace(p.apiKey) != ""
}

func (p *AliyunProvider) DefaultChatModel() string {
	return p.chatModel
}

func (p *AliyunProvider) DefaultEmbeddingModel() string {
	return p.embeddingModel
}

func (p *AliyunProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	if len(req.Messages) == 0 {
		return nil, errors.New("messages is required")
	}

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	body := map[string]any{
		"model":    model,
		"messages": req.Messages,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	var result openAIChatResponse
	if err := p.postJSON(ctx, "/chat/completions", body, &result); err != nil {
		return nil, err
	}

	resp := &ChatResponse{
		Model:        result.Model,
		Usage:        result.Usage,
		RawRequestID: result.ID,
	}
	for _, choice := range result.Choices {
		resp.Choices = append(resp.Choices, ChatChoice{
			Index:   choice.Index,
			Message: choice.Message,
		})
	}
	if len(result.Choices) > 0 {
		resp.Content = result.Choices[0].Message.Content
	}
	if resp.Model == "" {
		resp.Model = model
	}

	return resp, nil
}

func (p *AliyunProvider) StreamChat(ctx context.Context, req ChatRequest, onEvent func(ChatStreamEvent) error) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	if len(req.Messages) == 0 {
		return errors.New("messages is required")
	}
	if onEvent == nil {
		return errors.New("on_event is required")
	}

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	body := map[string]any{
		"model":    model,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	content, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr apiErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error.Message != "" {
			return fmt.Errorf("aliyun llm stream api error: status=%d message=%s", resp.StatusCode, apiErr.Error.Message)
		}
		return fmt.Errorf("aliyun llm stream api error: status=%d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return onEvent(ChatStreamEvent{Model: model, Done: true})
		}

		var chunk openAIChatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return fmt.Errorf("decode stream chunk: %w", err)
		}
		event := chatStreamChunkToEvent(model, chunk)
		if event.Delta == "" && event.Role == "" && event.FinishReason == "" && event.Usage.TotalTokens == 0 {
			continue
		}
		if err := onEvent(event); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	return nil
}

func (p *AliyunProvider) Embeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	if len(req.Input) == 0 {
		return nil, errors.New("input is required")
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	body := map[string]any{
		"model": model,
		"input": req.Input,
	}

	var result openAIEmbeddingResponse
	if err := p.postJSON(ctx, "/embeddings", body, &result); err != nil {
		return nil, err
	}

	resp := &EmbeddingResponse{
		Model: result.Model,
		Usage: result.Usage,
	}
	for _, item := range result.Data {
		resp.Data = append(resp.Data, EmbeddingItem{
			Index:     item.Index,
			Embedding: item.Embedding,
		})
		resp.Embeddings = append(resp.Embeddings, item.Embedding)
	}
	if resp.Model == "" {
		resp.Model = model
	}

	return resp, nil
}

func (p *AliyunProvider) postJSON(ctx context.Context, path string, payload any, target any) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var apiErr apiErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error.Message != "" {
			return fmt.Errorf("aliyun llm api error: status=%d message=%s", resp.StatusCode, apiErr.Error.Message)
		}
		return fmt.Errorf("aliyun llm api error: status=%d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int     `json:"index"`
		Message Message `json:"message"`
	} `json:"choices"`
	Usage TokenUsage `json:"usage"`
}

type openAIChatStreamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage TokenUsage `json:"usage"`
}

func chatStreamChunkToEvent(defaultModel string, chunk openAIChatStreamChunk) ChatStreamEvent {
	model := chunk.Model
	if model == "" {
		model = defaultModel
	}
	event := ChatStreamEvent{
		Model: model,
		Usage: chunk.Usage,
	}
	if len(chunk.Choices) > 0 {
		event.Role = chunk.Choices[0].Delta.Role
		event.Delta = chunk.Choices[0].Delta.Content
		event.FinishReason = chunk.Choices[0].FinishReason
		event.Done = event.FinishReason != ""
	}
	return event
}

type openAIEmbeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage TokenUsage `json:"usage"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
