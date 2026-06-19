package rag

import (
	"errors"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

func TestCollectionEmbeddingDimension(t *testing.T) {
	collection := &entity.Collection{
		Schema: entity.NewSchema().
			WithField(entity.NewField().WithName(fieldID).WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true)).
			WithField(entity.NewField().WithName(fieldEmbedding).WithDataType(entity.FieldTypeFloatVector).WithDim(1024)),
	}

	dimension, err := collectionEmbeddingDimension(collection)
	if err != nil {
		t.Fatalf("collectionEmbeddingDimension: %v", err)
	}
	if dimension != 1024 {
		t.Fatalf("expected dimension 1024, got %d", dimension)
	}
}

func TestCollectionEmbeddingDimensionRequiresEmbeddingField(t *testing.T) {
	collection := &entity.Collection{
		Schema: entity.NewSchema().
			WithField(entity.NewField().WithName(fieldID).WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true)),
	}

	_, err := collectionEmbeddingDimension(collection)
	if err == nil || !strings.Contains(err.Error(), `field "embedding" not found`) {
		t.Fatalf("expected missing embedding field error, got %v", err)
	}
}

func TestMilvusStoreValidateDocumentDimensions(t *testing.T) {
	store := &MilvusStore{collection: "test_chunks", dimension: 3}

	err := store.validateDocumentDimensions([]VectorDocument{
		{Vector: []float32{1, 2, 3}},
		{Vector: []float32{1, 2}},
	})
	if err == nil {
		t.Fatal("expected vector dimension mismatch error")
	}
	if !strings.Contains(err.Error(), "expected=3 actual=2") || !strings.Contains(err.Error(), "index 1") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMilvusStoreValidateSearchDimension(t *testing.T) {
	store := &MilvusStore{collection: "test_chunks", dimension: 1024}

	err := store.validateVectorDimension("search", 1536)
	if err == nil {
		t.Fatal("expected search dimension mismatch error")
	}
	if !strings.Contains(err.Error(), "expected=1024 actual=1536") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMilvusPartitionErrorClassifiers(t *testing.T) {
	loadedErr := errors.New("partition cannot be dropped, partition is loaded, please release it first")
	if !isMilvusPartitionLoadedForDropError(loadedErr) {
		t.Fatalf("expected loaded partition error")
	}
	if isMilvusPartitionNotFoundError(loadedErr) {
		t.Fatalf("loaded partition error should not be treated as not found")
	}

	notFoundErr := errors.New("partition p_t1_p2 does not exist")
	if !isMilvusPartitionNotFoundError(notFoundErr) {
		t.Fatalf("expected partition not found error")
	}
	if isMilvusPartitionLoadedForDropError(notFoundErr) {
		t.Fatalf("not found partition error should not be treated as loaded")
	}
}

func TestMilvusAlreadyLoadedClassifierDoesNotMatchNotLoaded(t *testing.T) {
	if !isMilvusAlreadyLoadedError(errors.New("collection has been loaded")) {
		t.Fatalf("expected already loaded error")
	}
	if isMilvusAlreadyLoadedError(errors.New("partition has not been loaded to memory or load failed")) {
		t.Fatalf("not-loaded errors must not be treated as already-loaded")
	}
}

func TestMilvusLoadModeMismatchClassifier(t *testing.T) {
	err := errors.New("load the partition after load collection is not supported[LoadParameterMismatched]")
	if !isMilvusLoadModeMismatchError(err) {
		t.Fatalf("expected load mode mismatch error")
	}
	if isMilvusNotLoadedError(err) {
		t.Fatalf("load mode mismatch should not be treated as not loaded")
	}
}

func TestMilvusNotLoadedClassifier(t *testing.T) {
	err := errors.New("partition has not been loaded to memory or load failed")
	if !isMilvusNotLoadedError(err) {
		t.Fatalf("expected not-loaded error")
	}
	if isMilvusAlreadyLoadedError(err) {
		t.Fatalf("not-loaded error should not be treated as already loaded")
	}
}
