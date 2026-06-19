package handler

import (
	"errors"
	"net/http"

	"ling-shu/internal/asr"
	"ling-shu/internal/llm"
	"ling-shu/internal/service"
	"ling-shu/internal/tts"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type ProviderHandler struct {
	providerService *service.ProviderService
}

type providerChatRequest struct {
	TenantID    uint64        `json:"tenant_id"`
	ProjectID   uint64        `json:"project_id"`
	Model       string        `json:"model"`
	Messages    []llm.Message `json:"messages" binding:"required"`
	Temperature *float64      `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type providerTranscribeRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	ProjectID uint64 `json:"project_id"`
	Model     string `json:"model"`
	AudioURL  string `json:"audio_url" binding:"required"`
	Language  string `json:"language"`
}

type providerSynthesizeRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	ProjectID uint64 `json:"project_id"`
	Model     string `json:"model"`
	Text      string `json:"text" binding:"required"`
	Voice     string `json:"voice"`
	Format    string `json:"format"`
}

func NewProviderHandler(providerService *service.ProviderService) *ProviderHandler {
	return &ProviderHandler{providerService: providerService}
}

func (h *ProviderHandler) Summary(c *gin.Context) {
	response.Success(c, h.providerService.SummaryWithScope(c.Request.Context(), service.ProviderScopeInput{
		TenantID:  parseUint64Default(c.Query("tenant_id"), 0),
		ProjectID: parseUint64Default(c.Query("project_id"), 0),
	}))
}

func (h *ProviderHandler) Chat(c *gin.Context) {
	var req providerChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	result, err := h.providerService.Chat(c.Request.Context(), service.ProviderChatInput{
		TenantID:    req.TenantID,
		ProjectID:   req.ProjectID,
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		if errors.Is(err, service.ErrProviderNotConfigured) {
			response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "provider is not configured")
			return
		}
		writeError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *ProviderHandler) StreamChat(c *gin.Context) {
	var req providerChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	writeSSEHeaders(c)
	err := h.providerService.StreamChat(c.Request.Context(), service.ProviderChatInput{
		TenantID:    req.TenantID,
		ProjectID:   req.ProjectID,
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}, func(event llm.ChatStreamEvent) error {
		return writeSSEvent(c, "delta", event)
	})
	if err != nil {
		_ = writeSSEvent(c, "error", streamError{Message: providerErrorMessage(err)})
	}
}

func (h *ProviderHandler) Transcribe(c *gin.Context) {
	var req providerTranscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	result, err := h.providerService.Transcribe(c.Request.Context(), service.ProviderTranscribeInput{
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Model:     req.Model,
		AudioURL:  req.AudioURL,
		Language:  req.Language,
	})
	if err != nil {
		writeProviderError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *ProviderHandler) StreamTranscribe(c *gin.Context) {
	var req providerTranscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	writeSSEHeaders(c)
	err := h.providerService.StreamTranscribe(c.Request.Context(), service.ProviderTranscribeInput{
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Model:     req.Model,
		AudioURL:  req.AudioURL,
		Language:  req.Language,
	}, func(event asr.TranscribeStreamEvent) error {
		return writeSSEvent(c, "transcript", event)
	})
	if err != nil {
		_ = writeSSEvent(c, "error", streamError{Message: providerErrorMessage(err)})
	}
}

func (h *ProviderHandler) GetTranscribeTask(c *gin.Context) {
	result, err := h.providerService.GetTranscribeTask(c.Request.Context(), c.Param("task_id"))
	if err != nil {
		writeProviderError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *ProviderHandler) Synthesize(c *gin.Context) {
	var req providerSynthesizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	result, err := h.providerService.Synthesize(c.Request.Context(), service.ProviderSynthesizeInput{
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Model:     req.Model,
		Text:      req.Text,
		Voice:     req.Voice,
		Format:    req.Format,
	})
	if err != nil {
		writeProviderError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *ProviderHandler) StreamSynthesize(c *gin.Context) {
	var req providerSynthesizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	writeSSEHeaders(c)
	err := h.providerService.StreamSynthesize(c.Request.Context(), service.ProviderSynthesizeInput{
		TenantID:  req.TenantID,
		ProjectID: req.ProjectID,
		Model:     req.Model,
		Text:      req.Text,
		Voice:     req.Voice,
		Format:    req.Format,
	}, func(event tts.SynthesizeStreamEvent) error {
		return writeSSEvent(c, "audio", event)
	})
	if err != nil {
		_ = writeSSEvent(c, "error", streamError{Message: providerErrorMessage(err)})
	}
}

func writeProviderError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrProviderNotConfigured) {
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "provider is not configured")
		return
	}
	writeError(c, err)
}

type streamError struct {
	Message string `json:"message"`
}

func providerErrorMessage(err error) string {
	if errors.Is(err, service.ErrProviderNotConfigured) {
		return response.FriendlyMessage("service is not configured")
	}
	if errors.Is(err, service.ErrInvalidInput) {
		return response.FriendlyMessage("invalid input")
	}
	return response.FriendlyMessage(sanitizeUserFacingError(err, "service call failed"))
}
