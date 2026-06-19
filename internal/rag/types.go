package rag

import (
	"context"
	"errors"

	"ling-shu/internal/query"
)

var (
	ErrInvalidRequest                 = errors.New("invalid rag request")
	ErrEmbeddingProviderNotConfigured = errors.New("rag embedding provider is not configured")
	ErrVectorStoreNotConfigured       = errors.New("rag vector store is not configured")
)

const (
	KBTypeTerm    = "term"
	KBTypeMetric  = "metric"
	KBTypeFewShot = "fewshot"
	defaultTopK   = 8
)

type Request struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Question     string
	Limit        int
}

type Context struct {
	BusinessTerms []query.AgentKnowledge `json:"business_terms"`
	Metrics       []query.AgentKnowledge `json:"metrics"`
	FewShots      []query.AgentFewShot   `json:"few_shots"`
	Hits          []Hit                  `json:"hits,omitempty"`
}

type Hit struct {
	ID           int64   `json:"id"`
	Score        float32 `json:"score"`
	TenantID     uint64  `json:"tenant_id"`
	ProjectID    uint64  `json:"project_id"`
	DatasourceID uint64  `json:"datasource_id,omitempty"`
	KBType       string  `json:"kb_type"`
	RefID        uint64  `json:"ref_id"`
	ChunkText    string  `json:"chunk_text"`
}

type VectorDocument struct {
	ID           int64
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	KBType       string
	RefID        uint64
	ChunkNo      int
	ChunkText    string
	Vector       []float32
}

type VectorSearchRequest struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Vector       []float32
	TopK         int
}

type VectorStore interface {
	EnsureCollection(ctx context.Context, dimension int) error
	ReplaceByProject(ctx context.Context, tenantID uint64, projectID uint64, docs []VectorDocument) error
	Search(ctx context.Context, req VectorSearchRequest) ([]Hit, error)
	Close() error
}
