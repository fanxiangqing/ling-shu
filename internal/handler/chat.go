package handler

import (
	"errors"
	"net/http"

	"ling-shu/internal/query"
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	chatService *service.ChatService
}

type createChatSessionRequest struct {
	TenantID  uint64 `json:"tenant_id" binding:"required"`
	ProjectID uint64 `json:"project_id"`
	UserID    uint64 `json:"user_id"`
	Title     string `json:"title"`
}

type sendChatMessageRequest struct {
	TenantID              uint64                  `json:"tenant_id" binding:"required"`
	ProjectID             uint64                  `json:"project_id" binding:"required"`
	UserID                uint64                  `json:"user_id"`
	Content               string                  `json:"content" binding:"required"`
	DatasourceID          uint64                  `json:"datasource_id"`
	SelectedDatasourceIDs []uint64                `json:"selected_datasource_ids"`
	MaxRows               int                     `json:"max_rows"`
	AutoExecute           bool                    `json:"auto_execute"`
	Datasources           []query.AgentDatasource `json:"datasources"`
	BusinessTerms         []query.AgentKnowledge  `json:"business_terms"`
	Metrics               []query.AgentKnowledge  `json:"metrics"`
	FewShots              []query.AgentFewShot    `json:"few_shots"`
	Permission            query.AgentPermission   `json:"permission"`
}

func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

func (h *ChatHandler) CreateSession(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	tenantID := parseUint64Default(c.Param("tenant_id"), 0)
	var req createChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	if projectID == 0 {
		projectID = req.ProjectID
	}
	if tenantID == 0 {
		tenantID = req.TenantID
	}

	session, err := h.chatService.CreateSession(c.Request.Context(), service.CreateChatSessionInput{
		TenantID:  tenantID,
		ProjectID: projectID,
		UserID:    resolveUserID(c, req.UserID),
		Title:     req.Title,
	})
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, session)
}

func (h *ChatHandler) ListSessions(c *gin.Context) {
	page, pageSize := pageParams(c)
	tenantID := parseUint64Default(c.Param("tenant_id"), 0)
	if tenantID == 0 {
		tenantID = parseUint64Default(c.Query("tenant_id"), 0)
	}
	result, err := h.chatService.ListSessions(c.Request.Context(), service.ListChatSessionsInput{
		TenantID:  tenantID,
		ProjectID: parseUint64Default(c.Param("project_id"), parseUint64Default(c.Query("project_id"), 0)),
		UserID:    resolveUserID(c, parseUint64Default(c.Query("user_id"), 0)),
		Status:    c.Query("status"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ChatHandler) ListMessages(c *gin.Context) {
	page, pageSize := pageParams(c)
	result, err := h.chatService.ListMessages(c.Request.Context(), service.ListChatMessagesInput{
		TenantID:  parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID: parseUint64Default(c.Query("project_id"), 0),
		SessionID: parseUint64Default(c.Param("session_id"), 0),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	var req sendChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	meta := requestMetadata(c)

	result, err := h.chatService.SendMessage(c.Request.Context(), service.SendChatMessageInput{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		SessionID:             sessionID,
		UserID:                resolveUserID(c, req.UserID),
		Content:               req.Content,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		MaxRows:               req.MaxRows,
		AutoExecute:           req.AutoExecute,
		Datasources:           req.Datasources,
		BusinessTerms:         req.BusinessTerms,
		Metrics:               req.Metrics,
		FewShots:              req.FewShots,
		Permission:            req.Permission,
		RequestID:             meta.RequestID,
		IP:                    meta.IP,
		UserAgent:             meta.UserAgent,
	})
	if err != nil {
		writeChatError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ChatHandler) StreamMessage(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	var req sendChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	meta := requestMetadata(c)
	writeSSEHeaders(c)

	result, err := h.chatService.StreamMessage(c.Request.Context(), service.SendChatMessageInput{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		SessionID:             sessionID,
		UserID:                resolveUserID(c, req.UserID),
		Content:               req.Content,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		MaxRows:               req.MaxRows,
		AutoExecute:           req.AutoExecute,
		Datasources:           req.Datasources,
		BusinessTerms:         req.BusinessTerms,
		Metrics:               req.Metrics,
		FewShots:              req.FewShots,
		Permission:            req.Permission,
		RequestID:             meta.RequestID,
		IP:                    meta.IP,
		UserAgent:             meta.UserAgent,
	}, func(event query.AgentEvent) error {
		return writeSSEvent(c, event.Type, event)
	})
	if err != nil {
		_ = c.Error(err)
		_ = writeSSEvent(c, query.EventError, streamError{Message: queryAgentErrorMessage(err)})
		return
	}
	_ = writeSSEvent(c, "result", result)
}

func writeChatError(c *gin.Context, err error) {
	_ = c.Error(err)
	switch {
	case errors.Is(err, query.ErrInvalidAgentInput):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid input")
	case errors.Is(err, query.ErrLLMNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "model service is not configured")
	case errors.Is(err, query.ErrPromptNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "prompt renderer is not configured")
	default:
		writeError(c, err)
	}
}
