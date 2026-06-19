package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ling-shu/internal/service"

	"github.com/gin-gonic/gin"
)

func TestRequirePermissionAllowsRequestAndRestoresBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &fakePermissionChecker{allowed: true}
	engine := gin.New()
	engine.POST("/projects/:project_id/query", func(c *gin.Context) {
		c.Set(UserIDKey, uint64(7))
	}, RequirePermission(checker, "query.execute", RequireProjectScope()), func(c *gin.Context) {
		var payload map[string]any
		if err := c.ShouldBindJSON(&payload); err != nil {
			t.Fatalf("bind json after middleware: %v", err)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/projects/2/query", strings.NewReader(`{"tenant_id":1,"sql":"select 1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if checker.last.UserID != 7 || checker.last.TenantID != 1 || checker.last.ProjectID != 2 || checker.last.Code != "query.execute" {
		t.Fatalf("unexpected permission input: %+v", checker.last)
	}
}

func TestRequirePermissionRejectsMissingProjectScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/query", func(c *gin.Context) {
		c.Set(UserIDKey, uint64(7))
	}, RequirePermission(&fakePermissionChecker{allowed: true}, "query.execute", RequireProjectScope()))

	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(`{"tenant_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestRequirePermissionRejectsDeniedPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/audit", func(c *gin.Context) {
		c.Set(UserIDKey, uint64(7))
	}, RequirePermission(&fakePermissionChecker{}, "audit.view", RequireTenantScope()))

	req := httptest.NewRequest(http.MethodGet, "/audit?tenant_id=1", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
}

func TestRequirePermissionUsesDatasourceScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &fakePermissionChecker{allowed: true}
	engine := gin.New()
	engine.POST("/datasources/:id/sync", func(c *gin.Context) {
		c.Set(UserIDKey, uint64(7))
	}, RequirePermission(
		checker,
		"metadata.sync",
		WithDatasourceScope(fakeDatasourceScopeResolver{tenantID: 1, projectID: 2}, "id"),
		RequireProjectScope(),
	), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/datasources/9/sync", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if checker.last.TenantID != 1 || checker.last.ProjectID != 2 || checker.last.Code != "metadata.sync" {
		t.Fatalf("unexpected permission input: %+v", checker.last)
	}
}

type fakePermissionChecker struct {
	allowed bool
	last    service.CheckPermissionInput
}

func (f *fakePermissionChecker) Check(ctx context.Context, input service.CheckPermissionInput) (*service.PermissionCheckResult, error) {
	f.last = input
	return &service.PermissionCheckResult{Allowed: f.allowed}, nil
}

type fakeDatasourceScopeResolver struct {
	tenantID  uint64
	projectID uint64
}

func (f fakeDatasourceScopeResolver) ResolveDatasourceScope(ctx context.Context, datasourceID uint64) (uint64, uint64, error) {
	return f.tenantID, f.projectID, nil
}
