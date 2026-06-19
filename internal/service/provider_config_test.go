package service

import (
	"context"
	"errors"
	"testing"

	"ling-shu/internal/llm"
	"ling-shu/internal/model"

	"gorm.io/gorm"
)

func TestProviderConfigServiceUpsertsStreamingConfigs(t *testing.T) {
	repo := &providerConfigFakeRepository{}
	service := NewProviderConfigService(repo)
	ctx := context.Background()

	llmEnabled := true
	llmConfig, err := service.UpsertDefaultLLM(ctx, UpsertLLMConfigInput{
		TenantID:   1,
		ProjectID:  2,
		APIKey:     "dashscope-secret",
		ConfigJSON: `{"temperature":0.2}`,
		Enabled:    &llmEnabled,
	})
	if err != nil {
		t.Fatalf("upsert llm config: %v", err)
	}
	if llmConfig.Provider != "aliyun" || llmConfig.Model != "qwen-plus" || llmConfig.APIBase == "" || !llmConfig.Enabled {
		t.Fatalf("unexpected llm config: %+v", llmConfig)
	}
	llmProvider, err := service.ResolveLLMProvider(ctx, 1, 2)
	if err != nil {
		t.Fatalf("resolve llm provider: %v", err)
	}
	if !llmProvider.Configured() || llmProvider.DefaultChatModel() != "qwen-plus" {
		t.Fatalf("unexpected resolved llm provider: configured=%v model=%s", llmProvider.Configured(), llmProvider.DefaultChatModel())
	}

	asrEnabled := true
	enableWords := true
	asrConfig, err := service.UpsertDefaultASR(ctx, UpsertASRConfigInput{
		TenantID:        1,
		ProjectID:       2,
		AccessKeyID:     "ak-id",
		AccessKeySecret: "ak-secret",
		AppKey:          "asr-app",
		EnableWords:     &enableWords,
		Enabled:         &asrEnabled,
	})
	if err != nil {
		t.Fatalf("upsert asr config: %v", err)
	}
	if asrConfig.Provider != "aliyun" || asrConfig.Model != "nls-realtime-asr" || asrConfig.AudioFormat != "pcm" || asrConfig.SampleRate != 16000 {
		t.Fatalf("unexpected asr defaults: %+v", asrConfig)
	}
	if !asrConfig.EnableIntermediateResult || !asrConfig.EnablePunctuationPrediction || !asrConfig.EnableInverseTextNormalization || !asrConfig.EnableWords || !asrConfig.Enabled {
		t.Fatalf("unexpected asr switches: %+v", asrConfig)
	}
	asrProvider, err := service.ResolveASRProvider(ctx, 1, 2)
	if err != nil {
		t.Fatalf("resolve asr provider: %v", err)
	}
	if !asrProvider.Configured() || asrProvider.DefaultModel() != "nls-realtime-asr" {
		t.Fatalf("unexpected resolved asr provider: configured=%v model=%s", asrProvider.Configured(), asrProvider.DefaultModel())
	}

	sampleRate := 8000
	asrConfig, err = service.UpsertDefaultASR(ctx, UpsertASRConfigInput{
		TenantID:   1,
		ProjectID:  2,
		SampleRate: &sampleRate,
	})
	if err != nil {
		t.Fatalf("update asr config: %v", err)
	}
	if asrConfig.SampleRate != 8000 || asrConfig.AccessKeySecretCiphertext != "ak-secret" {
		t.Fatalf("expected asr sample rate update and secret preservation, got %+v", asrConfig)
	}

	ttsEnabled := true
	volume := 80
	speechRate := 120
	ttsConfig, err := service.UpsertDefaultTTS(ctx, UpsertTTSConfigInput{
		TenantID:        1,
		ProjectID:       2,
		AccessKeyID:     "ak-id",
		AccessKeySecret: "ak-secret",
		AppKey:          "tts-app",
		Volume:          &volume,
		SpeechRate:      &speechRate,
		Enabled:         &ttsEnabled,
	})
	if err != nil {
		t.Fatalf("upsert tts config: %v", err)
	}
	if ttsConfig.Provider != "aliyun" || ttsConfig.Model != "nls-tts" || ttsConfig.Voice != "aixia" || ttsConfig.AudioFormat != "mp3" {
		t.Fatalf("unexpected tts defaults: %+v", ttsConfig)
	}
	if ttsConfig.Volume != 80 || ttsConfig.SpeechRate != 120 || !ttsConfig.Enabled {
		t.Fatalf("unexpected tts controls: %+v", ttsConfig)
	}
	ttsProvider, err := service.ResolveTTSProvider(ctx, 1, 2)
	if err != nil {
		t.Fatalf("resolve tts provider: %v", err)
	}
	if !ttsProvider.Configured() || ttsProvider.DefaultModel() != "nls-tts" {
		t.Fatalf("unexpected resolved tts provider: configured=%v model=%s", ttsProvider.Configured(), ttsProvider.DefaultModel())
	}
}

func TestProviderConfigServiceRejectsInvalidStreamingConfig(t *testing.T) {
	service := NewProviderConfigService(&providerConfigFakeRepository{})
	invalidSampleRate := 44100
	_, err := service.UpsertDefaultASR(context.Background(), UpsertASRConfigInput{
		TenantID:   1,
		ProjectID:  2,
		SampleRate: &invalidSampleRate,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}

	invalidVolume := 120
	_, err = service.UpsertDefaultTTS(context.Background(), UpsertTTSConfigInput{
		TenantID:  1,
		ProjectID: 2,
		Volume:    &invalidVolume,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestProviderServiceResolvesProjectConfigWithGlobalFallback(t *testing.T) {
	repo := &providerConfigFakeRepository{}
	configService := NewProviderConfigService(repo)
	globalLLM := serviceFakeLLMProvider{chatModel: "global-model", configured: true}
	providerService := NewProviderService(globalLLM, nil, nil, configService)

	provider, err := providerService.ResolveLLMProvider(context.Background(), ProviderScopeInput{TenantID: 1, ProjectID: 2})
	if err != nil {
		t.Fatalf("resolve global fallback: %v", err)
	}
	if provider.DefaultChatModel() != "global-model" {
		t.Fatalf("expected global fallback, got %s", provider.DefaultChatModel())
	}

	repo.llm = &model.ProjectLLMConfig{
		TenantID:         1,
		ProjectID:        2,
		Provider:         "aliyun",
		Model:            "project-model",
		APIBase:          "https://dashscope.aliyuncs.com/compatible-mode/v1",
		APIKeyCiphertext: "project-key",
		Enabled:          false,
		IsDefault:        true,
	}
	_, err = providerService.ResolveLLMProvider(context.Background(), ProviderScopeInput{TenantID: 1, ProjectID: 2})
	if !errors.Is(err, ErrProviderNotConfigured) {
		t.Fatalf("expected disabled project config to block fallback, got %v", err)
	}
}

type providerConfigFakeRepository struct {
	llm *model.ProjectLLMConfig
	asr *model.ProjectASRConfig
	tts *model.ProjectTTSConfig
}

func (r *providerConfigFakeRepository) GetDefaultLLM(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectLLMConfig, error) {
	if r.llm == nil || r.llm.TenantID != tenantID || r.llm.ProjectID != projectID {
		return nil, gorm.ErrRecordNotFound
	}
	config := *r.llm
	return &config, nil
}

func (r *providerConfigFakeRepository) UpsertDefaultLLM(ctx context.Context, config *model.ProjectLLMConfig) error {
	if config.ID == 0 {
		config.ID = 1
	}
	config.IsDefault = true
	copy := *config
	r.llm = &copy
	return nil
}

func (r *providerConfigFakeRepository) GetDefaultASR(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectASRConfig, error) {
	if r.asr == nil || r.asr.TenantID != tenantID || r.asr.ProjectID != projectID {
		return nil, gorm.ErrRecordNotFound
	}
	config := *r.asr
	return &config, nil
}

func (r *providerConfigFakeRepository) UpsertDefaultASR(ctx context.Context, config *model.ProjectASRConfig) error {
	if config.ID == 0 {
		config.ID = 1
	}
	config.IsDefault = true
	copy := *config
	r.asr = &copy
	return nil
}

func (r *providerConfigFakeRepository) GetDefaultTTS(ctx context.Context, tenantID uint64, projectID uint64) (*model.ProjectTTSConfig, error) {
	if r.tts == nil || r.tts.TenantID != tenantID || r.tts.ProjectID != projectID {
		return nil, gorm.ErrRecordNotFound
	}
	config := *r.tts
	return &config, nil
}

func (r *providerConfigFakeRepository) UpsertDefaultTTS(ctx context.Context, config *model.ProjectTTSConfig) error {
	if config.ID == 0 {
		config.ID = 1
	}
	config.IsDefault = true
	copy := *config
	r.tts = &copy
	return nil
}

type serviceFakeLLMProvider struct {
	chatModel  string
	configured bool
}

func (p serviceFakeLLMProvider) Name() string { return "aliyun" }
func (p serviceFakeLLMProvider) Configured() bool {
	return p.configured
}
func (p serviceFakeLLMProvider) DefaultChatModel() string {
	return p.chatModel
}
func (p serviceFakeLLMProvider) DefaultEmbeddingModel() string {
	return "text-embedding-v4"
}
func (p serviceFakeLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Model: p.chatModel}, nil
}
func (p serviceFakeLLMProvider) StreamChat(ctx context.Context, req llm.ChatRequest, onEvent func(llm.ChatStreamEvent) error) error {
	return onEvent(llm.ChatStreamEvent{Model: p.chatModel, Done: true})
}
func (p serviceFakeLLMProvider) Embeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return &llm.EmbeddingResponse{Model: p.DefaultEmbeddingModel()}, nil
}
