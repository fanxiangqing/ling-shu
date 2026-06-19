package rag

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	milvusclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

const (
	fieldID           = "id"
	fieldTenantID     = "tenant_id"
	fieldProjectID    = "project_id"
	fieldDatasourceID = "datasource_id"
	fieldKBType       = "kb_type"
	fieldRefID        = "ref_id"
	fieldChunkNo      = "chunk_no"
	fieldChunkText    = "chunk_text"
	fieldEmbedding    = "embedding"
)

const (
	milvusDropPartitionAttempts = 5
	milvusPartitionRetryDelay   = 200 * time.Millisecond
)

type MilvusConfig struct {
	Address    string
	Collection string
	Dimension  int
	TopK       int
	Timeout    time.Duration
}

type MilvusStore struct {
	client     milvusclient.Client
	collection string
	dimension  int
	topK       int
}

func NewMilvusStore(ctx context.Context, cfg MilvusConfig) (*MilvusStore, error) {
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, ErrInvalidRequest
	}
	if strings.TrimSpace(cfg.Collection) == "" {
		cfg.Collection = "ling_shu_kb_chunks"
	}
	if cfg.Dimension <= 0 {
		return nil, ErrInvalidRequest
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 8
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	connectCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client, err := milvusclient.NewGrpcClient(connectCtx, cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("connect milvus: %w", err)
	}
	store := &MilvusStore{
		client:     client,
		collection: cfg.Collection,
		dimension:  cfg.Dimension,
		topK:       cfg.TopK,
	}
	if err := store.EnsureCollection(connectCtx, cfg.Dimension); err != nil {
		_ = client.Close()
		return nil, err
	}
	return store, nil
}

func (s *MilvusStore) EnsureCollection(ctx context.Context, dimension int) error {
	if s == nil || s.client == nil {
		return ErrInvalidRequest
	}
	exists, err := s.client.HasCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("check milvus collection: %w", err)
	}
	if !exists {
		schema := entity.NewSchema().
			WithName(s.collection).
			WithDescription("Ling-Shu knowledge chunks").
			WithAutoID(false).
			WithField(entity.NewField().WithName(fieldID).WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true)).
			WithField(entity.NewField().WithName(fieldTenantID).WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName(fieldProjectID).WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName(fieldDatasourceID).WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName(fieldKBType).WithDataType(entity.FieldTypeVarChar).WithMaxLength(64)).
			WithField(entity.NewField().WithName(fieldRefID).WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName(fieldChunkNo).WithDataType(entity.FieldTypeInt64)).
			WithField(entity.NewField().WithName(fieldChunkText).WithDataType(entity.FieldTypeVarChar).WithMaxLength(4096)).
			WithField(entity.NewField().WithName(fieldEmbedding).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(dimension)))
		if err := s.client.CreateCollection(ctx, schema, 2); err != nil {
			return fmt.Errorf("create milvus collection: %w", err)
		}
		idx, err := entity.NewIndexHNSW(entity.IP, 16, 40)
		if err != nil {
			return fmt.Errorf("create milvus hnsw index: %w", err)
		}
		if err := s.client.CreateIndex(ctx, s.collection, fieldEmbedding, idx, false); err != nil {
			return fmt.Errorf("create milvus index: %w", err)
		}
	} else {
		collection, err := s.client.DescribeCollection(ctx, s.collection)
		if err != nil {
			return fmt.Errorf("describe milvus collection: %w", err)
		}
		actualDimension, err := collectionEmbeddingDimension(collection)
		if err != nil {
			return fmt.Errorf("inspect milvus collection schema: %w", err)
		}
		if actualDimension != dimension {
			return fmt.Errorf("milvus collection %q embedding dimension mismatch: configured=%d actual=%d; update rag.milvus.dimension to match the existing collection or recreate the collection and rebuild the RAG index", s.collection, dimension, actualDimension)
		}
	}
	s.dimension = dimension
	if err := s.loadCollection(ctx); err != nil {
		return err
	}
	return nil
}

func (s *MilvusStore) ReplaceByProject(ctx context.Context, tenantID uint64, projectID uint64, docs []VectorDocument) error {
	if s == nil || s.client == nil {
		return ErrInvalidRequest
	}
	if len(docs) > 0 {
		if err := s.validateDocumentDimensions(docs); err != nil {
			return err
		}
	}
	partition := projectPartitionName(tenantID, projectID)
	exists, err := s.client.HasPartition(ctx, s.collection, partition)
	if err != nil {
		return fmt.Errorf("check milvus partition: %w", err)
	}
	if exists {
		if err := s.dropProjectPartition(ctx, partition); err != nil {
			return err
		}
	}
	if len(docs) == 0 {
		return nil
	}
	if err := s.client.CreatePartition(ctx, s.collection, partition); err != nil {
		return fmt.Errorf("create milvus project partition: %w", err)
	}

	ids := make([]int64, 0, len(docs))
	tenantIDs := make([]int64, 0, len(docs))
	projectIDs := make([]int64, 0, len(docs))
	datasourceIDs := make([]int64, 0, len(docs))
	kbTypes := make([]string, 0, len(docs))
	refIDs := make([]int64, 0, len(docs))
	chunkNos := make([]int64, 0, len(docs))
	chunkTexts := make([]string, 0, len(docs))
	vectors := make([][]float32, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
		tenantIDs = append(tenantIDs, int64(doc.TenantID))
		projectIDs = append(projectIDs, int64(doc.ProjectID))
		datasourceIDs = append(datasourceIDs, int64(doc.DatasourceID))
		kbTypes = append(kbTypes, doc.KBType)
		refIDs = append(refIDs, int64(doc.RefID))
		chunkNos = append(chunkNos, int64(doc.ChunkNo))
		chunkTexts = append(chunkTexts, truncateMilvusText(doc.ChunkText))
		vectors = append(vectors, normalizeVector(doc.Vector))
	}
	_, err = s.client.Insert(ctx, s.collection, partition,
		entity.NewColumnInt64(fieldID, ids),
		entity.NewColumnInt64(fieldTenantID, tenantIDs),
		entity.NewColumnInt64(fieldProjectID, projectIDs),
		entity.NewColumnInt64(fieldDatasourceID, datasourceIDs),
		entity.NewColumnVarChar(fieldKBType, kbTypes),
		entity.NewColumnInt64(fieldRefID, refIDs),
		entity.NewColumnInt64(fieldChunkNo, chunkNos),
		entity.NewColumnVarChar(fieldChunkText, chunkTexts),
		entity.NewColumnFloatVector(fieldEmbedding, len(vectors[0]), vectors),
	)
	if err != nil {
		return fmt.Errorf("insert milvus chunks: %w", err)
	}
	if err := s.client.Flush(ctx, s.collection, false); err != nil {
		return fmt.Errorf("flush milvus chunks: %w", err)
	}
	if err := s.loadCollection(ctx); err != nil {
		return err
	}
	return nil
}

func (s *MilvusStore) Search(ctx context.Context, req VectorSearchRequest) ([]Hit, error) {
	if s == nil || s.client == nil {
		return nil, ErrInvalidRequest
	}
	if req.TenantID == 0 || req.ProjectID == 0 || len(req.Vector) == 0 {
		return nil, ErrInvalidRequest
	}
	if err := s.validateVectorDimension("search", len(req.Vector)); err != nil {
		return nil, err
	}
	topK := req.TopK
	if topK <= 0 {
		topK = s.topK
	}
	if topK <= 0 {
		topK = 8
	}

	partition := projectPartitionName(req.TenantID, req.ProjectID)
	exists, err := s.client.HasPartition(ctx, s.collection, partition)
	if err != nil {
		return nil, fmt.Errorf("check milvus partition: %w", err)
	}
	if !exists {
		return nil, nil
	}
	if err := s.loadCollection(ctx); err != nil {
		return nil, err
	}
	expr := fmt.Sprintf("%s == %d && %s == %d", fieldTenantID, req.TenantID, fieldProjectID, req.ProjectID)
	if req.DatasourceID > 0 {
		expr += fmt.Sprintf(" && (%s == 0 || %s == %d)", fieldDatasourceID, fieldDatasourceID, req.DatasourceID)
	}
	sp, err := entity.NewIndexHNSWSearchParam(64)
	if err != nil {
		return nil, fmt.Errorf("create milvus search param: %w", err)
	}
	results, err := s.client.Search(ctx, s.collection, []string{partition}, expr,
		[]string{fieldTenantID, fieldProjectID, fieldDatasourceID, fieldKBType, fieldRefID, fieldChunkText},
		[]entity.Vector{entity.FloatVector(normalizeVector(req.Vector))},
		fieldEmbedding,
		entity.IP,
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("search milvus chunks: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}
	return searchResultToHits(results[0])
}

func (s *MilvusStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *MilvusStore) dropProjectPartition(ctx context.Context, partition string) error {
	if err := s.releaseProjectPartitionForMutation(ctx, partition); err != nil {
		return fmt.Errorf("release milvus project partition: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < milvusDropPartitionAttempts; attempt++ {
		err := s.client.DropPartition(ctx, s.collection, partition)
		if err == nil || isMilvusPartitionNotFoundError(err) {
			return nil
		}
		lastErr = err
		if !isMilvusPartitionLoadedForDropError(err) {
			return fmt.Errorf("drop milvus project partition: %w", err)
		}
		if releaseErr := s.releaseProjectPartitionForMutation(ctx, partition); releaseErr != nil {
			lastErr = releaseErr
		}
		if err := sleepWithContext(ctx, milvusPartitionRetryDelay); err != nil {
			return err
		}
	}
	return fmt.Errorf("drop milvus project partition: %w", lastErr)
}

func (s *MilvusStore) loadCollection(ctx context.Context) error {
	err := s.client.LoadCollection(ctx, s.collection, false)
	if err == nil || isMilvusAlreadyLoadedError(err) {
		return nil
	}
	if isMilvusLoadModeMismatchError(err) {
		if releaseErr := s.releaseLoadedPartitions(ctx); releaseErr != nil {
			return fmt.Errorf("release milvus loaded partitions before loading collection: %w", releaseErr)
		}
		err = s.client.LoadCollection(ctx, s.collection, false)
		if err == nil || isMilvusAlreadyLoadedError(err) {
			return nil
		}
	}
	return fmt.Errorf("load milvus collection: %w", err)
}

func (s *MilvusStore) releaseProjectPartitionForMutation(ctx context.Context, partition string) error {
	if err := s.releaseMilvusCollection(ctx); err == nil {
		return nil
	} else if !isMilvusLoadModeMismatchError(err) {
		return err
	}
	return s.releaseProjectPartition(ctx, partition)
}

func (s *MilvusStore) releaseMilvusCollection(ctx context.Context) error {
	err := s.client.ReleaseCollection(ctx, s.collection)
	if err != nil && !isMilvusNotLoadedError(err) {
		return err
	}
	return s.waitAllPartitionsReleased(ctx)
}

func (s *MilvusStore) releaseLoadedPartitions(ctx context.Context) error {
	partitions, err := s.client.ShowPartitions(ctx, s.collection)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(partitions))
	for _, item := range partitions {
		if item != nil && item.Loaded {
			names = append(names, item.Name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	err = s.client.ReleasePartitions(ctx, s.collection, names)
	if err != nil && !isMilvusPartitionNotFoundError(err) && !isMilvusNotLoadedError(err) {
		return err
	}
	return s.waitAllPartitionsReleased(ctx)
}

func (s *MilvusStore) releaseProjectPartition(ctx context.Context, partition string) error {
	err := s.client.ReleasePartitions(ctx, s.collection, []string{partition})
	if err != nil && !isMilvusPartitionNotFoundError(err) && !isMilvusNotLoadedError(err) {
		return err
	}
	return s.waitProjectPartitionReleased(ctx, partition)
}

func (s *MilvusStore) waitAllPartitionsReleased(ctx context.Context) error {
	for attempt := 0; attempt < milvusDropPartitionAttempts; attempt++ {
		partitions, err := s.client.ShowPartitions(ctx, s.collection)
		if err != nil {
			return err
		}
		loaded := false
		for _, item := range partitions {
			if item != nil && item.Loaded {
				loaded = true
				break
			}
		}
		if !loaded {
			return nil
		}
		if err := sleepWithContext(ctx, milvusPartitionRetryDelay); err != nil {
			return err
		}
	}
	return nil
}

func (s *MilvusStore) waitProjectPartitionReleased(ctx context.Context, partition string) error {
	for attempt := 0; attempt < milvusDropPartitionAttempts; attempt++ {
		loaded, exists, err := s.projectPartitionLoaded(ctx, partition)
		if err != nil {
			return err
		}
		if !exists || !loaded {
			return nil
		}
		if err := sleepWithContext(ctx, milvusPartitionRetryDelay); err != nil {
			return err
		}
	}
	return nil
}

func (s *MilvusStore) projectPartitionLoaded(ctx context.Context, partition string) (bool, bool, error) {
	partitions, err := s.client.ShowPartitions(ctx, s.collection)
	if err != nil {
		return false, false, err
	}
	for _, item := range partitions {
		if item != nil && item.Name == partition {
			return item.Loaded, true, nil
		}
	}
	return false, false, nil
}

func searchResultToHits(result milvusclient.SearchResult) ([]Hit, error) {
	hits := make([]Hit, 0, result.ResultCount)
	for i := 0; i < result.ResultCount; i++ {
		id, err := result.IDs.GetAsInt64(i)
		if err != nil {
			return nil, err
		}
		hit := Hit{
			ID:    id,
			Score: result.Scores[i],
		}
		hit.TenantID = uint64(columnInt64(result.Fields.GetColumn(fieldTenantID), i))
		hit.ProjectID = uint64(columnInt64(result.Fields.GetColumn(fieldProjectID), i))
		hit.DatasourceID = uint64(columnInt64(result.Fields.GetColumn(fieldDatasourceID), i))
		hit.KBType = columnString(result.Fields.GetColumn(fieldKBType), i)
		hit.RefID = uint64(columnInt64(result.Fields.GetColumn(fieldRefID), i))
		hit.ChunkText = columnString(result.Fields.GetColumn(fieldChunkText), i)
		hits = append(hits, hit)
	}
	return hits, nil
}

func (s *MilvusStore) validateDocumentDimensions(docs []VectorDocument) error {
	for idx, doc := range docs {
		if err := s.validateVectorDimension("document", len(doc.Vector)); err != nil {
			return fmt.Errorf("%w at index %d", err, idx)
		}
	}
	return nil
}

func (s *MilvusStore) validateVectorDimension(operation string, actual int) error {
	if s.dimension <= 0 {
		return nil
	}
	if actual == s.dimension {
		return nil
	}
	return fmt.Errorf("milvus %s vector dimension mismatch: collection=%q expected=%d actual=%d; ensure providers.llm.embedding_model output dimension matches rag.milvus.dimension, then rebuild the RAG index", operation, s.collection, s.dimension, actual)
}

func collectionEmbeddingDimension(collection *entity.Collection) (int, error) {
	if collection == nil || collection.Schema == nil {
		return 0, fmt.Errorf("collection schema is empty")
	}
	for _, field := range collection.Schema.Fields {
		if field == nil || field.Name != fieldEmbedding {
			continue
		}
		if field.DataType != entity.FieldTypeFloatVector {
			return 0, fmt.Errorf("field %q is %s, expected float vector", fieldEmbedding, field.DataType.Name())
		}
		dimValue := strings.TrimSpace(field.TypeParams[entity.TypeParamDim])
		if dimValue == "" {
			return 0, fmt.Errorf("field %q missing dimension type param", fieldEmbedding)
		}
		dimension, err := strconv.Atoi(dimValue)
		if err != nil || dimension <= 0 {
			return 0, fmt.Errorf("field %q has invalid dimension %q", fieldEmbedding, dimValue)
		}
		return dimension, nil
	}
	return 0, fmt.Errorf("field %q not found", fieldEmbedding)
}

func isMilvusPartitionLoadedForDropError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "partition") && strings.Contains(lower, "loaded")
}

func isMilvusPartitionNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "partition") {
		return false
	}
	return strings.Contains(lower, "not found") ||
		strings.Contains(lower, "not exist") ||
		strings.Contains(lower, "does not exist") ||
		strings.Contains(lower, "not existed")
}

func isMilvusAlreadyLoadedError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	if isMilvusNotLoadedError(err) {
		return false
	}
	return strings.Contains(lower, "already loaded") ||
		strings.Contains(lower, "has been loaded") ||
		strings.Contains(lower, "loaded already")
}

func isMilvusNotLoadedError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "not loaded") ||
		strings.Contains(lower, "not been loaded") ||
		strings.Contains(lower, "has not been loaded")
}

func isMilvusLoadModeMismatchError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "loadparametermismatched") ||
		strings.Contains(lower, "after load collection") ||
		strings.Contains(lower, "after load partition") ||
		strings.Contains(lower, "after load partitions")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func columnInt64(column entity.Column, idx int) int64 {
	if column == nil {
		return 0
	}
	value, err := column.GetAsInt64(idx)
	if err != nil {
		return 0
	}
	return value
}

func columnString(column entity.Column, idx int) string {
	if column == nil {
		return ""
	}
	value, err := column.GetAsString(idx)
	if err != nil {
		return ""
	}
	return value
}

func normalizeVector(vector []float32) []float32 {
	if len(vector) == 0 {
		return vector
	}
	var sum float64
	for _, value := range vector {
		sum += float64(value * value)
	}
	norm := float32(math.Sqrt(sum))
	if norm == 0 {
		return vector
	}
	out := make([]float32, len(vector))
	for i, value := range vector {
		out[i] = value / norm
	}
	return out
}

func truncateMilvusText(value string) string {
	const maxRunes = 4096
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

func projectPartitionName(tenantID uint64, projectID uint64) string {
	return fmt.Sprintf("p_t%d_p%d", tenantID, projectID)
}
