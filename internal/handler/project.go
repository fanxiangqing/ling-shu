package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type ProjectHandler struct {
	projectService *service.ProjectService
}

type createProjectRequest struct {
	TenantID      uint64   `json:"tenant_id" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Code          string   `json:"code" binding:"required"`
	Description   string   `json:"description"`
	DatasourceIDs []uint64 `json:"datasource_ids" binding:"required"`
}

func NewProjectHandler(projectService *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	project, err := h.projectService.Create(c.Request.Context(), service.CreateProjectInput{
		TenantID:      req.TenantID,
		Name:          req.Name,
		Code:          req.Code,
		Description:   req.Description,
		DatasourceIDs: req.DatasourceIDs,
		CreatedBy:     authenticatedUserID(c),
	})
	if err != nil {
		writeError(c, err)
		return
	}

	response.Success(c, project)
}

func (h *ProjectHandler) List(c *gin.Context) {
	page, pageSize := pageParams(c)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)

	result, err := h.projectService.List(c.Request.Context(), tenantID, authenticatedUserID(c), page, pageSize)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)
	if err := h.projectService.Delete(c.Request.Context(), service.DeleteProjectInput{
		TenantID:  tenantID,
		ProjectID: projectID,
	}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
