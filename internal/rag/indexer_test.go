package rag

import (
	"context"
	"testing"

	"ling-shu/internal/llm"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestIndexerRebuild(t *testing.T) {
	knowledgeRepo := &fakeKnowledgeRepository{
		terms: []model.KBTerm{
			{BaseModel: model.BaseModel{ID: 11}, TenantID: 1, ProjectID: 2, Term: "GMV", Definition: "成交金额"},
		},
		metrics: []model.KBMetric{
			{BaseModel: model.BaseModel{ID: 12}, TenantID: 1, ProjectID: 2, Name: "销售额", Description: "订单销售额", Formula: "sum(pay_amount)", DatasourceID: 7},
		},
		fewShots: []model.KBFewShotSQL{
			{BaseModel: model.BaseModel{ID: 13}, TenantID: 1, ProjectID: 2, DatasourceID: 7, Question: "今天销售额", SQLText: "select sum(pay_amount) from orders"},
		},
	}
	ragRepo := &fakeRAGRepository{}
	vectorStore := &fakeVectorStore{}
	indexer := NewIndexer(knowledgeRepo, ragRepo, fakeEmbeddingProvider{configured: true}, vectorStore, "test_chunks")

	result, err := indexer.Rebuild(context.Background(), RebuildRequest{TenantID: 1, ProjectID: 2})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if result.ChunkCount != 3 || result.VectorCount != 3 || result.Collection != "test_chunks" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(vectorStore.docs) != 3 {
		t.Fatalf("expected 3 vector docs, got %d", len(vectorStore.docs))
	}
	if len(ragRepo.chunks) != 3 {
		t.Fatalf("expected 3 metadata chunks, got %d", len(ragRepo.chunks))
	}
	if ragRepo.chunks[0].EmbeddingProvider != "fake" || ragRepo.chunks[0].VectorCollection != "test_chunks" || ragRepo.chunks[0].VectorID == "" {
		t.Fatalf("unexpected chunk metadata: %+v", ragRepo.chunks[0])
	}
	if vectorStore.docs[1].DatasourceID != 7 || vectorStore.docs[1].KBType != KBTypeMetric {
		t.Fatalf("unexpected metric vector doc: %+v", vectorStore.docs[1])
	}
}

func TestIndexerRequiresConfiguredEmbedder(t *testing.T) {
	indexer := NewIndexer(&fakeKnowledgeRepository{
		terms: []model.KBTerm{{BaseModel: model.BaseModel{ID: 11}, TenantID: 1, ProjectID: 2, Term: "GMV", Definition: "成交金额"}},
	}, &fakeRAGRepository{}, fakeEmbeddingProvider{}, &fakeVectorStore{}, "test_chunks")

	_, err := indexer.Rebuild(context.Background(), RebuildRequest{TenantID: 1, ProjectID: 2})
	if err != ErrEmbeddingProviderNotConfigured {
		t.Fatalf("expected embedding provider error, got %v", err)
	}
}

type fakeRAGRepository struct {
	chunks []model.KBChunk
}

func (r *fakeRAGRepository) ReplaceChunks(ctx context.Context, tenantID uint64, projectID uint64, chunks []model.KBChunk) error {
	r.chunks = append([]model.KBChunk(nil), chunks...)
	return nil
}

func (r *fakeRAGRepository) ListChunks(ctx context.Context, filter repository.RAGChunkFilter, page repository.Page) ([]model.KBChunk, int64, error) {
	return r.chunks, int64(len(r.chunks)), nil
}

type fakeVectorStore struct {
	docs []VectorDocument
	hits []Hit
}

func (s *fakeVectorStore) EnsureCollection(ctx context.Context, dimension int) error {
	return nil
}

func (s *fakeVectorStore) ReplaceByProject(ctx context.Context, tenantID uint64, projectID uint64, docs []VectorDocument) error {
	s.docs = append([]VectorDocument(nil), docs...)
	return nil
}

func (s *fakeVectorStore) Search(ctx context.Context, req VectorSearchRequest) ([]Hit, error) {
	return append([]Hit(nil), s.hits...), nil
}

func (s *fakeVectorStore) Close() error {
	return nil
}

type fakeEmbeddingProvider struct {
	configured bool
}

func (p fakeEmbeddingProvider) Name() string                  { return "fake" }
func (p fakeEmbeddingProvider) Configured() bool              { return p.configured }
func (p fakeEmbeddingProvider) DefaultChatModel() string      { return "fake-chat" }
func (p fakeEmbeddingProvider) DefaultEmbeddingModel() string { return "fake-embedding" }
func (p fakeEmbeddingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, nil
}
func (p fakeEmbeddingProvider) StreamChat(ctx context.Context, req llm.ChatRequest, onEvent func(llm.ChatStreamEvent) error) error {
	return nil
}
func (p fakeEmbeddingProvider) Embeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	resp := &llm.EmbeddingResponse{Model: "fake-embedding"}
	for idx := range req.Input {
		resp.Embeddings = append(resp.Embeddings, []float64{float64(idx + 1), 0.5})
	}
	return resp, nil
}
