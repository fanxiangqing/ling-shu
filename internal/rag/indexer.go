package rag

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"

	"ling-shu/internal/llm"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

const (
	defaultRebuildLimit = 1000
	rebuildPageSize     = 100
)

type Indexer struct {
	knowledgeRepo repository.KnowledgeRepository
	ragRepo       repository.RAGRepository
	embedder      llm.Provider
	vectorStore   VectorStore
	collection    string
}

type RebuildRequest struct {
	TenantID     uint64 `json:"tenant_id"`
	ProjectID    uint64 `json:"project_id"`
	DatasourceID uint64 `json:"datasource_id,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type RebuildResult struct {
	Collection     string `json:"collection"`
	ChunkCount     int    `json:"chunk_count"`
	VectorCount    int    `json:"vector_count"`
	EmbeddingModel string `json:"embedding_model"`
}

func NewIndexer(knowledgeRepo repository.KnowledgeRepository, ragRepo repository.RAGRepository, embedder llm.Provider, vectorStore VectorStore, collection string) *Indexer {
	return &Indexer{
		knowledgeRepo: knowledgeRepo,
		ragRepo:       ragRepo,
		embedder:      embedder,
		vectorStore:   vectorStore,
		collection:    collection,
	}
}

func (i *Indexer) Rebuild(ctx context.Context, req RebuildRequest) (*RebuildResult, error) {
	if req.TenantID == 0 || req.ProjectID == 0 {
		return nil, ErrInvalidRequest
	}
	if i.knowledgeRepo == nil || i.ragRepo == nil {
		return nil, ErrInvalidRequest
	}
	if i.vectorStore == nil {
		return nil, ErrVectorStoreNotConfigured
	}

	chunks, err := i.collectChunks(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		if err := i.vectorStore.ReplaceByProject(ctx, req.TenantID, req.ProjectID, nil); err != nil {
			return nil, err
		}
		if err := i.ragRepo.ReplaceChunks(ctx, req.TenantID, req.ProjectID, nil); err != nil {
			return nil, err
		}
		return &RebuildResult{Collection: i.collection}, nil
	}
	if i.embedder == nil || !i.embedder.Configured() {
		return nil, ErrEmbeddingProviderNotConfigured
	}

	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.ChunkText)
	}
	embeddingResp, err := i.embedder.Embeddings(ctx, llm.EmbeddingRequest{Input: texts})
	if err != nil {
		return nil, err
	}
	if len(embeddingResp.Embeddings) != len(chunks) {
		return nil, fmt.Errorf("embedding count mismatch: chunks=%d embeddings=%d", len(chunks), len(embeddingResp.Embeddings))
	}

	docs := make([]VectorDocument, 0, len(chunks))
	now := time.Now()
	for idx, chunk := range chunks {
		vector := float64ToFloat32(embeddingResp.Embeddings[idx])
		if len(vector) == 0 {
			return nil, fmt.Errorf("empty embedding at index %d", idx)
		}
		vectorID := deterministicVectorID(chunk.TenantID, chunk.ProjectID, chunk.DatasourceID, chunk.KBType, chunk.RefID, chunk.ChunkNo)
		chunks[idx].VectorID = strconv.FormatInt(vectorID, 10)
		chunks[idx].VectorCollection = i.collection
		chunks[idx].EmbeddingProvider = i.embedder.Name()
		chunks[idx].EmbeddingModel = firstNonEmpty(embeddingResp.Model, i.embedder.DefaultEmbeddingModel())
		chunks[idx].CreatedAt = now
		docs = append(docs, VectorDocument{
			ID:           vectorID,
			TenantID:     chunk.TenantID,
			ProjectID:    chunk.ProjectID,
			DatasourceID: chunk.DatasourceID,
			KBType:       chunk.KBType,
			RefID:        chunk.RefID,
			ChunkNo:      chunk.ChunkNo,
			ChunkText:    chunk.ChunkText,
			Vector:       vector,
		})
	}

	if err := i.vectorStore.ReplaceByProject(ctx, req.TenantID, req.ProjectID, docs); err != nil {
		return nil, err
	}
	if err := i.ragRepo.ReplaceChunks(ctx, req.TenantID, req.ProjectID, chunks); err != nil {
		return nil, err
	}

	return &RebuildResult{
		Collection:     i.collection,
		ChunkCount:     len(chunks),
		VectorCount:    len(docs),
		EmbeddingModel: firstNonEmpty(embeddingResp.Model, i.embedder.DefaultEmbeddingModel()),
	}, nil
}

func (i *Indexer) collectChunks(ctx context.Context, req RebuildRequest) ([]model.KBChunk, error) {
	enabled := true
	limit := rebuildLimit(req.Limit)
	filter := repository.KnowledgeFilter{
		TenantID:     req.TenantID,
		ProjectID:    req.ProjectID,
		DatasourceID: req.DatasourceID,
		Enabled:      &enabled,
	}

	var chunks []model.KBChunk
	for pageNo := 1; len(chunks) < limit; pageNo++ {
		page := repository.Page{Page: pageNo, PageSize: rebuildPageSize}
		terms, _, err := i.knowledgeRepo.ListTerms(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		for _, term := range terms {
			chunks = append(chunks, chunkFromTerm(term))
			if len(chunks) >= limit {
				return chunks, nil
			}
		}
		if len(terms) < rebuildPageSize {
			break
		}
	}
	for pageNo := 1; len(chunks) < limit; pageNo++ {
		page := repository.Page{Page: pageNo, PageSize: rebuildPageSize}
		metrics, _, err := i.knowledgeRepo.ListMetrics(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		for _, metric := range metrics {
			chunks = append(chunks, chunkFromMetric(metric))
			if len(chunks) >= limit {
				return chunks, nil
			}
		}
		if len(metrics) < rebuildPageSize {
			break
		}
	}
	for pageNo := 1; len(chunks) < limit; pageNo++ {
		page := repository.Page{Page: pageNo, PageSize: rebuildPageSize}
		fewShots, _, err := i.knowledgeRepo.ListFewShots(ctx, filter, page)
		if err != nil {
			return nil, err
		}
		for _, fewShot := range fewShots {
			chunks = append(chunks, chunkFromFewShot(fewShot))
			if len(chunks) >= limit {
				return chunks, nil
			}
		}
		if len(fewShots) < rebuildPageSize {
			break
		}
	}
	return chunks, nil
}

func rebuildLimit(limit int) int {
	if limit <= 0 {
		return defaultRebuildLimit
	}
	if limit > defaultRebuildLimit {
		return defaultRebuildLimit
	}
	return limit
}

func chunkFromTerm(term model.KBTerm) model.KBChunk {
	parts := []string{
		"业务术语: " + strings.TrimSpace(term.Term),
	}
	aliases := aliasesFromJSON(term.AliasesJSON)
	if len(aliases) > 0 {
		parts = append(parts, "别名: "+strings.Join(aliases, ", "))
	}
	parts = append(parts, "定义: "+strings.TrimSpace(term.Definition))
	return model.KBChunk{
		TenantID:  term.TenantID,
		ProjectID: term.ProjectID,
		KBType:    KBTypeTerm,
		RefID:     term.ID,
		ChunkNo:   0,
		ChunkText: strings.Join(nonEmptyLines(parts), "\n"),
	}
}

func chunkFromMetric(metric model.KBMetric) model.KBChunk {
	parts := []string{
		"指标: " + strings.TrimSpace(metric.Name),
		"描述: " + strings.TrimSpace(metric.Description),
		"计算口径: " + strings.TrimSpace(metric.Formula),
	}
	if metric.DefaultTimeColumn != "" {
		parts = append(parts, "默认时间字段: "+strings.TrimSpace(metric.DefaultTimeColumn))
	}
	return model.KBChunk{
		TenantID:     metric.TenantID,
		ProjectID:    metric.ProjectID,
		DatasourceID: metric.DatasourceID,
		KBType:       KBTypeMetric,
		RefID:        metric.ID,
		ChunkNo:      0,
		ChunkText:    strings.Join(nonEmptyLines(parts), "\n"),
	}
}

func chunkFromFewShot(fewShot model.KBFewShotSQL) model.KBChunk {
	parts := []string{
		"问题: " + strings.TrimSpace(fewShot.Question),
		"SQL: " + strings.TrimSpace(fewShot.SQLText),
	}
	if fewShot.Explanation != "" {
		parts = append(parts, "解释: "+strings.TrimSpace(fewShot.Explanation))
	}
	return model.KBChunk{
		TenantID:     fewShot.TenantID,
		ProjectID:    fewShot.ProjectID,
		DatasourceID: fewShot.DatasourceID,
		KBType:       KBTypeFewShot,
		RefID:        fewShot.ID,
		ChunkNo:      0,
		ChunkText:    strings.Join(nonEmptyLines(parts), "\n"),
	}
}

func nonEmptyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func deterministicVectorID(values ...any) int64 {
	h := fnv.New64a()
	for _, value := range values {
		_, _ = h.Write([]byte(fmt.Sprint(value)))
		_, _ = h.Write([]byte{0})
	}
	return int64(h.Sum64() & math.MaxInt64)
}
