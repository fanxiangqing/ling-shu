package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ling-shu/internal/middleware"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/internal/service"

	"github.com/gin-gonic/gin"
)

func TestAuditHandlerListLogsDoesNotDefaultToAuthenticatedUser(t *testing.T) {
	repo := &handlerAuditFakeRepository{}
	handler := NewAuditHandler(service.NewAuditService(repo), nil)

	c, w := newAuditTestContext("/audit/logs?tenant_id=1&project_id=2")
	c.Set(middleware.UserIDKey, uint64(9))
	handler.ListLogs(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.lastFilter.UserID != 0 {
		t.Fatalf("expected no user filter, got %d", repo.lastFilter.UserID)
	}
}

func TestAuditHandlerListLogsUsesExplicitUserFilter(t *testing.T) {
	repo := &handlerAuditFakeRepository{}
	handler := NewAuditHandler(service.NewAuditService(repo), nil)

	c, w := newAuditTestContext("/audit/logs?tenant_id=1&project_id=2&user_id=7")
	c.Set(middleware.UserIDKey, uint64(9))
	handler.ListLogs(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.lastFilter.UserID != 7 {
		t.Fatalf("expected explicit user filter 7, got %d", repo.lastFilter.UserID)
	}
}

func TestAuditHandlerQueryExecutionsDoesNotDefaultToAuthenticatedUser(t *testing.T) {
	repo := &handlerQueryFakeRepository{}
	handler := NewAuditHandler(nil, service.NewQueryService(nil, repo, nil, nil))

	c, w := newAuditTestContext("/audit/query-executions?tenant_id=1&project_id=2")
	c.Set(middleware.UserIDKey, uint64(9))
	handler.QueryExecutions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.lastFilter.UserID != 0 {
		t.Fatalf("expected no user filter, got %d", repo.lastFilter.UserID)
	}
}

func TestAuditHandlerQueryExecutionsUsesExplicitUserFilter(t *testing.T) {
	repo := &handlerQueryFakeRepository{}
	handler := NewAuditHandler(nil, service.NewQueryService(nil, repo, nil, nil))

	c, w := newAuditTestContext("/audit/query-executions?tenant_id=1&project_id=2&user_id=7")
	c.Set(middleware.UserIDKey, uint64(9))
	handler.QueryExecutions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if repo.lastFilter.UserID != 7 {
		t.Fatalf("expected explicit user filter 7, got %d", repo.lastFilter.UserID)
	}
}

func TestAuditHandlerQueryExecutionsIncludesEmbedAuditPayload(t *testing.T) {
	payload := `{"source":"embed","app_id":"emb_test","external_user_id":"u-1","external_user_name":"测试用户","session_key":"dashboard:123"}`
	auditRepo := &handlerAuditFakeRepository{
		logs: []model.AuditLog{
			{
				PayloadJSON: &payload,
			},
		},
	}
	queryRepo := &handlerQueryFakeRepository{
		executions: []model.QueryExecution{
			{
				TenantID:     1,
				ID:           10,
				ProjectID:    2,
				UserID:       3,
				DatasourceID: 4,
				Question:     "现在有多少用户？",
				Status:       "success",
			},
		},
	}
	handler := NewAuditHandler(service.NewAuditService(auditRepo), service.NewQueryService(nil, queryRepo, nil, nil))

	c, w := newAuditTestContext("/audit/query-executions?tenant_id=1&project_id=2")
	handler.QueryExecutions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"source":"embed"`) || !strings.Contains(w.Body.String(), `"app_id":"emb_test"`) || !strings.Contains(w.Body.String(), `"session_key":"dashboard:123"`) {
		t.Fatalf("expected embed audit payload in response, got %s", w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if auditRepo.lastFilter.ResourceID != 10 {
		t.Fatalf("expected audit lookup by execution id 10, got %+v", auditRepo.lastFilter)
	}
}

func newAuditTestContext(target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return c, w
}

type handlerAuditFakeRepository struct {
	logs       []model.AuditLog
	lastFilter repository.AuditLogFilter
}

func (r *handlerAuditFakeRepository) Create(ctx context.Context, log *model.AuditLog) error {
	return nil
}

func (r *handlerAuditFakeRepository) List(ctx context.Context, filter repository.AuditLogFilter, page repository.Page) ([]model.AuditLog, int64, error) {
	r.lastFilter = filter
	return r.logs, int64(len(r.logs)), nil
}

type handlerQueryFakeRepository struct {
	executions []model.QueryExecution
	lastFilter repository.QueryExecutionFilter
}

func (r *handlerQueryFakeRepository) CreateExecution(ctx context.Context, execution *model.QueryExecution) error {
	return nil
}

func (r *handlerQueryFakeRepository) FinishExecution(ctx context.Context, id uint64, updates repository.QueryExecutionFinish) error {
	return nil
}

func (r *handlerQueryFakeRepository) CreateReviewResult(ctx context.Context, result *model.SQLReviewResult) error {
	return nil
}

func (r *handlerQueryFakeRepository) ListExecutions(ctx context.Context, filter repository.QueryExecutionFilter, page repository.Page) ([]model.QueryExecution, int64, error) {
	r.lastFilter = filter
	return r.executions, int64(len(r.executions)), nil
}
