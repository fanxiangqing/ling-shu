package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ling-shu/internal/asr"
	"ling-shu/internal/llm"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/internal/tts"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	defaultLLMAPIBase        = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	defaultLLMModel          = "qwen-plus"
	defaultLLMEmbeddingModel = "text-embedding-v4"
	defaultASRModel          = "nls-realtime-asr"
	defaultASRFormat         = "pcm"
	defaultASRSampleRate     = 16000
	defaultASRTimeoutMS      = 120000
	defaultTTSModel          = "nls-tts"
	defaultTTSVoice          = "aixia"
	defaultTTSFormat         = "mp3"
	defaultTTSSampleRate     = 16000
	defaultTTSVolume         = 50
	defaultTTSTimeoutMS      = 60000
	defaultNLSTokenEndpoint  = "https://nls-meta.cn-shanghai.aliyuncs.com/"
	defaultNLSTokenRegionID  = "cn-shanghai"
	defaultNLSWebsocketURL   = "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1"
	defaultNLSRefreshBefore  = 600
)

type ProviderConfigService struct {
	configRepo repository.ProviderConfigRepository
	logger     *zap.Logger
}

type UpsertLLMConfigInput struct {
	TenantID   uint64
	ProjectID  uint64
	Provider   string
	Model      string
	APIBase    string
	APIKey     string
	ConfigJSON string
	Enabled    *bool
}

type UpsertASRConfigInput struct {
	TenantID                       uint64
	ProjectID                      uint64
	Provider                       string
	Model                          string
	AccessKeyID                    string
	AccessKeySecret                string
	AppKey                         string
	TokenEndpoint                  string
	TokenRegionID                  string
	TokenRefreshBeforeSeconds      *int
	WebsocketURL                   string
	Format                         string
	SampleRate                     *int
	EnableIntermediateResult       *bool
	EnablePunctuationPrediction    *bool
	EnableInverseTextNormalization *bool
	EnableWords                    *bool
	TimeoutMS                      *int
	ConfigJSON                     string
	Enabled                        *bool
}

type UpsertTTSConfigInput struct {
	TenantID                  uint64
	ProjectID                 uint64
	Provider                  string
	Model                     string
	Voice                     string
	AccessKeyID               string
	AccessKeySecret           string
	AppKey                    string
	TokenEndpoint             string
	TokenRegionID             string
	TokenRefreshBeforeSeconds *int
	WebsocketURL              string
	Format                    string
	SampleRate                *int
	Volume                    *int
	SpeechRate                *int
	PitchRate                 *int
	EnableSubtitle            *bool
	TimeoutMS                 *int
	ConfigJSON                string
	Enabled                   *bool
}

func NewProviderConfigService(configRepo repository.ProviderConfigRepository) *ProviderConfigService {
	return &ProviderConfigService{configRepo: configRepo, logger: zap.NewNop()}
}

func (s *ProviderConfigService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	s.logger = logger
}

func (s *ProviderConfigService) ResolveLLMProvider(ctx context.Context, tenantID uint64, projectID uint64) (llm.Provider, error) {
	config, err := s.GetDefaultLLM(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled || strings.TrimSpace(config.APIKeyCiphertext) == "" {
		s.logger.Debug("llm provider config is not enabled or incomplete",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Bool("enabled", config.Enabled),
			zap.Bool("has_api_key", strings.TrimSpace(config.APIKeyCiphertext) != ""),
			zap.String("model", config.Model),
		)
		return nil, ErrProviderNotConfigured
	}
	if strings.ToLower(strings.TrimSpace(config.Provider)) != llm.ProviderAliyun {
		s.logger.Warn("llm provider config has unsupported provider",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.String("provider", config.Provider),
		)
		return nil, ErrInvalidInput
	}
	return llm.NewAliyunProvider(llm.AliyunConfig{
		APIKey:         config.APIKeyCiphertext,
		BaseURL:        config.APIBase,
		ChatModel:      config.Model,
		EmbeddingModel: defaultLLMEmbeddingModel,
	}), nil
}

func (s *ProviderConfigService) ResolveASRProvider(ctx context.Context, tenantID uint64, projectID uint64) (asr.Provider, error) {
	config, err := s.GetDefaultASR(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled || strings.TrimSpace(config.AccessKeyIDCiphertext) == "" || strings.TrimSpace(config.AccessKeySecretCiphertext) == "" || strings.TrimSpace(config.AppKey) == "" {
		s.logger.Debug("asr provider config is not enabled or incomplete",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Bool("enabled", config.Enabled),
			zap.Bool("has_access_key_id", strings.TrimSpace(config.AccessKeyIDCiphertext) != ""),
			zap.Bool("has_access_key_secret", strings.TrimSpace(config.AccessKeySecretCiphertext) != ""),
			zap.Bool("has_app_key", strings.TrimSpace(config.AppKey) != ""),
			zap.String("model", config.Model),
		)
		return nil, ErrProviderNotConfigured
	}
	if strings.ToLower(strings.TrimSpace(config.Provider)) != asr.ProviderAliyun {
		s.logger.Warn("asr provider config has unsupported provider",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.String("provider", config.Provider),
		)
		return nil, ErrInvalidInput
	}
	return asr.NewAliyunProvider(asr.AliyunConfig{
		AccessKeyID:                    config.AccessKeyIDCiphertext,
		AccessKeySecret:                config.AccessKeySecretCiphertext,
		TokenEndpoint:                  config.TokenEndpoint,
		TokenRegionID:                  config.TokenRegionID,
		TokenRefreshBefore:             secondsDuration(config.TokenRefreshBeforeSeconds, defaultNLSRefreshBefore),
		AppKey:                         config.AppKey,
		WebsocketURL:                   config.WebsocketURL,
		Model:                          config.Model,
		Format:                         config.AudioFormat,
		SampleRate:                     config.SampleRate,
		EnableIntermediateResult:       config.EnableIntermediateResult,
		EnablePunctuationPrediction:    config.EnablePunctuationPrediction,
		EnableInverseTextNormalization: config.EnableInverseTextNormalization,
		EnableWords:                    config.EnableWords,
		Timeout:                        millisecondsDuration(config.TimeoutMS, defaultASRTimeoutMS),
	}), nil
}

func (s *ProviderConfigService) ResolveTTSProvider(ctx context.Context, tenantID uint64, projectID uint64) (tts.Provider, error) {
	config, err := s.GetDefaultTTS(ctx, tenantID, projectID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled || strings.TrimSpace(config.AccessKeyIDCiphertext) == "" || strings.TrimSpace(config.AccessKeySecretCiphertext) == "" || strings.TrimSpace(config.AppKey) == "" {
		s.logger.Debug("tts provider config is not enabled or incomplete",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Bool("enabled", config.Enabled),
			zap.Bool("has_access_key_id", strings.TrimSpace(config.AccessKeyIDCiphertext) != ""),
			zap.Bool("has_access_key_secret", strings.TrimSpace(config.AccessKeySecretCiphertext) != ""),
			zap.Bool("has_app_key", strings.TrimSpace(config.AppKey) != ""),
			zap.String("model", config.Model),
		)
		return nil, ErrProviderNotConfigured
	}
	if strings.ToLower(strings.TrimSpace(config.Provider)) != tts.ProviderAliyun {
		s.logger.Warn("tts provider config has unsupported provider",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.String("provider", config.Provider),
		)
		return nil, ErrInvalidInput
	}
	return tts.NewAliyunProvider(tts.AliyunConfig{
		AccessKeyID:        config.AccessKeyIDCiphertext,
		AccessKeySecret:    config.AccessKeySecretCiphertext,
		TokenEndpoint:      config.TokenEndpoint,
		TokenRegionID:      config.TokenRegionID,
		TokenRefreshBefore: secondsDuration(config.TokenRefreshBeforeSeconds, defaultNLSRefreshBefore),
		AppKey:             config.AppKey,
		WebsocketURL:       config.WebsocketURL,
		Model:              config.Model,
		Voice:              config.Voice,
		Format:             config.AudioFormat,
		SampleRate:         config.SampleRate,
		Volume:             config.Volume,
		SpeechRate:         config.SpeechRate,
		PitchRate:          config.PitchRate,
		EnableSubtitle:     config.EnableSubtitle,
		Timeout:            millisecondsDuration(config.TimeoutMS, defaultTTSTimeoutMS),
	}), nil
}

func (s *ProviderConfigService) GetDefaultLLM(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectLLMConfig, error) {
	if tenantID == 0 || projectID == 0 {
		return nil, ErrInvalidInput
	}
	config, err := s.configRepo.GetDefaultLLM(ctx, tenantID, projectID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("llm provider config get failed",
				zap.Uint64("tenant_id", tenantID),
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
		return nil, err
	}
	return config, nil
}

func (s *ProviderConfigService) UpsertDefaultLLM(ctx context.Context, input UpsertLLMConfigInput) (*model.ProjectLLMConfig, error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return nil, ErrInvalidInput
	}

	existing, err := s.configRepo.GetDefaultLLM(ctx, input.TenantID, input.ProjectID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("llm provider config load before upsert failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}
	configJSON, err := mergeJSONConfig(input.ConfigJSON, existingStringPtr(existing, func(c *model.ProjectLLMConfig) *string {
		return c.ConfigJSON
	}))
	if err != nil {
		s.logger.Warn("llm provider config json invalid",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}

	provider := lowerStringChoice(input.Provider, existingString(existing, func(c *model.ProjectLLMConfig) string { return c.Provider }), llm.ProviderAliyun)
	if provider != llm.ProviderAliyun {
		s.logger.Warn("llm provider config upsert rejected unsupported provider",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", provider),
		)
		return nil, ErrInvalidInput
	}

	config := &model.ProjectLLMConfig{
		TenantID:         input.TenantID,
		ProjectID:        input.ProjectID,
		Provider:         provider,
		Model:            stringChoice(input.Model, existingString(existing, func(c *model.ProjectLLMConfig) string { return c.Model }), defaultLLMModel),
		APIBase:          stringChoice(input.APIBase, existingString(existing, func(c *model.ProjectLLMConfig) string { return c.APIBase }), defaultLLMAPIBase),
		APIKeyCiphertext: stringChoice(input.APIKey, existingString(existing, func(c *model.ProjectLLMConfig) string { return c.APIKeyCiphertext }), ""),
		ConfigJSON:       configJSON,
		Enabled:          boolChoice(input.Enabled, existingBool(existing, func(c *model.ProjectLLMConfig) bool { return c.Enabled }), true),
		IsDefault:        true,
	}
	if err := s.configRepo.UpsertDefaultLLM(ctx, config); err != nil {
		s.logger.Error("llm provider config upsert failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", config.Provider),
			zap.String("model", config.Model),
			zap.Bool("enabled", config.Enabled),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("llm provider config upserted",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("provider", config.Provider),
		zap.String("model", config.Model),
		zap.Bool("enabled", config.Enabled),
	)
	return s.GetDefaultLLM(ctx, input.TenantID, input.ProjectID)
}

func (s *ProviderConfigService) GetDefaultASR(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectASRConfig, error) {
	if tenantID == 0 || projectID == 0 {
		return nil, ErrInvalidInput
	}
	config, err := s.configRepo.GetDefaultASR(ctx, tenantID, projectID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("asr provider config get failed",
				zap.Uint64("tenant_id", tenantID),
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
		return nil, err
	}
	return config, nil
}

func (s *ProviderConfigService) UpsertDefaultASR(ctx context.Context, input UpsertASRConfigInput) (*model.ProjectASRConfig, error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return nil, ErrInvalidInput
	}

	existing, err := s.configRepo.GetDefaultASR(ctx, input.TenantID, input.ProjectID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("asr provider config load before upsert failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}
	configJSON, err := mergeJSONConfig(input.ConfigJSON, existingStringPtr(existing, func(c *model.ProjectASRConfig) *string {
		return c.ConfigJSON
	}))
	if err != nil {
		s.logger.Warn("asr provider config json invalid",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}

	provider := lowerStringChoice(input.Provider, existingString(existing, func(c *model.ProjectASRConfig) string { return c.Provider }), asr.ProviderAliyun)
	if provider != asr.ProviderAliyun {
		s.logger.Warn("asr provider config upsert rejected unsupported provider",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", provider),
		)
		return nil, ErrInvalidInput
	}

	sampleRate := intChoice(input.SampleRate, existingInt(existing, func(c *model.ProjectASRConfig) int { return c.SampleRate }), defaultASRSampleRate)
	if sampleRate != 8000 && sampleRate != 16000 {
		s.logger.Warn("asr provider config upsert rejected invalid sample rate",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Int("sample_rate", sampleRate),
		)
		return nil, ErrInvalidInput
	}

	tokenRefreshBefore := intChoice(input.TokenRefreshBeforeSeconds, existingInt(existing, func(c *model.ProjectASRConfig) int {
		return c.TokenRefreshBeforeSeconds
	}), defaultNLSRefreshBefore)
	timeoutMS := intChoice(input.TimeoutMS, existingInt(existing, func(c *model.ProjectASRConfig) int { return c.TimeoutMS }), defaultASRTimeoutMS)
	if tokenRefreshBefore <= 0 || timeoutMS <= 0 {
		s.logger.Warn("asr provider config upsert rejected invalid timeout settings",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Int("token_refresh_before_seconds", tokenRefreshBefore),
			zap.Int("timeout_ms", timeoutMS),
		)
		return nil, ErrInvalidInput
	}

	config := &model.ProjectASRConfig{
		TenantID:                       input.TenantID,
		ProjectID:                      input.ProjectID,
		Provider:                       provider,
		Model:                          stringChoice(input.Model, existingString(existing, func(c *model.ProjectASRConfig) string { return c.Model }), defaultASRModel),
		AccessKeyIDCiphertext:          stringChoice(input.AccessKeyID, existingString(existing, func(c *model.ProjectASRConfig) string { return c.AccessKeyIDCiphertext }), ""),
		AccessKeySecretCiphertext:      stringChoice(input.AccessKeySecret, existingString(existing, func(c *model.ProjectASRConfig) string { return c.AccessKeySecretCiphertext }), ""),
		AppKey:                         stringChoice(input.AppKey, existingString(existing, func(c *model.ProjectASRConfig) string { return c.AppKey }), ""),
		TokenEndpoint:                  stringChoice(input.TokenEndpoint, existingString(existing, func(c *model.ProjectASRConfig) string { return c.TokenEndpoint }), defaultNLSTokenEndpoint),
		TokenRegionID:                  stringChoice(input.TokenRegionID, existingString(existing, func(c *model.ProjectASRConfig) string { return c.TokenRegionID }), defaultNLSTokenRegionID),
		TokenRefreshBeforeSeconds:      tokenRefreshBefore,
		WebsocketURL:                   stringChoice(input.WebsocketURL, existingString(existing, func(c *model.ProjectASRConfig) string { return c.WebsocketURL }), defaultNLSWebsocketURL),
		AudioFormat:                    lowerStringChoice(input.Format, existingString(existing, func(c *model.ProjectASRConfig) string { return c.AudioFormat }), defaultASRFormat),
		SampleRate:                     sampleRate,
		EnableIntermediateResult:       boolChoice(input.EnableIntermediateResult, existingBool(existing, func(c *model.ProjectASRConfig) bool { return c.EnableIntermediateResult }), true),
		EnablePunctuationPrediction:    boolChoice(input.EnablePunctuationPrediction, existingBool(existing, func(c *model.ProjectASRConfig) bool { return c.EnablePunctuationPrediction }), true),
		EnableInverseTextNormalization: boolChoice(input.EnableInverseTextNormalization, existingBool(existing, func(c *model.ProjectASRConfig) bool { return c.EnableInverseTextNormalization }), true),
		EnableWords:                    boolChoice(input.EnableWords, existingBool(existing, func(c *model.ProjectASRConfig) bool { return c.EnableWords }), false),
		TimeoutMS:                      timeoutMS,
		ConfigJSON:                     configJSON,
		Enabled:                        boolChoice(input.Enabled, existingBool(existing, func(c *model.ProjectASRConfig) bool { return c.Enabled }), false),
		IsDefault:                      true,
	}
	if err := s.configRepo.UpsertDefaultASR(ctx, config); err != nil {
		s.logger.Error("asr provider config upsert failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", config.Provider),
			zap.String("model", config.Model),
			zap.Bool("enabled", config.Enabled),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("asr provider config upserted",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("provider", config.Provider),
		zap.String("model", config.Model),
		zap.Bool("enabled", config.Enabled),
	)
	return s.GetDefaultASR(ctx, input.TenantID, input.ProjectID)
}

func (s *ProviderConfigService) GetDefaultTTS(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectTTSConfig, error) {
	if tenantID == 0 || projectID == 0 {
		return nil, ErrInvalidInput
	}
	config, err := s.configRepo.GetDefaultTTS(ctx, tenantID, projectID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("tts provider config get failed",
				zap.Uint64("tenant_id", tenantID),
				zap.Uint64("project_id", projectID),
				zap.Error(err),
			)
		}
		return nil, err
	}
	return config, nil
}

func (s *ProviderConfigService) UpsertDefaultTTS(ctx context.Context, input UpsertTTSConfigInput) (*model.ProjectTTSConfig, error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return nil, ErrInvalidInput
	}

	existing, err := s.configRepo.GetDefaultTTS(ctx, input.TenantID, input.ProjectID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("tts provider config load before upsert failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}
	configJSON, err := mergeJSONConfig(input.ConfigJSON, existingStringPtr(existing, func(c *model.ProjectTTSConfig) *string {
		return c.ConfigJSON
	}))
	if err != nil {
		s.logger.Warn("tts provider config json invalid",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Error(err),
		)
		return nil, err
	}

	provider := lowerStringChoice(input.Provider, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.Provider }), tts.ProviderAliyun)
	if provider != tts.ProviderAliyun {
		s.logger.Warn("tts provider config upsert rejected unsupported provider",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", provider),
		)
		return nil, ErrInvalidInput
	}

	sampleRate := intChoice(input.SampleRate, existingInt(existing, func(c *model.ProjectTTSConfig) int { return c.SampleRate }), defaultTTSSampleRate)
	volume := intChoice(input.Volume, existingInt(existing, func(c *model.ProjectTTSConfig) int { return c.Volume }), defaultTTSVolume)
	speechRate := intChoice(input.SpeechRate, existingInt(existing, func(c *model.ProjectTTSConfig) int { return c.SpeechRate }), 0)
	pitchRate := intChoice(input.PitchRate, existingInt(existing, func(c *model.ProjectTTSConfig) int { return c.PitchRate }), 0)
	tokenRefreshBefore := intChoice(input.TokenRefreshBeforeSeconds, existingInt(existing, func(c *model.ProjectTTSConfig) int {
		return c.TokenRefreshBeforeSeconds
	}), defaultNLSRefreshBefore)
	timeoutMS := intChoice(input.TimeoutMS, existingInt(existing, func(c *model.ProjectTTSConfig) int { return c.TimeoutMS }), defaultTTSTimeoutMS)
	if sampleRate <= 0 || volume < 0 || volume > 100 || speechRate < -500 || speechRate > 500 || pitchRate < -500 || pitchRate > 500 || tokenRefreshBefore <= 0 || timeoutMS <= 0 {
		s.logger.Warn("tts provider config upsert rejected invalid audio settings",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Int("sample_rate", sampleRate),
			zap.Int("volume", volume),
			zap.Int("speech_rate", speechRate),
			zap.Int("pitch_rate", pitchRate),
			zap.Int("token_refresh_before_seconds", tokenRefreshBefore),
			zap.Int("timeout_ms", timeoutMS),
		)
		return nil, ErrInvalidInput
	}

	config := &model.ProjectTTSConfig{
		TenantID:                  input.TenantID,
		ProjectID:                 input.ProjectID,
		Provider:                  provider,
		Model:                     stringChoice(input.Model, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.Model }), defaultTTSModel),
		Voice:                     stringChoice(input.Voice, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.Voice }), defaultTTSVoice),
		AccessKeyIDCiphertext:     stringChoice(input.AccessKeyID, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.AccessKeyIDCiphertext }), ""),
		AccessKeySecretCiphertext: stringChoice(input.AccessKeySecret, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.AccessKeySecretCiphertext }), ""),
		AppKey:                    stringChoice(input.AppKey, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.AppKey }), ""),
		TokenEndpoint:             stringChoice(input.TokenEndpoint, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.TokenEndpoint }), defaultNLSTokenEndpoint),
		TokenRegionID:             stringChoice(input.TokenRegionID, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.TokenRegionID }), defaultNLSTokenRegionID),
		TokenRefreshBeforeSeconds: tokenRefreshBefore,
		WebsocketURL:              stringChoice(input.WebsocketURL, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.WebsocketURL }), defaultNLSWebsocketURL),
		AudioFormat:               lowerStringChoice(input.Format, existingString(existing, func(c *model.ProjectTTSConfig) string { return c.AudioFormat }), defaultTTSFormat),
		SampleRate:                sampleRate,
		Volume:                    volume,
		SpeechRate:                speechRate,
		PitchRate:                 pitchRate,
		EnableSubtitle:            boolChoice(input.EnableSubtitle, existingBool(existing, func(c *model.ProjectTTSConfig) bool { return c.EnableSubtitle }), false),
		TimeoutMS:                 timeoutMS,
		ConfigJSON:                configJSON,
		Enabled:                   boolChoice(input.Enabled, existingBool(existing, func(c *model.ProjectTTSConfig) bool { return c.Enabled }), false),
		IsDefault:                 true,
	}
	if err := s.configRepo.UpsertDefaultTTS(ctx, config); err != nil {
		s.logger.Error("tts provider config upsert failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", config.Provider),
			zap.String("model", config.Model),
			zap.Bool("enabled", config.Enabled),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("tts provider config upserted",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("provider", config.Provider),
		zap.String("model", config.Model),
		zap.Bool("enabled", config.Enabled),
	)
	return s.GetDefaultTTS(ctx, input.TenantID, input.ProjectID)
}

func mergeJSONConfig(raw string, existing *string) (*string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return existing, nil
	}
	if !json.Valid([]byte(value)) {
		return nil, ErrInvalidInput
	}
	return &value, nil
}

func stringChoice(value string, existing string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		return trimmed
	}
	if strings.TrimSpace(existing) != "" {
		return strings.TrimSpace(existing)
	}
	return fallback
}

func lowerStringChoice(value string, existing string, fallback string) string {
	return strings.ToLower(stringChoice(value, existing, fallback))
}

func intChoice(value *int, existing int, fallback int) int {
	if value != nil {
		return *value
	}
	if existing != 0 {
		return existing
	}
	return fallback
}

func boolChoice(value *bool, existing *bool, fallback bool) bool {
	if value != nil {
		return *value
	}
	if existing != nil {
		return *existing
	}
	return fallback
}

func existingString[T any](existing *T, getter func(*T) string) string {
	if existing == nil {
		return ""
	}
	return getter(existing)
}

func existingStringPtr[T any](existing *T, getter func(*T) *string) *string {
	if existing == nil {
		return nil
	}
	return getter(existing)
}

func existingInt[T any](existing *T, getter func(*T) int) int {
	if existing == nil {
		return 0
	}
	return getter(existing)
}

func existingBool[T any](existing *T, getter func(*T) bool) *bool {
	if existing == nil {
		return nil
	}
	value := getter(existing)
	return &value
}

func secondsDuration(value int, fallback int) time.Duration {
	if value <= 0 {
		value = fallback
	}
	return time.Duration(value) * time.Second
}

func millisecondsDuration(value int, fallback int) time.Duration {
	if value <= 0 {
		value = fallback
	}
	return time.Duration(value) * time.Millisecond
}
