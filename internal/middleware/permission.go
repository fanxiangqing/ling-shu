package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type PermissionChecker interface {
	Check(ctx context.Context, input service.CheckPermissionInput) (*service.PermissionCheckResult, error)
}

type DatasourceScopeResolver interface {
	ResolveDatasourceScope(ctx context.Context, datasourceID uint64) (tenantID uint64, projectID uint64, err error)
}

type PermissionOption func(*permissionOptions)

type permissionOptions struct {
	requireTenant       bool
	requireProject      bool
	datasourceResolver  DatasourceScopeResolver
	datasourceParamName string
}

func RequireTenantScope() PermissionOption {
	return func(opts *permissionOptions) {
		opts.requireTenant = true
	}
}

func RequireProjectScope() PermissionOption {
	return func(opts *permissionOptions) {
		opts.requireTenant = true
		opts.requireProject = true
	}
}

func WithDatasourceScope(resolver DatasourceScopeResolver, paramName string) PermissionOption {
	return func(opts *permissionOptions) {
		opts.datasourceResolver = resolver
		opts.datasourceParamName = strings.TrimSpace(paramName)
	}
}

func RequirePermission(checker PermissionChecker, code string, options ...PermissionOption) gin.HandlerFunc {
	opts := permissionOptions{}
	for _, option := range options {
		option(&opts)
	}
	return func(c *gin.Context) {
		if checker == nil || strings.TrimSpace(code) == "" {
			response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "permission checker is not configured")
			c.Abort()
			return
		}
		userID := currentUserID(c)
		if userID == 0 {
			response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "authentication is required")
			c.Abort()
			return
		}
		scope, err := permissionScope(c)
		if err != nil {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid permission scope")
			c.Abort()
			return
		}
		if opts.datasourceResolver != nil {
			datasourceID := parseUint(c.Param(opts.datasourceParamName))
			if datasourceID == 0 {
				response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "datasource id is required")
				c.Abort()
				return
			}
			tenantID, projectID, err := opts.datasourceResolver.ResolveDatasourceScope(c.Request.Context(), datasourceID)
			if err != nil || tenantID == 0 || (opts.requireProject && projectID == 0) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, "datasource scope not found")
				c.Abort()
				return
			}
			scope.tenantID = tenantID
			scope.projectID = projectID
		}
		if opts.requireTenant && scope.tenantID == 0 {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "tenant_id is required")
			c.Abort()
			return
		}
		if opts.requireProject && scope.projectID == 0 {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "project_id is required")
			c.Abort()
			return
		}
		result, err := checker.Check(c.Request.Context(), service.CheckPermissionInput{
			UserID:    userID,
			TenantID:  scope.tenantID,
			ProjectID: scope.projectID,
			Code:      code,
		})
		if err != nil {
			response.Error(c, http.StatusInternalServerError, response.CodeInternal, "check permission failed")
			c.Abort()
			return
		}
		if result == nil || !result.Allowed {
			response.Error(c, http.StatusForbidden, response.CodeForbidden, "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

type scopeValues struct {
	tenantID  uint64
	projectID uint64
}

func currentUserID(c *gin.Context) uint64 {
	value, ok := c.Get(UserIDKey)
	if !ok {
		return 0
	}
	userID, _ := value.(uint64)
	return userID
}

func permissionScope(c *gin.Context) (scopeValues, error) {
	scope := scopeValues{
		tenantID:  parseUint(c.Query("tenant_id")),
		projectID: parseUint(c.Query("project_id")),
	}
	if value := parseUint(c.Param("project_id")); value > 0 {
		scope.projectID = value
	}
	if value := parseUint(c.Param("tenant_id")); value > 0 {
		scope.tenantID = value
	}
	bodyScope, err := scopeFromJSONBody(c)
	if err != nil {
		return scopeValues{}, err
	}
	if bodyScope.tenantID > 0 {
		scope.tenantID = bodyScope.tenantID
	}
	if bodyScope.projectID > 0 {
		scope.projectID = bodyScope.projectID
	}
	return scope, nil
}

func scopeFromJSONBody(c *gin.Context) (scopeValues, error) {
	if c.Request == nil || c.Request.Body == nil || !strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		return scopeValues{}, nil
	}
	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return scopeValues{}, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(content))
	if len(bytes.TrimSpace(content)) == 0 {
		return scopeValues{}, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return scopeValues{}, err
	}
	return scopeValues{
		tenantID:  numberFromPayload(payload["tenant_id"]),
		projectID: numberFromPayload(payload["project_id"]),
	}, nil
}

func numberFromPayload(value any) uint64 {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return uint64(typed)
		}
	case string:
		return parseUint(typed)
	}
	return 0
}

func parseUint(value string) uint64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
