package llm

import "context"

const ProviderAliyun = "aliyun"

type Provider interface {
	Name() string
	Configured() bool
	DefaultChatModel() string
	DefaultEmbeddingModel() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	StreamChat(ctx context.Context, req ChatRequest, onEvent func(ChatStreamEvent) error) error
	Embeddings(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Model        string       `json:"model"`
	Content      string       `json:"content"`
	Usage        TokenUsage   `json:"usage"`
	RawRequestID string       `json:"raw_request_id,omitempty"`
	Choices      []ChatChoice `json:"choices,omitempty"`
}

type ChatStreamEvent struct {
	Model        string     `json:"model,omitempty"`
	Role         string     `json:"role,omitempty"`
	Delta        string     `json:"delta,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Usage        TokenUsage `json:"usage,omitempty"`
	Done         bool       `json:"done"`
}

type ChatChoice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type EmbeddingRequest struct {
	Model string   `json:"model,omitempty"`
	Input []string `json:"input"`
}

type EmbeddingResponse struct {
	Model      string          `json:"model"`
	Embeddings [][]float64     `json:"embeddings"`
	Usage      TokenUsage      `json:"usage"`
	Data       []EmbeddingItem `json:"data,omitempty"`
}

type EmbeddingItem struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}
