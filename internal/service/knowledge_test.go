package service

import (
	"context"
	"errors"
	"testing"

	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestKnowledgeServiceCreateTerm(t *testing.T) {
	repo := &knowledgeFakeRepository{}
	service := NewKnowledgeService(repo)

	term, err := service.CreateTerm(context.Background(), CreateKBTermInput{
		TenantID:   1,
		ProjectID:  2,
		Term:       "GMV",
		Aliases:    []string{"成交额", " 商品交易总额 "},
		Definition: "订单支付金额总和",
	})
	if err != nil {
		t.Fatalf("create term: %v", err)
	}
	if term.AliasesJSON == nil || *term.AliasesJSON != `["成交额","商品交易总额"]` {
		t.Fatalf("unexpected aliases json: %+v", term.AliasesJSON)
	}
	if !term.Enabled {
		t.Fatal("expected term enabled by default")
	}
	if len(repo.terms) != 1 {
		t.Fatalf("expected one term, got %d", len(repo.terms))
	}
}

func TestKnowledgeServiceCreateFewShotValidatesSQL(t *testing.T) {
	service := NewKnowledgeService(&knowledgeFakeRepository{})

	_, err := service.CreateFewShot(context.Background(), CreateKBFewShotInput{
		TenantID:  1,
		ProjectID: 2,
		Question:  "订单数",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestKnowledgeServiceListMetricsUsesDatasourceFilter(t *testing.T) {
	repo := &knowledgeFakeRepository{
		metrics: []model.KBMetric{{Name: "销售额", DatasourceID: 7}},
	}
	service := NewKnowledgeService(repo)
	enabled := true

	result, err := service.ListMetrics(context.Background(), ListKnowledgeInput{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 7,
		Enabled:      &enabled,
	})
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if result.Total != 1 || repo.lastFilter.DatasourceID != 7 || repo.lastFilter.Enabled == nil || !*repo.lastFilter.Enabled {
		t.Fatalf("unexpected result/filter: result=%+v filter=%+v", result, repo.lastFilter)
	}
}

func TestKnowledgeServiceManageTermLifecycle(t *testing.T) {
	repo := &knowledgeFakeRepository{
		terms: []model.KBTerm{{BaseModel: model.BaseModel{ID: 9}, TenantID: 1, ProjectID: 2, Term: "GMV", Enabled: true}},
	}
	service := NewKnowledgeService(repo)

	if err := service.UpdateTermEnabled(context.Background(), UpdateKnowledgeEnabledInput{
		KnowledgeItemInput: KnowledgeItemInput{TenantID: 1, ProjectID: 2, ID: 9},
		Enabled:            false,
	}); err != nil {
		t.Fatalf("update term enabled: %v", err)
	}
	if repo.lastScope.ID != 9 || repo.lastEnabled {
		t.Fatalf("unexpected update call: scope=%+v enabled=%v", repo.lastScope, repo.lastEnabled)
	}

	if err := service.DeleteTerm(context.Background(), KnowledgeItemInput{TenantID: 1, ProjectID: 2, ID: 9}); err != nil {
		t.Fatalf("delete term: %v", err)
	}
	if repo.lastDeleted != "term" || repo.lastScope.ID != 9 {
		t.Fatalf("unexpected delete call: type=%s scope=%+v", repo.lastDeleted, repo.lastScope)
	}
}

func TestKnowledgeServiceRefreshesIndexBestEffort(t *testing.T) {
	repo := &knowledgeFakeRepository{}
	refresher := &knowledgeFakeIndexRefresher{err: errors.New("rag down")}
	service := NewKnowledgeService(repo)
	service.SetIndexRefresher(refresher)

	term, err := service.CreateTerm(context.Background(), CreateKBTermInput{
		TenantID:   1,
		ProjectID:  2,
		Term:       "GMV",
		Definition: "订单支付金额总和",
	})
	if err != nil {
		t.Fatalf("create term should not fail when refresh fails: %v", err)
	}
	if term == nil || len(repo.terms) != 1 {
		t.Fatalf("expected term to be created, term=%+v count=%d", term, len(repo.terms))
	}
	if refresher.calls != 1 || refresher.tenantID != 1 || refresher.projectID != 2 || refresher.datasourceID != 0 {
		t.Fatalf("unexpected refresh call: %+v", refresher)
	}
}

type knowledgeFakeRepository struct {
	terms       []model.KBTerm
	metrics     []model.KBMetric
	fewShots    []model.KBFewShotSQL
	lastFilter  repository.KnowledgeFilter
	lastScope   repository.KnowledgeItemScope
	lastEnabled bool
	lastDeleted string
}

func (r *knowledgeFakeRepository) CreateTerm(ctx context.Context, term *model.KBTerm) error {
	term.ID = uint64(len(r.terms) + 1)
	r.terms = append(r.terms, *term)
	return nil
}

func (r *knowledgeFakeRepository) ListTerms(ctx context.Context, filter repository.KnowledgeFilter, page repository.Page) ([]model.KBTerm, int64, error) {
	r.lastFilter = filter
	return r.terms, int64(len(r.terms)), nil
}

func (r *knowledgeFakeRepository) UpdateTermEnabled(ctx context.Context, scope repository.KnowledgeItemScope, enabled bool) error {
	r.lastScope = scope
	r.lastEnabled = enabled
	return nil
}

func (r *knowledgeFakeRepository) DeleteTerm(ctx context.Context, scope repository.KnowledgeItemScope) error {
	r.lastScope = scope
	r.lastDeleted = "term"
	return nil
}

func (r *knowledgeFakeRepository) CreateMetric(ctx context.Context, metric *model.KBMetric) error {
	metric.ID = uint64(len(r.metrics) + 1)
	r.metrics = append(r.metrics, *metric)
	return nil
}

func (r *knowledgeFakeRepository) ListMetrics(ctx context.Context, filter repository.KnowledgeFilter, page repository.Page) ([]model.KBMetric, int64, error) {
	r.lastFilter = filter
	return r.metrics, int64(len(r.metrics)), nil
}

func (r *knowledgeFakeRepository) UpdateMetricEnabled(ctx context.Context, scope repository.KnowledgeItemScope, enabled bool) error {
	r.lastScope = scope
	r.lastEnabled = enabled
	return nil
}

func (r *knowledgeFakeRepository) DeleteMetric(ctx context.Context, scope repository.KnowledgeItemScope) error {
	r.lastScope = scope
	r.lastDeleted = "metric"
	return nil
}

func (r *knowledgeFakeRepository) CreateFewShot(ctx context.Context, fewShot *model.KBFewShotSQL) error {
	fewShot.ID = uint64(len(r.fewShots) + 1)
	r.fewShots = append(r.fewShots, *fewShot)
	return nil
}

func (r *knowledgeFakeRepository) ListFewShots(ctx context.Context, filter repository.KnowledgeFilter, page repository.Page) ([]model.KBFewShotSQL, int64, error) {
	r.lastFilter = filter
	return r.fewShots, int64(len(r.fewShots)), nil
}

func (r *knowledgeFakeRepository) UpdateFewShotEnabled(ctx context.Context, scope repository.KnowledgeItemScope, enabled bool) error {
	r.lastScope = scope
	r.lastEnabled = enabled
	return nil
}

func (r *knowledgeFakeRepository) DeleteFewShot(ctx context.Context, scope repository.KnowledgeItemScope) error {
	r.lastScope = scope
	r.lastDeleted = "fewshot"
	return nil
}

type knowledgeFakeIndexRefresher struct {
	calls        int
	tenantID     uint64
	projectID    uint64
	datasourceID uint64
	err          error
}

func (r *knowledgeFakeIndexRefresher) RefreshKnowledgeIndex(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) error {
	r.calls++
	r.tenantID = tenantID
	r.projectID = projectID
	r.datasourceID = datasourceID
	return r.err
}
