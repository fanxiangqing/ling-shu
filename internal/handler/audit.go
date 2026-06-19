package handler

import (
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	auditService *service.AuditService
	queryService *service.QueryService
}

func NewAuditHandler(auditService *service.AuditService, queryService *service.QueryService) *AuditHandler {
	return &AuditHandler{
		auditService: auditService,
		queryService: queryService,
	}
}

func (h *AuditHandler) ListLogs(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.auditService.ListLogs(c.Request.Context(), service.ListAuditLogsInput{
		TenantID:     parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID:    parseUint64Default(c.Query("project_id"), 0),
		UserID:       resolveUserID(c, parseUint64Default(c.Query("user_id"), 0)),
		EventType:    c.Query("event_type"),
		ResourceType: c.Query("resource_type"),
		ResourceID:   parseUint64Default(c.Query("resource_id"), 0),
		StartTime:    parseOptionalTime(c.Query("start_time")),
		EndTime:      parseOptionalTime(c.Query("end_time")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AuditHandler) QueryExecutions(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.queryService.History(c.Request.Context(), service.QueryHistoryInput{
		TenantID:     parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID:    parseUint64Default(c.Query("project_id"), 0),
		UserID:       resolveUserID(c, parseUint64Default(c.Query("user_id"), 0)),
		DatasourceID: parseUint64Default(c.Query("datasource_id"), 0),
		Status:       c.Query("status"),
		StartTime:    parseOptionalTime(c.Query("start_time")),
		EndTime:      parseOptionalTime(c.Query("end_time")),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, result)
}
