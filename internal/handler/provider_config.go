package handler

import (
	"net/http"

	"ling-shu/internal/service"
	"ling-shu/pkg/response"

	"github.com/gin-gonic/gin"
)

type ProviderConfigHandler struct {
	configService *service.ProviderConfigService
}

type upsertLLMConfigRequest struct {
	TenantID   uint64 `json:"tenant_id" binding:"required"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	APIBase    string `json:"api_base"`
	APIKey     string `json:"api_key"`
	ConfigJSON string `json:"config_json"`
	Enabled    *bool  `json:"enabled"`
}

type upsertASRConfigRequest struct {
	TenantID                       uint64 `json:"tenant_id" binding:"required"`
	Provider                       string `json:"provider"`
	Model                          string `json:"model"`
	AccessKeyID                    string `json:"access_key_id"`
	AccessKeySecret                string `json:"access_key_secret"`
	AppKey                         string `json:"app_key"`
	TokenEndpoint                  string `json:"token_endpoint"`
	TokenRegionID                  string `json:"token_region_id"`
	TokenRefreshBeforeSeconds      *int   `json:"token_refresh_before_seconds"`
	WebsocketURL                   string `json:"websocket_url"`
	Format                         string `json:"format"`
	SampleRate                     *int   `json:"sample_rate"`
	EnableIntermediateResult       *bool  `json:"enable_intermediate_result"`
	EnablePunctuationPrediction    *bool  `json:"enable_punctuation_prediction"`
	EnableInverseTextNormalization *bool  `json:"enable_inverse_text_normalization"`
	EnableWords                    *bool  `json:"enable_words"`
	TimeoutMS                      *int   `json:"timeout_ms"`
	ConfigJSON                     string `json:"config_json"`
	Enabled                        *bool  `json:"enabled"`
}

type upsertTTSConfigRequest struct {
	TenantID                  uint64 `json:"tenant_id" binding:"required"`
	Provider                  string `json:"provider"`
	Model                     string `json:"model"`
	Voice                     string `json:"voice"`
	AccessKeyID               string `json:"access_key_id"`
	AccessKeySecret           string `json:"access_key_secret"`
	AppKey                    string `json:"app_key"`
	TokenEndpoint             string `json:"token_endpoint"`
	TokenRegionID             string `json:"token_region_id"`
	TokenRefreshBeforeSeconds *int   `json:"token_refresh_before_seconds"`
	WebsocketURL              string `json:"websocket_url"`
	Format                    string `json:"format"`
	SampleRate                *int   `json:"sample_rate"`
	Volume                    *int   `json:"volume"`
	SpeechRate                *int   `json:"speech_rate"`
	PitchRate                 *int   `json:"pitch_rate"`
	EnableSubtitle            *bool  `json:"enable_subtitle"`
	TimeoutMS                 *int   `json:"timeout_ms"`
	ConfigJSON                string `json:"config_json"`
	Enabled                   *bool  `json:"enabled"`
}

func NewProviderConfigHandler(configService *service.ProviderConfigService) *ProviderConfigHandler {
	return &ProviderConfigHandler{configService: configService}
}

func (h *ProviderConfigHandler) GetLLM(c *gin.Context) {
	tenantID, projectID := providerConfigScope(c)
	config, err := h.configService.GetDefaultLLM(c.Request.Context(), tenantID, projectID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ProviderConfigHandler) UpsertLLM(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req upsertLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	config, err := h.configService.UpsertDefaultLLM(c.Request.Context(), service.UpsertLLMConfigInput{
		TenantID:   req.TenantID,
		ProjectID:  projectID,
		Provider:   req.Provider,
		Model:      req.Model,
		APIBase:    req.APIBase,
		APIKey:     req.APIKey,
		ConfigJSON: req.ConfigJSON,
		Enabled:    req.Enabled,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ProviderConfigHandler) GetASR(c *gin.Context) {
	tenantID, projectID := providerConfigScope(c)
	config, err := h.configService.GetDefaultASR(c.Request.Context(), tenantID, projectID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ProviderConfigHandler) UpsertASR(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req upsertASRConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	config, err := h.configService.UpsertDefaultASR(c.Request.Context(), service.UpsertASRConfigInput{
		TenantID:                       req.TenantID,
		ProjectID:                      projectID,
		Provider:                       req.Provider,
		Model:                          req.Model,
		AccessKeyID:                    req.AccessKeyID,
		AccessKeySecret:                req.AccessKeySecret,
		AppKey:                         req.AppKey,
		TokenEndpoint:                  req.TokenEndpoint,
		TokenRegionID:                  req.TokenRegionID,
		TokenRefreshBeforeSeconds:      req.TokenRefreshBeforeSeconds,
		WebsocketURL:                   req.WebsocketURL,
		Format:                         req.Format,
		SampleRate:                     req.SampleRate,
		EnableIntermediateResult:       req.EnableIntermediateResult,
		EnablePunctuationPrediction:    req.EnablePunctuationPrediction,
		EnableInverseTextNormalization: req.EnableInverseTextNormalization,
		EnableWords:                    req.EnableWords,
		TimeoutMS:                      req.TimeoutMS,
		ConfigJSON:                     req.ConfigJSON,
		Enabled:                        req.Enabled,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ProviderConfigHandler) GetTTS(c *gin.Context) {
	tenantID, projectID := providerConfigScope(c)
	config, err := h.configService.GetDefaultTTS(c.Request.Context(), tenantID, projectID)
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, config)
}

func (h *ProviderConfigHandler) UpsertTTS(c *gin.Context) {
	projectID := parseUint64Default(c.Param("project_id"), 0)
	var req upsertTTSConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request body")
		return
	}

	config, err := h.configService.UpsertDefaultTTS(c.Request.Context(), service.UpsertTTSConfigInput{
		TenantID:                  req.TenantID,
		ProjectID:                 projectID,
		Provider:                  req.Provider,
		Model:                     req.Model,
		Voice:                     req.Voice,
		AccessKeyID:               req.AccessKeyID,
		AccessKeySecret:           req.AccessKeySecret,
		AppKey:                    req.AppKey,
		TokenEndpoint:             req.TokenEndpoint,
		TokenRegionID:             req.TokenRegionID,
		TokenRefreshBeforeSeconds: req.TokenRefreshBeforeSeconds,
		WebsocketURL:              req.WebsocketURL,
		Format:                    req.Format,
		SampleRate:                req.SampleRate,
		Volume:                    req.Volume,
		SpeechRate:                req.SpeechRate,
		PitchRate:                 req.PitchRate,
		EnableSubtitle:            req.EnableSubtitle,
		TimeoutMS:                 req.TimeoutMS,
		ConfigJSON:                req.ConfigJSON,
		Enabled:                   req.Enabled,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, config)
}

func providerConfigScope(c *gin.Context) (uint64, uint64) {
	return parseUint64Default(c.Query("tenant_id"), 0), parseUint64Default(c.Param("project_id"), 0)
}
