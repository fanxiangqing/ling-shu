package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type QueryHandler struct {
	queryService *service.QueryService
}

type reviewSQLRequest struct {
	TenantID     uint64 `json:"tenant_id" binding:"required"`
	ProjectID    uint64 `json:"project_id" binding:"required"`
	DatasourceID uint64 `json:"datasource_id"`
	UserID       uint64 `json:"user_id"`
	SQL          string `json:"sql" binding:"required"`
	MaxRows      int    `json:"max_rows"`
}

type executeSQLRequest struct {
	TenantID     uint64 `json:"tenant_id" binding:"required"`
	ProjectID    uint64 `json:"project_id" binding:"required"`
	DatasourceID uint64 `json:"datasource_id" binding:"required"`
	SessionID    uint64 `json:"session_id"`
	UserID       uint64 `json:"user_id"`
	Question     string `json:"question"`
	SQL          string `json:"sql" binding:"required"`
	MaxRows      int    `json:"max_rows"`
}

func NewQueryHandler(queryService *service.QueryService) *QueryHandler {
	return &QueryHandler{queryService: queryService}
}

func (h *QueryHandler) Review(c *gin.Context) {
	var req reviewSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	meta := requestMetadata(c)

	result, err := h.queryService.ReviewSQL(c.Request.Context(), service.ReviewSQLInput{
		TenantID:     req.TenantID,
		ProjectID:    req.ProjectID,
		DatasourceID: req.DatasourceID,
		UserID:       resolveUserID(c, req.UserID),
		SQL:          req.SQL,
		MaxRows:      req.MaxRows,
		RequestID:    meta.RequestID,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *QueryHandler) Execute(c *gin.Context) {
	var req executeSQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	meta := requestMetadata(c)

	result, err := h.queryService.ExecuteSQL(c.Request.Context(), service.ExecuteSQLInput{
		TenantID:     req.TenantID,
		ProjectID:    req.ProjectID,
		DatasourceID: req.DatasourceID,
		SessionID:    req.SessionID,
		UserID:       resolveUserID(c, req.UserID),
		Question:     req.Question,
		SQL:          req.SQL,
		MaxRows:      req.MaxRows,
		RequestID:    meta.RequestID,
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *QueryHandler) History(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.queryService.History(c.Request.Context(), service.QueryHistoryInput{
		TenantID:     parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID:    parseUint64Default(c.Query("project_id"), 0),
		UserID:       resolveUserID(c, parseUint64Default(c.Query("user_id"), 0)),
		DatasourceID: parseUint64Default(c.Query("datasource_id"), 0),
		Status:       c.Query("status"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}
