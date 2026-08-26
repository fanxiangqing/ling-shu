package memory

import (
	"context"
	"testing"
	"time"

	"ling-shu/internal/llm"
	"ling-shu/internal/rag"
)

type userMemoryTestProvider struct{}

func (userMemoryTestProvider) Name() string                  { return "test" }
func (userMemoryTestProvider) Configured() bool              { return true }
func (userMemoryTestProvider) DefaultChatModel() string      { return "chat" }
func (userMemoryTestProvider) DefaultEmbeddingModel() string { return "embedding" }
func (userMemoryTestProvider) Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}
func (userMemoryTestProvider) StreamChat(context.Context, llm.ChatRequest, func(llm.ChatStreamEvent) error) error {
	return nil
}
func (userMemoryTestProvider) Embeddings(context.Context, llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return &llm.EmbeddingResponse{Model: "embedding-v1", Embeddings: [][]float64{{0.5, 0.25}}}, nil
}

type userMemoryTestVectorStore struct {
	docs []rag.VectorDocument
	hits []rag.Hit
}

func (s *userMemoryTestVectorStore) EnsureCollection(context.Context, int) error { return nil }
func (s *userMemoryTestVectorStore) ReplaceByProject(context.Context, uint64, uint64, []rag.VectorDocument) error {
	return nil
}
func (s *userMemoryTestVectorStore) Search(context.Context, rag.VectorSearchRequest) ([]rag.Hit, error) {
	return s.hits, nil
}
func (s *userMemoryTestVectorStore) Close() error { return nil }
func (s *userMemoryTestVectorStore) UpsertDocuments(_ context.Context, _, _ uint64, docs []rag.VectorDocument) error {
	s.docs = append([]rag.VectorDocument(nil), docs...)
	return nil
}
func (s *userMemoryTestVectorStore) DeleteDocuments(context.Context, uint64, uint64, []int64) error {
	return nil
}

func TestVectorUserMemoryIndexUsesUserIsolation(t *testing.T) {
	scope := UserScope{TenantID: 1, ProjectID: 2, UserID: 3}
	store := &userMemoryTestVectorStore{hits: []rag.Hit{
		{RefID: 10, KBType: userMemoryKBType, DatasourceID: 3, Score: 0.9},
		{RefID: 11, KBType: userMemoryKBType, DatasourceID: 9, Score: 0.99},
		{RefID: 12, KBType: rag.KBTypeTerm, DatasourceID: 3, Score: 1},
	}}
	index := NewVectorUserMemoryIndex(func(context.Context, UserScope) (llm.Provider, error) {
		return userMemoryTestProvider{}, nil
	}, store, 8)
	scores, err := index.Recall(context.Background(), scope, "详细回答", 8)
	if err != nil {
		t.Fatalf("recall memory: %v", err)
	}
	if len(scores) != 1 || scores[10] != 0.9 {
		t.Fatalf("expected only current user's memory hit, got %v", scores)
	}

	now := time.Now()
	item := BuildUserMemory(scope, UserMemoryOperation{
		Action: UserMemoryActionRemember, Kind: UserMemoryKindPreference, Content: "回答要详细", Confidence: 1,
	}, UserMemorySourceExplicit, 0, 0, now)
	item.ID = 10
	item.UpdatedAt = now
	metadata, err := index.Index(context.Background(), item)
	if err != nil {
		t.Fatalf("index memory: %v", err)
	}
	if metadata.Provider != "test" || metadata.Model != "embedding-v1" || len(store.docs) != 1 {
		t.Fatalf("unexpected indexed memory: metadata=%+v docs=%+v", metadata, store.docs)
	}
	if store.docs[0].DatasourceID != scope.UserID || store.docs[0].RefID != item.ID {
		t.Fatalf("vector document lost user isolation: %+v", store.docs[0])
	}
}
