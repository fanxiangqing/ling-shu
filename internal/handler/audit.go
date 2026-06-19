package handler

import (
	"encoding/json"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/model"
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
		UserID:       parseUint64Default(c.Query("user_id"), 0),
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

type auditQueryExecutionView struct {
	model.QueryExecution
	Source           string         `json:"source,omitempty"`
	AppID            string         `json:"app_id,omitempty"`
	ExternalUserID   string         `json:"external_user_id,omitempty"`
	ExternalUserName string         `json:"external_user_name,omitempty"`
	SessionKey       string         `json:"session_key,omitempty"`
	AuditPayload     map[string]any `json:"audit_payload,omitempty"`
}

func (h *AuditHandler) queryExecutionAuditViews(c *gin.Context, result service.PageResult[model.QueryExecution]) service.PageResult[auditQueryExecutionView] {
	views := make([]auditQueryExecutionView, 0, len(result.Items))
	for _, item := range result.Items {
		view := auditQueryExecutionView{QueryExecution: item}
		payload := h.queryExecutionAuditPayload(c, item)
		if len(payload) > 0 {
			view.AuditPayload = payload
			view.Source = stringFromMap(payload, "source")
			view.AppID = firstNonEmptyHandler(stringFromMap(payload, "app_id"), stringFromMap(payload, "embed_app_id"))
			view.ExternalUserID = stringFromMap(payload, "external_user_id")
			view.ExternalUserName = stringFromMap(payload, "external_user_name")
			view.SessionKey = stringFromMap(payload, "session_key")
		}
		views = append(views, view)
	}
	return service.PageResult[auditQueryExecutionView]{
		Items:    views,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	}
}

func (h *AuditHandler) queryExecutionAuditPayload(c *gin.Context, execution model.QueryExecution) map[string]any {
	if h.auditService == nil || execution.ID == 0 {
		return nil
	}
	result, err := h.auditService.ListLogs(c.Request.Context(), service.ListAuditLogsInput{
		TenantID:     execution.TenantID,
		ProjectID:    execution.ProjectID,
		EventType:    auditpkg.EventQueryExecute,
		ResourceType: auditpkg.ResourceQueryExecution,
		ResourceID:   execution.ID,
		Page:         1,
		PageSize:     1,
	})
	if err != nil || len(result.Items) == 0 || result.Items[0].PayloadJSON == nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(*result.Items[0].PayloadJSON), &payload); err != nil {
		return nil
	}
	return payload
}

func stringFromMap(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func (h *AuditHandler) QueryExecutions(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.queryService.History(c.Request.Context(), service.QueryHistoryInput{
		TenantID:     parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID:    parseUint64Default(c.Query("project_id"), 0),
		UserID:       parseUint64Default(c.Query("user_id"), 0),
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
	if h.auditService == nil {
		response.Success(c, result)
		return
	}
	response.Success(c, h.queryExecutionAuditViews(c, result))
}
