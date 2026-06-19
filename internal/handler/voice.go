package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"ling-shu/internal/query"
	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type VoiceHandler struct {
	voiceService *service.VoiceService
}

type voiceChatRequest struct {
	TenantID              uint64   `json:"tenant_id" binding:"required"`
	ProjectID             uint64   `json:"project_id" binding:"required"`
	UserID                uint64   `json:"user_id"`
	AudioURL              string   `json:"audio_url" binding:"required"`
	Language              string   `json:"language"`
	AutoExecute           bool     `json:"auto_execute"`
	MaxRows               int      `json:"max_rows"`
	DatasourceID          uint64   `json:"datasource_id"`
	SelectedDatasourceIDs []uint64 `json:"selected_datasource_ids"`
	Voice                 string   `json:"voice"`
	Format                string   `json:"format"`
}

type realtimeVoiceClientMessage struct {
	Type string `json:"type"`
}

type realtimeVoiceServerMessage struct {
	Type    string                        `json:"type"`
	Event   *service.VoiceChatStreamEvent `json:"event,omitempty"`
	Result  *service.VoiceChatResult      `json:"result,omitempty"`
	Message string                        `json:"message,omitempty"`
}

func NewVoiceHandler(voiceService *service.VoiceService) *VoiceHandler {
	return &VoiceHandler{voiceService: voiceService}
}

func (h *VoiceHandler) Chat(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	var req voiceChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	meta := requestMetadata(c)

	result, err := h.voiceService.Chat(c.Request.Context(), service.VoiceChatInput{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		SessionID:             sessionID,
		UserID:                resolveUserID(c, req.UserID),
		AudioURL:              req.AudioURL,
		Language:              req.Language,
		AutoExecute:           req.AutoExecute,
		MaxRows:               req.MaxRows,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		Voice:                 req.Voice,
		Format:                req.Format,
		RequestID:             meta.RequestID,
		IP:                    meta.IP,
		UserAgent:             meta.UserAgent,
	})
	if err != nil {
		writeVoiceError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *VoiceHandler) StreamChat(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	var req voiceChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}
	meta := requestMetadata(c)
	writeSSEHeaders(c)

	result, err := h.voiceService.StreamChat(c.Request.Context(), service.VoiceChatInput{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		SessionID:             sessionID,
		UserID:                resolveUserID(c, req.UserID),
		AudioURL:              req.AudioURL,
		Language:              req.Language,
		AutoExecute:           req.AutoExecute,
		MaxRows:               req.MaxRows,
		DatasourceID:          req.DatasourceID,
		SelectedDatasourceIDs: req.SelectedDatasourceIDs,
		Voice:                 req.Voice,
		Format:                req.Format,
		RequestID:             meta.RequestID,
		IP:                    meta.IP,
		UserAgent:             meta.UserAgent,
	}, func(event service.VoiceChatStreamEvent) error {
		return writeStreamEvent(c, event.Stage, event)
	})
	if err != nil {
		_ = c.Error(err)
		_ = writeStreamEvent(c, "error", streamError{Message: providerErrorMessage(err)})
		return
	}
	_ = writeStreamEvent(c, "result", result)
}

func (h *VoiceHandler) RealtimeChat(c *gin.Context) {
	sessionID := parseUint64Default(c.Param("session_id"), 0)
	tenantID := parseUint64Default(c.Query("tenant_id"), 0)
	projectID := parseUint64Default(c.Query("project_id"), 0)
	userID := resolveUserID(c, parseUint64Default(c.Query("user_id"), 0))
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
	if sessionID == 0 || tenantID == 0 || projectID == 0 || userID == 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request query")
		return
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
			TenantID:              tenantID,
			ProjectID:             projectID,
			SessionID:             sessionID,
			UserID:                userID,
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

func writeVoiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProviderNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "provider is not configured")
	case errors.Is(err, service.ErrProviderStreamingUnsupported):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "provider streaming audio is not supported")
	case errors.Is(err, query.ErrLLMNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "llm provider is not configured")
	case errors.Is(err, query.ErrInvalidAgentInput), errors.Is(err, service.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid input")
	default:
		writeChatError(c, err)
	}
}

func isRealtimeAudioInputFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "pcm", "wav":
		return true
	default:
		return false
	}
}

func contextWithCancel(c *gin.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(c.Request.Context())
}

func parseBoolDefault(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseUint64List(value string) []uint64 {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]uint64, 0, len(parts))
	for _, part := range parts {
		item := parseUint64Default(strings.TrimSpace(part), 0)
		if item > 0 {
			items = append(items, item)
		}
	}
	return items
}

func shouldStopRealtimeVoice(payload []byte) bool {
	content := strings.TrimSpace(string(payload))
	if content == "" {
		return false
	}
	if content == "stop" || content == "end" {
		return true
	}
	var message realtimeVoiceClientMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return false
	}
	action := strings.ToLower(strings.TrimSpace(message.Type))
	return action == "stop" || action == "end"
}
