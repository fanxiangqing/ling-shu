package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	permissionService *service.PermissionService
}

type bindRoleRequest struct {
	UserID    uint64 `json:"user_id" binding:"required"`
	RoleCode  string `json:"role_code" binding:"required"`
	TenantID  uint64 `json:"tenant_id"`
	ProjectID uint64 `json:"project_id"`
	CreatedBy uint64 `json:"created_by"`
}

type checkPermissionRequest struct {
	UserID    uint64 `json:"user_id"`
	TenantID  uint64 `json:"tenant_id"`
	ProjectID uint64 `json:"project_id"`
	Code      string `json:"code"`
	Resource  string `json:"resource"`
	Action    string `json:"action"`
}

func NewPermissionHandler(permissionService *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{permissionService: permissionService}
}

func (h *PermissionHandler) ListRoles(c *gin.Context) {
	roles, err := h.permissionService.ListRoles(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, roles)
}

func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	permissions, err := h.permissionService.ListPermissions(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, permissions)
}

func (h *PermissionHandler) BindRole(c *gin.Context) {
	var req bindRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	binding, err := h.permissionService.BindRole(c.Request.Context(), service.BindRoleInput{
		UserID:    req.UserID,
		RoleCode:  req.RoleCode,
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		CreatedBy: resolveUserID(c, req.CreatedBy),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, binding)
}

func (h *PermissionHandler) ListRoleBindings(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.permissionService.ListRoleBindings(c.Request.Context(), service.ListRoleBindingsInput{
		UserID:    resolveUserID(c, parseUint64Default(c.Query("user_id"), 0)),
		TenantID:  parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID: parseUint64Default(c.Query("project_id"), 0),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PermissionHandler) Check(c *gin.Context) {
	var req checkPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.permissionService.Check(c.Request.Context(), service.CheckPermissionInput{
		UserID:    resolveUserID(c, req.UserID),
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Code:      req.Code,
		Resource:  req.Resource,
		Action:    req.Action,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}
