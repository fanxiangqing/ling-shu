package rag

import (
	"context"
	"errors"
	"testing"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestRetrieverRetrieve(t *testing.T) {
	repo := &fakeKnowledgeRepository{
		terms:    []model.KBTerm{{Term: "客单价", Definition: "平均订单金额"}, {Term: "GMV", Definition: "成交金额"}},
		metrics:  []model.KBMetric{{Name: "退款率", Description: "退款订单占比"}, {Name: "销售额", Description: "订单销售额", Formula: "sum(pay_amount)", DatasourceID: 7}},
		fewShots: []model.KBFewShotSQL{{Question: "今天销售额", SQLText: "select sum(pay_amount) from orders", DatasourceID: 7}},
	}
	retriever := NewRetriever(repo)

	context, err := retriever.Retrieve(context.Background(), Request{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 7,
		Question:     "今天 GMV 和销售额是多少",
	})
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if repo.lastFilter.DatasourceID != 7 || repo.lastFilter.Enabled == nil || !*repo.lastFilter.Enabled {
		t.Fatalf("unexpected filter: %+v", repo.lastFilter)
	}
	if len(context.BusinessTerms) != 2 || context.BusinessTerms[0].Name != "GMV" {
		t.Fatalf("unexpected terms: %+v", context.BusinessTerms)
	}
	if len(context.Metrics) != 2 || context.Metrics[0].Expression != "sum(pay_amount)" {
		t.Fatalf("unexpected metrics: %+v", context.Metrics)
	}
	if len(context.FewShots) != 1 || context.FewShots[0].DatasourceID != 7 {
		t.Fatalf("unexpected fewshots: %+v", context.FewShots)
	}
}

func TestRetrieverRejectsInvalidRequest(t *testing.T) {
	retriever := NewRetriever(&fakeKnowledgeRepository{})

	_, err := retriever.Retrieve(context.Background(), Request{TenantID: 1})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestRetrieverMergesVectorHits(t *testing.T) {
	store := &fakeVectorStore{
		hits: []Hit{
			{
				ID:           101,
				Score:        0.91,
				TenantID:     1,
				ProjectID:    2,
				DatasourceID: 7,
				KBType:       KBTypeMetric,
				RefID:        12,
				ChunkText:    "指标: 销售额\n描述: 订单销售额\n计算口径: sum(pay_amount)",
			},
			{
				ID:           102,
				Score:        0.88,
				TenantID:     1,
				ProjectID:    2,
				DatasourceID: 7,
				KBType:       KBTypeFewShot,
				RefID:        13,
				ChunkText:    "问题: 今天销售额\nSQL: select sum(pay_amount) from orders",
			},
		},
	}
	retriever := NewRetriever(
		&fakeKnowledgeRepository{},
		WithEmbedder(fakeEmbeddingProvider{configured: true}),
		WithVectorStore(store),
		WithTopK(2),
	)

	context, err := retriever.Retrieve(context.Background(), Request{
		TenantID:  1,
		ProjectID: 2,
		Question:  "今天销售额是多少",
	})
	if err != nil {
		t.Fatalf("retrieve: %v", err)
	}
	if len(context.Hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(context.Hits))
	}
	if len(context.Metrics) != 1 || context.Metrics[0].Name != "销售额" || context.Metrics[0].Expression != "sum(pay_amount)" {
		t.Fatalf("unexpected vector metrics: %+v", context.Metrics)
	}
	if len(context.FewShots) != 1 || context.FewShots[0].SQL == "" {
		t.Fatalf("unexpected vector fewshots: %+v", context.FewShots)
	}
}

type fakeKnowledgeRepository struct {
	terms      []model.KBTerm
	metrics    []model.KBMetric
	fewShots   []model.KBFewShotSQL
	lastFilter repository.KnowledgeFilter
}

func (r *fakeKnowledgeRepository) CreateTerm(ctx context.Context, term *model.KBTerm) error {
	return nil
}

func (r *fakeKnowledgeRepository) ListTerms(ctx context.Context, filter repository.KnowledgeFilter, page repository.Page) ([]model.KBTerm, int64, error) {
	r.lastFilter = filter
	return r.terms, int64(len(r.terms)), nil
}

func (r *fakeKnowledgeRepository) UpdateTermEnabled(ctx context.Context, scope repository.KnowledgeItemScope, enabled bool) error {
	return nil
}

func (r *fakeKnowledgeRepository) DeleteTerm(ctx context.Context, scope repository.KnowledgeItemScope) error {
	return nil
}

func (r *fakeKnowledgeRepository) CreateMetric(ctx context.Context, metric *model.KBMetric) error {
	return nil
}

func (r *fakeKnowledgeRepository) ListMetrics(ctx context.Context, filter repository.KnowledgeFilter, page repository.Page) ([]model.KBMetric, int64, error) {
	r.lastFilter = filter
	return r.metrics, int64(len(r.metrics)), nil
}

func (r *fakeKnowledgeRepository) UpdateMetricEnabled(ctx context.Context, scope repository.KnowledgeItemScope, enabled bool) error {
	return nil
}

func (r *fakeKnowledgeRepository) DeleteMetric(ctx context.Context, scope repository.KnowledgeItemScope) error {
	return nil
}

func (r *fakeKnowledgeRepository) CreateFewShot(ctx context.Context, fewShot *model.KBFewShotSQL) error {
	return nil
}

func (r *fakeKnowledgeRepository) ListFewShots(ctx context.Context, filter repository.KnowledgeFilter, page repository.Page) ([]model.KBFewShotSQL, int64, error) {
	r.lastFilter = filter
	return r.fewShots, int64(len(r.fewShots)), nil
}

func (r *fakeKnowledgeRepository) UpdateFewShotEnabled(ctx context.Context, scope repository.KnowledgeItemScope, enabled bool) error {
	return nil
}

func (r *fakeKnowledgeRepository) DeleteFewShot(ctx context.Context, scope repository.KnowledgeItemScope) error {
	return nil
}
