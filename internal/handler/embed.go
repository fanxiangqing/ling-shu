package handler

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"ling-shu/internal/query"
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type EmbedHandler struct {
	embedService *service.EmbedService
	chatService  *service.ChatService
	voiceService *service.VoiceService
}

type createEmbedAppRequest struct {
	TenantID       uint64   `json:"tenant_id" binding:"required"`
	Name           string   `json:"name"`
	AllowedOrigins []string `json:"allowed_origins"`
	SessionPolicy  string   `json:"session_policy"`
	LauncherTitle  string   `json:"launcher_title"`
	WelcomeMessage string   `json:"welcome_message"`
}

type createEmbedTokenRequest struct {
	AppID            string `json:"app_id" binding:"required"`
	AppSecret        string `json:"app_secret" binding:"required"`
	ExternalUserID   string `json:"external_user_id" binding:"required"`
	ExternalUserName string `json:"external_user_name"`
	TTLSeconds       int    `json:"ttl_seconds"`
}

type updateEmbedAppStatusRequest struct {
	TenantID uint64 `json:"tenant_id"`
	Status   string `json:"status" binding:"required"`
}

type bootstrapEmbedRequest struct {
	AppID        string `json:"app_id" binding:"required"`
	AccessToken  string `json:"access_token" binding:"required"`
	SessionKey   string `json:"key"`
	SessionMode  string `json:"session_mode"`
	ParentOrigin string `json:"parent_origin"`
}

type embedSendMessageRequest struct {
	AccessToken           string   `json:"access_token"`
	Content               string   `json:"content" binding:"required"`
	DatasourceID          uint64   `json:"datasource_id"`
	SelectedDatasourceIDs []uint64 `json:"selected_datasource_ids"`
	MaxRows               int      `json:"max_rows"`
	AutoExecute           bool     `json:"auto_execute"`
}

func NewEmbedHandler(embedService *service.EmbedService, chatService *service.ChatService, voiceService *service.VoiceService) *EmbedHandler {
	return &EmbedHandler{embedService: embedService, chatService: chatService, voiceService: voiceService}
}

func (h *EmbedHandler) CreateApp(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req createEmbedAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.embedService.CreateApp(c.Request.Context(), service.CreateEmbedAppInput{
		TenantID:       req.TenantID,
		ProjectID:      projectID,
		Name:           req.Name,
		AllowedOrigins: req.AllowedOrigins,
		SessionPolicy:  req.SessionPolicy,
		LauncherTitle:  req.LauncherTitle,
		WelcomeMessage: req.WelcomeMessage,
		CreatedBy:      authenticatedUserID(c),
	})
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) ListApps(c *gin.Context) {
	page, pageSize := pageParams(c)
	projectID := parseUint64Default(c.Param("project_id"), 0)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)
	result, err := h.embedService.ListApps(c.Request.Context(), tenantID, projectID, page, pageSize)
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) RevealAppSecret(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	appID := parseUint64Default(c.Param("app_id"), 0)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)
	result, err := h.embedService.RevealAppSecret(c.Request.Context(), tenantID, projectID, appID)
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) UpdateAppStatus(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	appID := parseUint64Default(c.Param("app_id"), 0)
	var req updateEmbedAppStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	tenantID := parseUint64Default(c.Query("tenant_id"), req.TenantID)
	result, err := h.embedService.UpdateAppStatus(c.Request.Context(), tenantID, projectID, appID, req.Status)
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) DeleteApp(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	appID := parseUint64Default(c.Param("app_id"), 0)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)
	if err := h.embedService.DeleteApp(c.Request.Context(), tenantID, projectID, appID); err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, gin.H{"status": "deleted"})
}

func (h *EmbedHandler) CreateToken(c *gin.Context) {
	var req createEmbedTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.embedService.CreateToken(c.Request.Context(), service.CreateEmbedTokenInput{
		AppID:            req.AppID,
		AppSecret:        req.AppSecret,
		ExternalUserID:   req.ExternalUserID,
		ExternalUserName: req.ExternalUserName,
		TTLSeconds:       req.TTLSeconds,
	})
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) Bootstrap(c *gin.Context) {
	var req bootstrapEmbedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	result, err := h.embedService.Bootstrap(c.Request.Context(), service.BootstrapEmbedInput{
		AppID:        req.AppID,
		AccessToken:  req.AccessToken,
		SessionKey:   req.SessionKey,
		SessionMode:  req.SessionMode,
		ParentOrigin: req.ParentOrigin,
	})
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) ListMessages(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	access, err := h.embedService.ValidateSessionAccess(c.Request.Context(), embedAccessToken(c), sessionID)
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	page, pageSize := pageParams(c)
	result, err := h.chatService.ListMessages(c.Request.Context(), service.ListChatMessagesInput{
		TenantID:  access.EmbedSession.TenantID,
		ProjectID: access.EmbedSession.ProjectID,
		SessionID: sessionID,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *EmbedHandler) StreamMessage(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	var req embedSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	accessToken := firstNonEmptyHandler(req.AccessToken, embedAccessToken(c))
	access, err := h.embedService.ValidateSessionAccess(c.Request.Context(), accessToken, sessionID)
	if err != nil {
		writeEmbedError(c, err)
		return
	}
	meta := requestMetadata(c)
	writeSSEHeaders(c)
	result, err := h.chatService.StreamMessage(c.Request.Context(), service.SendChatMessageInput{
		TenantID:              access.EmbedSession.TenantID,
		ProjectID:             access.EmbedSession.ProjectID,
		SessionID:             sessionID,
		UserID:                access.EmbedSession.UserID,
		Content:               req.Content,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		MaxRows:               req.MaxRows,
		AutoExecute:           req.AutoExecute,
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

func (h *EmbedHandler) RealtimeVoice(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	access, err := h.embedService.ValidateSessionAccess(c.Request.Context(), embedAccessToken(c), sessionID)
	if err != nil {
		writeEmbedError(c, err)
		return
	}

	language := c.Query("language")
	autoExecute := parseBoolDefault(c.Query("auto_execute"), true)
	maxRows := int(parseUint64Default(c.Query("max_rows"), 0))
	datasourceID := parseUint64Default(c.Query("datasource_id"), 0)
	selectedDatasourceIDs := parseUint64List(c.Query("selected_datasource_ids"))
	voice := c.Query("voice")
	format := c.Query("tts_format")
	if format == "" && !isRealtimeAudioInputFormat(c.Query("format")) {
		format = c.Query("format")
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		_ = c.Error(err)
		return
	}
	defer conn.Close()

	ctx, cancel := contextWithCancel(c)
	defer cancel()

	meta := requestMetadata(c)
	audio := make(chan []byte, 32)
	var (
		closeAudioOnce sync.Once
		writeMu        sync.Mutex
	)
	closeAudio := func() {
		closeAudioOnce.Do(func() {
			close(audio)
		})
	}
	writeJSON := func(message realtimeVoiceServerMessage) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(message)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer conn.Close()
		result, streamErr := h.voiceService.StreamRealtimeChat(ctx, service.RealtimeVoiceChatInput{
			TenantID:              access.EmbedSession.TenantID,
			ProjectID:             access.EmbedSession.ProjectID,
			SessionID:             sessionID,
			UserID:                access.EmbedSession.UserID,
			Language:              language,
			AutoExecute:           autoExecute,
			MaxRows:               maxRows,
			DatasourceID:          datasourceID,
			SelectedDatasourceIDs: selectedDatasourceIDs,
			Voice:                 voice,
			Format:                format,
			RequestID:             meta.RequestID,
			IP:                    meta.IP,
			UserAgent:             meta.UserAgent,
		}, audio, func(event service.VoiceChatStreamEvent) error {
			eventCopy := event
			return writeJSON(realtimeVoiceServerMessage{Type: event.Stage, Event: &eventCopy})
		})
		if streamErr != nil {
			_ = writeJSON(realtimeVoiceServerMessage{Type: "error", Message: providerErrorMessage(streamErr)})
			return
		}
		_ = writeJSON(realtimeVoiceServerMessage{Type: "result", Result: result})
	}()

	for {
		messageType, payload, readErr := conn.ReadMessage()
		if readErr != nil {
			closeAudio()
			if websocket.IsCloseError(readErr, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
				break
			}
			_ = c.Error(readErr)
			break
		}

		switch messageType {
		case websocket.BinaryMessage:
			chunk := append([]byte(nil), payload...)
			select {
			case audio <- chunk:
			case <-done:
				closeAudio()
				goto waitDone
			case <-ctx.Done():
				closeAudio()
				goto waitDone
			}
		case websocket.TextMessage:
			if shouldStopRealtimeVoice(payload) {
				closeAudio()
				goto waitDone
			}
		case websocket.CloseMessage:
			closeAudio()
			goto waitDone
		}
	}

waitDone:
	closeAudio()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func embedAccessToken(c *gin.Context) string {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "bearer ") {
		return strings.TrimSpace(raw[len("Bearer "):])
	}
	if strings.HasPrefix(lower, "embed ") {
		return strings.TrimSpace(raw[len("Embed "):])
	}
	return strings.TrimSpace(c.Query("embed_token"))
}

func writeEmbedError(c *gin.Context, err error) {
	_ = c.Error(err)
	switch {
	case errors.Is(err, service.ErrEmbedSecretInvalid):
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "App Secret 不正确，请在内嵌应用中重新查看 Secret 后再试")
	case errors.Is(err, service.ErrEmbedTokenInvalid):
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "内嵌 Token 已失效或不匹配，请重新签发 Token")
	case errors.Is(err, service.ErrEmbedOriginDenied):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "当前页面来源不在允许嵌入来源中，请检查内嵌应用的来源配置")
	case errors.Is(err, service.ErrEmbedAppDisabled):
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "内嵌应用已停用，请先启用后再使用")
	case errors.Is(err, service.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "内嵌凭证无效，请重新签发 Token")
	case errors.Is(err, service.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid input")
	case errors.Is(err, service.ErrSecretEncryptFailed):
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "App Secret 加密失败，请检查服务端密钥配置")
	case errors.Is(err, service.ErrSecretDecryptFailed):
		response.Error(c, http.StatusConflict, response.CodeConflict, "App Secret 无法查看，请确认服务端密钥配置一致；旧版本创建的应用可能需要重新创建")
	default:
		writeVoiceError(c, err)
	}
}

func firstNonEmptyHandler(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
