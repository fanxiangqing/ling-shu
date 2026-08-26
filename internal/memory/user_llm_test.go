package memory

import (
	"context"
	"testing"

	"ling-shu/internal/llm"
)

type userMemoryExtractorProvider struct {
	content string
}

func (p userMemoryExtractorProvider) Name() string                  { return "test" }
func (p userMemoryExtractorProvider) Configured() bool              { return true }
func (p userMemoryExtractorProvider) DefaultChatModel() string      { return "chat" }
func (p userMemoryExtractorProvider) DefaultEmbeddingModel() string { return "embedding" }
func (p userMemoryExtractorProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: p.content}, nil
}
func (p userMemoryExtractorProvider) StreamChat(context.Context, llm.ChatRequest, func(llm.ChatStreamEvent) error) error {
	return nil
}
func (p userMemoryExtractorProvider) Embeddings(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return &llm.EmbeddingResponse{}, nil
}

func TestLLMUserMemoryExtractorRequiresExplicitConfidence(t *testing.T) {
	extractor := NewLLMUserMemoryExtractor(func(context.Context, UserScope) (llm.Provider, error) {
		return userMemoryExtractorProvider{content: `{"memories":[
			{"kind":"profile","content":"我负责项目经营分析","confidence":0},
			{"kind":"preference","content":"以后默认回答详细一些","confidence":0.9}
		]}`}, nil
	})
	operations, err := extractor.Extract(context.Background(), UserTurnInput{
		Scope:    UserScope{TenantID: 1, ProjectID: 2, UserID: 3},
		Question: "我负责项目经营分析，以后默认回答详细一些",
		Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("extract memories: %v", err)
	}
	if len(operations) != 1 || operations[0].Content != "以后默认回答详细一些" {
		t.Fatalf("expected only high-confidence memory, got %+v", operations)
	}
}
