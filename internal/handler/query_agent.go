package handler

import (
	"errors"
	"net/http"
	"strings"

	"ling-shu/internal/query"
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type QueryAgentHandler struct {
	queryAgentService *service.QueryAgentService
}

type askRequest struct {
	TenantID              uint64                  `json:"tenant_id"`
	ProjectID             uint64                  `json:"project_id" binding:"required"`
	DatasourceID          uint64                  `json:"datasource_id"`
	SelectedDatasourceIDs []uint64                `json:"selected_datasource_ids"`
	UserID                uint64                  `json:"user_id"`
	Question              string                  `json:"question" binding:"required"`
	MaxRows               int                     `json:"max_rows"`
	Datasources           []query.AgentDatasource `json:"datasources"`
	BusinessTerms         []query.AgentKnowledge  `json:"business_terms"`
	Metrics               []query.AgentKnowledge  `json:"metrics"`
	FewShots              []query.AgentFewShot    `json:"few_shots"`
	Conversation          []query.AgentMessage    `json:"conversation"`
	Permission            query.AgentPermission   `json:"permission"`
}

func NewQueryAgentHandler(queryAgentService *service.QueryAgentService) *QueryAgentHandler {
	return &QueryAgentHandler{queryAgentService: queryAgentService}
}

func (h *QueryAgentHandler) Ask(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	result, err := h.queryAgentService.Ask(c.Request.Context(), service.AskInput{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		UserID:                resolveUserID(c, req.UserID),
		Question:              req.Question,
		MaxRows:               req.MaxRows,
		Datasources:           req.Datasources,
		BusinessTerms:         req.BusinessTerms,
		Metrics:               req.Metrics,
		FewShots:              req.FewShots,
		Conversation:          req.Conversation,
		Permission:            req.Permission,
	})
	if err != nil {
		writeQueryAgentError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *QueryAgentHandler) StreamAsk(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	writeSSEHeaders(c)
	err := h.queryAgentService.StreamAsk(c.Request.Context(), service.AskInput{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		UserID:                resolveUserID(c, req.UserID),
		Question:              req.Question,
		MaxRows:               req.MaxRows,
		Datasources:           req.Datasources,
		BusinessTerms:         req.BusinessTerms,
		Metrics:               req.Metrics,
		FewShots:              req.FewShots,
		Conversation:          req.Conversation,
		Permission:            req.Permission,
	}, func(event query.AgentEvent) error {
		return writeSSEvent(c, event.Type, event)
	})
	if err != nil {
		_ = c.Error(err)
		_ = writeSSEvent(c, query.EventError, streamError{Message: queryAgentErrorMessage(err)})
	}
}

func writeQueryAgentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, query.ErrInvalidAgentInput):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid input")
	case errors.Is(err, query.ErrLLMNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "model service is not configured")
	default:
		writeError(c, err)
	}
}

func queryAgentErrorMessage(err error) string {
	if errors.Is(err, query.ErrInvalidAgentInput) {
		return response.FriendlyMessage("invalid input")
	}
	if errors.Is(err, query.ErrLLMNotConfigured) {
		return response.FriendlyMessage("model service is not configured")
	}
	return response.FriendlyMessage(sanitizeUserFacingError(err, "model service call failed"))
}

func sanitizeUserFacingError(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	message := err.Error()
	lower := strings.ToLower(message)
	if strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "context deadline") {
		return "模型响应时间较长，本次问数已中断。可以缩小问题范围、减少结果数量，或稍后重试。"
	}
	if strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "throttl") {
		return "模型服务当前繁忙，请稍后重试。"
	}
	if strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "invalid api key") ||
		strings.Contains(lower, "authentication") {
		return "模型服务认证失败，请检查项目的模型配置。"
	}
	if strings.Contains(lower, "aliyun") || strings.Contains(lower, "dashscope") || strings.Contains(lower, "alibaba") {
		return fallback
	}
	return message
}
