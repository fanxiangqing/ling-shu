package memory

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"ling-shu/internal/llm"
	"ling-shu/internal/rag"
)

const userMemoryKBType = "user_memory"

type UserMemoryVectorMetadata struct {
	Provider string
	Model    string
	VectorID string
}

type UserMemorySemanticIndex interface {
	Recall(ctx context.Context, scope UserScope, question string, limit int) (map[uint64]float32, error)
	Index(ctx context.Context, item UserMemory) (UserMemoryVectorMetadata, error)
	Delete(ctx context.Context, scope UserScope, memoryID uint64) error
}

type VectorUserMemoryIndex struct {
	resolver UserMemoryLLMResolver
	store    rag.VectorStore
	topK     int
}

func NewVectorUserMemoryIndex(resolver UserMemoryLLMResolver, store rag.VectorStore, topK int) *VectorUserMemoryIndex {
	if topK <= 0 {
		topK = userMemoryRecallLimit
	}
	return &VectorUserMemoryIndex{resolver: resolver, store: store, topK: topK}
}

func (i *VectorUserMemoryIndex) Recall(ctx context.Context, scope UserScope, question string, limit int) (map[uint64]float32, error) {
	if i == nil || i.resolver == nil || i.store == nil || scope.ProjectID == 0 || strings.TrimSpace(question) == "" {
		return nil, nil
	}
	provider, err := i.resolver(ctx, scope)
	if err != nil || provider == nil || !provider.Configured() {
		return nil, err
	}
	response, err := provider.Embeddings(ctx, llm.EmbeddingRequest{Input: []string{question}})
	if err != nil {
		return nil, err
	}
	if len(response.Embeddings) == 0 || len(response.Embeddings[0]) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = i.topK
	}
	hits, err := i.store.Search(ctx, rag.VectorSearchRequest{
		TenantID: scope.TenantID, ProjectID: scope.ProjectID, DatasourceID: scope.UserID,
		Vector: memoryFloat64ToFloat32(response.Embeddings[0]), TopK: limit,
	})
	if err != nil {
		return nil, err
	}
	scores := make(map[uint64]float32, len(hits))
	for _, hit := range hits {
		if hit.KBType == userMemoryKBType && hit.DatasourceID == scope.UserID {
			scores[hit.RefID] = hit.Score
		}
	}
	return scores, nil
}

func (i *VectorUserMemoryIndex) Index(ctx context.Context, item UserMemory) (UserMemoryVectorMetadata, error) {
	if i == nil || i.resolver == nil || i.store == nil || item.ID == 0 || item.Scope.ProjectID == 0 || !item.IsUsable(item.UpdatedAt) {
		return UserMemoryVectorMetadata{}, nil
	}
	incremental, ok := i.store.(rag.IncrementalVectorStore)
	if !ok {
		return UserMemoryVectorMetadata{}, errors.New("vector store does not support incremental updates")
	}
	if item.ID > math.MaxInt64 {
		return UserMemoryVectorMetadata{}, fmt.Errorf("user memory id exceeds vector id range: %d", item.ID)
	}
	provider, err := i.resolver(ctx, item.Scope)
	if err != nil || provider == nil || !provider.Configured() {
		return UserMemoryVectorMetadata{}, err
	}
	text := strings.TrimSpace(strings.Join([]string{item.Kind, item.Key, item.Content}, " "))
	response, err := provider.Embeddings(ctx, llm.EmbeddingRequest{Input: []string{text}})
	if err != nil {
		return UserMemoryVectorMetadata{}, err
	}
	if len(response.Embeddings) == 0 || len(response.Embeddings[0]) == 0 {
		return UserMemoryVectorMetadata{}, errors.New("empty user memory embedding")
	}
	vectorID := int64(item.ID)
	err = incremental.UpsertDocuments(ctx, item.Scope.TenantID, item.Scope.ProjectID, []rag.VectorDocument{{
		ID: vectorID, TenantID: item.Scope.TenantID, ProjectID: item.Scope.ProjectID,
		DatasourceID: item.Scope.UserID, KBType: userMemoryKBType, RefID: item.ID,
		ChunkText: item.Content, Vector: memoryFloat64ToFloat32(response.Embeddings[0]),
	}})
	if err != nil {
		return UserMemoryVectorMetadata{}, err
	}
	return UserMemoryVectorMetadata{
		Provider: provider.Name(), Model: firstNonEmptyMemory(response.Model, provider.DefaultEmbeddingModel()),
		VectorID: strconv.FormatInt(vectorID, 10),
	}, nil
}

func (i *VectorUserMemoryIndex) Delete(ctx context.Context, scope UserScope, memoryID uint64) error {
	if i == nil || i.store == nil || scope.ProjectID == 0 || memoryID == 0 {
		return nil
	}
	incremental, ok := i.store.(rag.IncrementalVectorStore)
	if !ok {
		return errors.New("vector store does not support incremental updates")
	}
	if memoryID > math.MaxInt64 {
		return nil
	}
	return incremental.DeleteDocuments(ctx, scope.TenantID, scope.ProjectID, []int64{int64(memoryID)})
}

func memoryFloat64ToFloat32(values []float64) []float32 {
	out := make([]float32, len(values))
	for index, value := range values {
		out[index] = float32(value)
	}
	return out
}
