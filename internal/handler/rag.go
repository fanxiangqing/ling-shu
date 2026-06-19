package handler

import (
	"errors"
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type RAGHandler struct {
	ragService *service.RAGService
}

type rebuildRAGRequest struct {
	TenantID     uint64 `json:"tenant_id" binding:"required"`
	DatasourceID uint64 `json:"datasource_id"`
	Limit        int    `json:"limit"`
}

type searchRAGRequest struct {
	TenantID     uint64 `json:"tenant_id" binding:"required"`
	DatasourceID uint64 `json:"datasource_id"`
	Question     string `json:"question" binding:"required"`
	Limit        int    `json:"limit"`
}

func NewRAGHandler(ragService *service.RAGService) *RAGHandler {
	return &RAGHandler{ragService: ragService}
}

func (h *RAGHandler) Rebuild(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req rebuildRAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	result, err := h.ragService.Rebuild(c.Request.Context(), service.RebuildRAGInput{
		TenantID:     req.TenantID,
		ProjectID:    projectID,
		DatasourceID: req.DatasourceID,
		Limit:        req.Limit,
	})
	if err != nil {
		writeRAGError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *RAGHandler) Search(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req searchRAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	result, err := h.ragService.Search(c.Request.Context(), service.SearchRAGInput{
		TenantID:     req.TenantID,
		ProjectID:    projectID,
		DatasourceID: req.DatasourceID,
		Question:     req.Question,
		Limit:        req.Limit,
	})
	if err != nil {
		writeRAGError(c, err)
		return
	}
	response.Success(c, result)
}

func writeRAGError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrProviderNotConfigured) {
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "rag provider is not configured")
		return
	}
	writeError(c, err)
}
