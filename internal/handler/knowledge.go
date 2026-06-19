package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type KnowledgeHandler struct {
	knowledgeService *service.KnowledgeService
}

type createKBTermRequest struct {
	TenantID   uint64   `json:"tenant_id" binding:"required"`
	Term       string   `json:"term" binding:"required"`
	Aliases    []string `json:"aliases"`
	Definition string   `json:"definition" binding:"required"`
	Enabled    *bool    `json:"enabled"`
	CreatedBy  uint64   `json:"created_by"`
}

type createKBMetricRequest struct {
	TenantID          uint64 `json:"tenant_id" binding:"required"`
	Name              string `json:"name" binding:"required"`
	Description       string `json:"description" binding:"required"`
	Formula           string `json:"formula" binding:"required"`
	DatasourceID      uint64 `json:"datasource_id"`
	DefaultTimeColumn string `json:"default_time_column"`
	Enabled           *bool  `json:"enabled"`
	CreatedBy         uint64 `json:"created_by"`
}

type createKBFewShotRequest struct {
	TenantID     uint64 `json:"tenant_id" binding:"required"`
	DatasourceID uint64 `json:"datasource_id"`
	Question     string `json:"question" binding:"required"`
	SQL          string `json:"sql" binding:"required"`
	Explanation  string `json:"explanation"`
	Enabled      *bool  `json:"enabled"`
	CreatedBy    uint64 `json:"created_by"`
}

type updateKnowledgeEnabledRequest struct {
	TenantID uint64 `json:"tenant_id" binding:"required"`
	Enabled  *bool  `json:"enabled" binding:"required"`
}

func NewKnowledgeHandler(knowledgeService *service.KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{knowledgeService: knowledgeService}
}

func (h *KnowledgeHandler) CreateTerm(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req createKBTermRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	term, err := h.knowledgeService.CreateTerm(c.Request.Context(), service.CreateKBTermInput{
		TenantID:   req.TenantID,
		ProjectID:  projectID,
		Term:       req.Term,
		Aliases:    req.Aliases,
		Definition: req.Definition,
		Enabled:    req.Enabled,
		CreatedBy:  resolveUserID(c, req.CreatedBy),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, term)
}

func (h *KnowledgeHandler) ListTerms(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.knowledgeService.ListTerms(c.Request.Context(), service.ListKnowledgeInput{
		TenantID:  parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID: parseUint64Default(c.Param("project_id"), 0),
		Enabled:   parseOptionalBool(c.Query("enabled")),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *KnowledgeHandler) UpdateTermEnabled(c *gin.Context) {
	var req updateKnowledgeEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := h.knowledgeService.UpdateTermEnabled(c.Request.Context(), service.UpdateKnowledgeEnabledInput{
		KnowledgeItemInput: knowledgeItemInput(c, req.TenantID),
		Enabled:            *req.Enabled,
	}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": *req.Enabled})
}

func (h *KnowledgeHandler) DeleteTerm(c *gin.Context) {
	if err := h.knowledgeService.DeleteTerm(c.Request.Context(), knowledgeItemInput(c, parseUint64Default(c.Query("tenant_id"), 0))); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *KnowledgeHandler) CreateMetric(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req createKBMetricRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	metric, err := h.knowledgeService.CreateMetric(c.Request.Context(), service.CreateKBMetricInput{
		TenantID:          req.TenantID,
		ProjectID:         projectID,
		Name:              req.Name,
		Description:       req.Description,
		Formula:           req.Formula,
		DatasourceID:      req.DatasourceID,
		DefaultTimeColumn: req.DefaultTimeColumn,
		Enabled:           req.Enabled,
		CreatedBy:         resolveUserID(c, req.CreatedBy),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, metric)
}

func (h *KnowledgeHandler) ListMetrics(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.knowledgeService.ListMetrics(c.Request.Context(), service.ListKnowledgeInput{
		TenantID:     parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID:    parseUint64Default(c.Param("project_id"), 0),
		DatasourceID: parseUint64Default(c.Query("datasource_id"), 0),
		Enabled:      parseOptionalBool(c.Query("enabled")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *KnowledgeHandler) UpdateMetricEnabled(c *gin.Context) {
	var req updateKnowledgeEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := h.knowledgeService.UpdateMetricEnabled(c.Request.Context(), service.UpdateKnowledgeEnabledInput{
		KnowledgeItemInput: knowledgeItemInput(c, req.TenantID),
		Enabled:            *req.Enabled,
	}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": *req.Enabled})
}

func (h *KnowledgeHandler) DeleteMetric(c *gin.Context) {
	if err := h.knowledgeService.DeleteMetric(c.Request.Context(), knowledgeItemInput(c, parseUint64Default(c.Query("tenant_id"), 0))); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *KnowledgeHandler) CreateFewShot(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req createKBFewShotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	fewShot, err := h.knowledgeService.CreateFewShot(c.Request.Context(), service.CreateKBFewShotInput{
		TenantID:     req.TenantID,
		ProjectID:    projectID,
		DatasourceID: req.DatasourceID,
		Question:     req.Question,
		SQL:          req.SQL,
		Explanation:  req.Explanation,
		Enabled:      req.Enabled,
		CreatedBy:    resolveUserID(c, req.CreatedBy),
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, fewShot)
}

func (h *KnowledgeHandler) ListFewShots(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.knowledgeService.ListFewShots(c.Request.Context(), service.ListKnowledgeInput{
		TenantID:     parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID:    parseUint64Default(c.Param("project_id"), 0),
		DatasourceID: parseUint64Default(c.Query("datasource_id"), 0),
		Enabled:      parseOptionalBool(c.Query("enabled")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *KnowledgeHandler) UpdateFewShotEnabled(c *gin.Context) {
	var req updateKnowledgeEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if err := h.knowledgeService.UpdateFewShotEnabled(c.Request.Context(), service.UpdateKnowledgeEnabledInput{
		KnowledgeItemInput: knowledgeItemInput(c, req.TenantID),
		Enabled:            *req.Enabled,
	}); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": *req.Enabled})
}

func (h *KnowledgeHandler) DeleteFewShot(c *gin.Context) {
	if err := h.knowledgeService.DeleteFewShot(c.Request.Context(), knowledgeItemInput(c, parseUint64Default(c.Query("tenant_id"), 0))); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func knowledgeItemInput(c *gin.Context, tenantID uint64) service.KnowledgeItemInput {
	return service.KnowledgeItemInput{
		TenantID:  tenantID,
		ProjectID: parseUint64Default(c.Param("project_id"), 0),
		ID:        parseUint64Default(c.Param("id"), 0),
	}
}
