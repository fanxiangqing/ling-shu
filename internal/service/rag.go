package service

import (
	"context"
	"errors"
	"time"

	ragpkg "ling-shu/internal/rag"

	"go.uber.org/zap"
)

type RAGService struct {
	indexer   *ragpkg.Indexer
	retriever RAGRetriever
	logger    *zap.Logger
}

type RebuildRAGInput struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Limit        int
}

type SearchRAGInput struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Question     string
	Limit        int
}

func NewRAGService(indexer *ragpkg.Indexer, retriever RAGRetriever) *RAGService {
	return &RAGService{
		indexer:   indexer,
		retriever: retriever,
		logger:    zap.NewNop(),
	}
}

func (s *RAGService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	s.logger = logger
}

func (s *RAGService) Rebuild(ctx context.Context, input RebuildRAGInput) (*ragpkg.RebuildResult, error) {
	if s == nil || s.indexer == nil {
		return nil, ErrInvalidInput
	}
	startedAt := time.Now()
	s.logger.Info("rag rebuild started",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Int("limit", input.Limit),
	)
	result, err := s.indexer.Rebuild(ctx, ragpkg.RebuildRequest{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		DatasourceID: input.DatasourceID,
		Limit:        input.Limit,
	})
	if err != nil {
		translated := translateRAGError(err)
		s.logger.Warn("rag rebuild failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Duration("duration", time.Since(startedAt)),
			zap.Error(translated),
		)
		return nil, translated
	}
	s.logger.Info("rag rebuild succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Int("chunk_count", result.ChunkCount),
		zap.Int("vector_count", result.VectorCount),
		zap.Duration("duration", time.Since(startedAt)),
	)
	return result, nil
}

func (s *RAGService) RefreshKnowledgeIndex(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) error {
	if s == nil || s.indexer == nil {
		return nil
	}
	_, err := s.Rebuild(ctx, RebuildRAGInput{
		TenantID:     tenantID,
		ProjectID:    projectID,
		DatasourceID: datasourceID,
	})
	return err
}

func (s *RAGService) Search(ctx context.Context, input SearchRAGInput) (*ragpkg.Context, error) {
	if s == nil || s.retriever == nil {
		return nil, ErrInvalidInput
	}
	startedAt := time.Now()
	result, err := s.retriever.Retrieve(ctx, ragpkg.Request{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		DatasourceID: input.DatasourceID,
		Question:     input.Question,
		Limit:        input.Limit,
	})
	if err != nil {
		translated := translateRAGError(err)
		s.logger.Warn("rag search failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Int("limit", input.Limit),
			zap.Duration("duration", time.Since(startedAt)),
			zap.Error(translated),
		)
		return nil, translated
	}
	s.logger.Info("rag search succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Int("limit", input.Limit),
		zap.Int("hit_count", len(result.Hits)),
		zap.Duration("duration", time.Since(startedAt)),
	)
	return result, nil
}

func translateRAGError(err error) error {
	switch {
	case errors.Is(err, ragpkg.ErrInvalidRequest):
		return ErrInvalidInput
	case errors.Is(err, ragpkg.ErrEmbeddingProviderNotConfigured), errors.Is(err, ragpkg.ErrVectorStoreNotConfigured):
		return ErrProviderNotConfigured
	default:
		return err
	}
}
