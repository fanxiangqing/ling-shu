package service

import (
	"context"
	"errors"
	"time"

	"ling-shu/internal/asr"
	"ling-shu/internal/llm"
	"ling-shu/internal/tts"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrProviderNotConfigured = errors.New("provider is not configured")
var ErrProviderStreamingUnsupported = errors.New("provider streaming audio is not supported")

type ProviderService struct {
	llmProvider   llm.Provider
	asrProvider   asr.Provider
	ttsProvider   tts.Provider
	configService *ProviderConfigService
	logger        *zap.Logger
}

type ProviderSummary struct {
	LLM ProviderInfo `json:"llm"`
	ASR ProviderInfo `json:"asr"`
	TTS ProviderInfo `json:"tts"`
}

type ProviderInfo struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Model      string `json:"model"`
	Source     string `json:"source,omitempty"`
}

type ProviderScopeInput struct {
	TenantID  uint64
	ProjectID uint64
}

type ProviderChatInput struct {
	TenantID    uint64
	ProjectID   uint64
	Model       string
	Messages    []llm.Message
	Temperature *float64
	MaxTokens   int
}

type ProviderTranscribeInput struct {
	TenantID  uint64
	ProjectID uint64
	Model     string
	AudioURL  string
	Language  string
}

type ProviderTranscribeAudioInput struct {
	TenantID  uint64
	ProjectID uint64
	Model     string
	Language  string
}

type ProviderSynthesizeInput struct {
	TenantID  uint64
	ProjectID uint64
	Model     string
	Text      string
	Voice     string
	Format    string
}

func NewProviderService(llmProvider llm.Provider, asrProvider asr.Provider, ttsProvider tts.Provider, configServices ...*ProviderConfigService) *ProviderService {
	var configService *ProviderConfigService
	if len(configServices) > 0 {
		configService = configServices[0]
	}
	return &ProviderService{
		llmProvider:   llmProvider,
		asrProvider:   asrProvider,
		ttsProvider:   ttsProvider,
		configService: configService,
		logger:        zap.NewNop(),
	}
}

func (s *ProviderService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *ProviderService) Summary() ProviderSummary {
	return s.SummaryWithScope(context.Background(), ProviderScopeInput{})
}

func (s *ProviderService) SummaryWithScope(ctx context.Context, input ProviderScopeInput) ProviderSummary {
	llmProvider, llmSource := s.resolveLLMProviderForSummary(ctx, input)
	asrProvider, asrSource := s.resolveASRProviderForSummary(ctx, input)
	ttsProvider, ttsSource := s.resolveTTSProviderForSummary(ctx, input)
	return ProviderSummary{
		LLM: ProviderInfo{
			Provider:   providerName(llmProvider),
			Configured: providerConfigured(llmProvider),
			Model:      defaultChatModel(llmProvider),
			Source:     llmSource,
		},
		ASR: ProviderInfo{
			Provider:   asrProviderName(asrProvider),
			Configured: asrProviderConfigured(asrProvider),
			Model:      asrDefaultModel(asrProvider),
			Source:     asrSource,
		},
		TTS: ProviderInfo{
			Provider:   ttsProviderName(ttsProvider),
			Configured: ttsProviderConfigured(ttsProvider),
			Model:      ttsDefaultModel(ttsProvider),
			Source:     ttsSource,
		},
	}
}

func (s *ProviderService) Chat(ctx context.Context, input ProviderChatInput) (*llm.ChatResponse, error) {
	started := time.Now()
	provider, err := s.ResolveLLMProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return nil, err
	}
	if provider == nil || !provider.Configured() {
		return nil, ErrProviderNotConfigured
	}
	if len(input.Messages) == 0 {
		return nil, ErrInvalidInput
	}
	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Model:       input.Model,
		Messages:    input.Messages,
		Temperature: input.Temperature,
		MaxTokens:   input.MaxTokens,
	})
	if err != nil {
		s.logger.Error("provider llm chat failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("model", firstNonEmptyService(input.Model, defaultChatModel(provider))),
			zap.Int("message_count", len(input.Messages)),
			zap.Int("max_tokens", input.MaxTokens),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Debug("provider llm chat succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", firstNonEmptyService(input.Model, resp.Model, defaultChatModel(provider))),
		zap.Int("message_count", len(input.Messages)),
		zap.Duration("duration", time.Since(started)),
	)
	return resp, nil
}

func (s *ProviderService) StreamChat(ctx context.Context, input ProviderChatInput, onEvent func(llm.ChatStreamEvent) error) error {
	started := time.Now()
	provider, err := s.ResolveLLMProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return err
	}
	if provider == nil || !provider.Configured() {
		return ErrProviderNotConfigured
	}
	if len(input.Messages) == 0 || onEvent == nil {
		return ErrInvalidInput
	}
	err = provider.StreamChat(ctx, llm.ChatRequest{
		Model:       input.Model,
		Messages:    input.Messages,
		Temperature: input.Temperature,
		MaxTokens:   input.MaxTokens,
	}, onEvent)
	if err != nil {
		s.logger.Error("provider llm stream chat failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("model", firstNonEmptyService(input.Model, defaultChatModel(provider))),
			zap.Int("message_count", len(input.Messages)),
			zap.Int("max_tokens", input.MaxTokens),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return err
	}
	s.logger.Debug("provider llm stream chat succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", firstNonEmptyService(input.Model, defaultChatModel(provider))),
		zap.Int("message_count", len(input.Messages)),
		zap.Duration("duration", time.Since(started)),
	)
	return nil
}

func (s *ProviderService) Transcribe(ctx context.Context, input ProviderTranscribeInput) (*asr.TranscribeResponse, error) {
	started := time.Now()
	provider, err := s.ResolveASRProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return nil, err
	}
	if provider == nil || !provider.Configured() {
		return nil, ErrProviderNotConfigured
	}
	if input.AudioURL == "" {
		return nil, ErrInvalidInput
	}
	resp, err := provider.Transcribe(ctx, asr.TranscribeRequest{
		Model:    input.Model,
		AudioURL: input.AudioURL,
		Language: input.Language,
	})
	if err != nil {
		s.logger.Error("provider asr transcribe failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("model", firstNonEmptyService(input.Model, asrDefaultModel(provider))),
			zap.String("audio_url_hash", sqlHash(input.AudioURL)),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Debug("provider asr transcribe succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", firstNonEmptyService(input.Model, asrDefaultModel(provider))),
		zap.String("audio_url_hash", sqlHash(input.AudioURL)),
		zap.Duration("duration", time.Since(started)),
	)
	return resp, nil
}

func (s *ProviderService) StreamTranscribe(ctx context.Context, input ProviderTranscribeInput, onEvent func(asr.TranscribeStreamEvent) error) error {
	started := time.Now()
	provider, err := s.ResolveASRProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return err
	}
	if provider == nil || !provider.Configured() {
		return ErrProviderNotConfigured
	}
	if input.AudioURL == "" || onEvent == nil {
		return ErrInvalidInput
	}
	err = provider.StreamTranscribe(ctx, asr.TranscribeRequest{
		Model:    input.Model,
		AudioURL: input.AudioURL,
		Language: input.Language,
	}, onEvent)
	if err != nil {
		s.logger.Error("provider asr stream transcribe failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("model", firstNonEmptyService(input.Model, asrDefaultModel(provider))),
			zap.String("audio_url_hash", sqlHash(input.AudioURL)),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return err
	}
	s.logger.Debug("provider asr stream transcribe succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", firstNonEmptyService(input.Model, asrDefaultModel(provider))),
		zap.String("audio_url_hash", sqlHash(input.AudioURL)),
		zap.Duration("duration", time.Since(started)),
	)
	return nil
}

func (s *ProviderService) StreamTranscribeAudio(ctx context.Context, input ProviderTranscribeAudioInput, audio <-chan []byte, onEvent func(asr.TranscribeStreamEvent) error) error {
	started := time.Now()
	provider, err := s.ResolveASRProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return err
	}
	if provider == nil || !provider.Configured() {
		return ErrProviderNotConfigured
	}
	streamProvider, ok := provider.(asr.AudioStreamProvider)
	if !ok {
		return ErrProviderStreamingUnsupported
	}
	if audio == nil || onEvent == nil {
		return ErrInvalidInput
	}
	err = streamProvider.StreamTranscribeAudio(ctx, asr.TranscribeRequest{
		Model:    input.Model,
		Language: input.Language,
	}, audio, onEvent)
	if err != nil {
		s.logger.Error("provider asr realtime transcribe failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("model", firstNonEmptyService(input.Model, asrDefaultModel(provider))),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return err
	}
	s.logger.Debug("provider asr realtime transcribe succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", firstNonEmptyService(input.Model, asrDefaultModel(provider))),
		zap.Duration("duration", time.Since(started)),
	)
	return nil
}

func (s *ProviderService) GetTranscribeTask(ctx context.Context, taskID string) (*asr.TranscribeResponse, error) {
	if s.asrProvider == nil || !s.asrProvider.Configured() {
		return nil, ErrProviderNotConfigured
	}
	if taskID == "" {
		return nil, ErrInvalidInput
	}
	started := time.Now()
	resp, err := s.asrProvider.GetTask(ctx, taskID)
	if err != nil {
		s.logger.Error("provider asr task fetch failed",
			zap.String("task_id", taskID),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Debug("provider asr task fetch succeeded",
		zap.String("task_id", taskID),
		zap.String("status", resp.Status),
		zap.Duration("duration", time.Since(started)),
	)
	return resp, nil
}

func (s *ProviderService) Synthesize(ctx context.Context, input ProviderSynthesizeInput) (*tts.SynthesizeResponse, error) {
	started := time.Now()
	provider, err := s.ResolveTTSProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return nil, err
	}
	if provider == nil || !provider.Configured() {
		return nil, ErrProviderNotConfigured
	}
	if input.Text == "" {
		return nil, ErrInvalidInput
	}
	resp, err := provider.Synthesize(ctx, tts.SynthesizeRequest{
		Model:  input.Model,
		Text:   input.Text,
		Voice:  input.Voice,
		Format: input.Format,
	})
	if err != nil {
		s.logger.Error("provider tts synthesize failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", provider.Name()),
			zap.String("model", firstNonEmptyService(input.Model, ttsDefaultModel(provider))),
			zap.String("voice", firstNonEmptyService(input.Voice, ttsDefaultVoice(provider))),
			zap.String("format", firstNonEmptyService(input.Format, ttsDefaultFormat(provider))),
			zap.Int("text_chars", len([]rune(input.Text))),
			zap.String("text_hash", sqlHash(input.Text)),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Debug("provider tts synthesize succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("provider", provider.Name()),
		zap.String("model", firstNonEmptyService(input.Model, ttsDefaultModel(provider))),
		zap.String("voice", firstNonEmptyService(input.Voice, ttsDefaultVoice(provider))),
		zap.String("format", firstNonEmptyService(input.Format, ttsDefaultFormat(provider))),
		zap.Int("text_chars", len([]rune(input.Text))),
		zap.String("text_hash", sqlHash(input.Text)),
		zap.Duration("duration", time.Since(started)),
	)
	return resp, nil
}

func (s *ProviderService) StreamSynthesize(ctx context.Context, input ProviderSynthesizeInput, onEvent func(tts.SynthesizeStreamEvent) error) error {
	started := time.Now()
	provider, err := s.ResolveTTSProvider(ctx, ProviderScopeInput{TenantID: input.TenantID, ProjectID: input.ProjectID})
	if err != nil {
		return err
	}
	if provider == nil || !provider.Configured() {
		return ErrProviderNotConfigured
	}
	if input.Text == "" || onEvent == nil {
		return ErrInvalidInput
	}
	err = provider.StreamSynthesize(ctx, tts.SynthesizeRequest{
		Model:  input.Model,
		Text:   input.Text,
		Voice:  input.Voice,
		Format: input.Format,
	}, onEvent)
	if err != nil {
		s.logger.Error("provider tts stream synthesize failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("provider", provider.Name()),
			zap.String("model", firstNonEmptyService(input.Model, ttsDefaultModel(provider))),
			zap.String("voice", firstNonEmptyService(input.Voice, ttsDefaultVoice(provider))),
			zap.String("format", firstNonEmptyService(input.Format, ttsDefaultFormat(provider))),
			zap.Int("text_chars", len([]rune(input.Text))),
			zap.String("text_hash", sqlHash(input.Text)),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return err
	}
	s.logger.Debug("provider tts stream synthesize succeeded",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("provider", provider.Name()),
		zap.String("model", firstNonEmptyService(input.Model, ttsDefaultModel(provider))),
		zap.String("voice", firstNonEmptyService(input.Voice, ttsDefaultVoice(provider))),
		zap.String("format", firstNonEmptyService(input.Format, ttsDefaultFormat(provider))),
		zap.Int("text_chars", len([]rune(input.Text))),
		zap.String("text_hash", sqlHash(input.Text)),
		zap.Duration("duration", time.Since(started)),
	)
	return nil
}

func (s *ProviderService) ResolveLLMProvider(ctx context.Context, input ProviderScopeInput) (llm.Provider, error) {
	if s.configService != nil && input.TenantID > 0 && input.ProjectID > 0 {
		provider, err := s.configService.ResolveLLMProvider(ctx, input.TenantID, input.ProjectID)
		if err == nil {
			s.logger.Debug("llm provider resolved from project config",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.String("model", defaultChatModel(provider)),
				zap.Bool("configured", providerConfigured(provider)),
			)
			return provider, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("llm provider project config resolve failed",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Error(err),
			)
			return nil, err
		}
	}
	s.logger.Debug("llm provider resolved from global config",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", defaultChatModel(s.llmProvider)),
		zap.Bool("configured", providerConfigured(s.llmProvider)),
	)
	return s.llmProvider, nil
}

func (s *ProviderService) ResolveASRProvider(ctx context.Context, input ProviderScopeInput) (asr.Provider, error) {
	if s.configService != nil && input.TenantID > 0 && input.ProjectID > 0 {
		provider, err := s.configService.ResolveASRProvider(ctx, input.TenantID, input.ProjectID)
		if err == nil {
			s.logger.Debug("asr provider resolved from project config",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.String("model", asrDefaultModel(provider)),
				zap.Bool("configured", asrProviderConfigured(provider)),
			)
			return provider, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("asr provider project config resolve failed",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Error(err),
			)
			return nil, err
		}
	}
	s.logger.Debug("asr provider resolved from global config",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", asrDefaultModel(s.asrProvider)),
		zap.Bool("configured", asrProviderConfigured(s.asrProvider)),
	)
	return s.asrProvider, nil
}

func (s *ProviderService) ResolveTTSProvider(ctx context.Context, input ProviderScopeInput) (tts.Provider, error) {
	if s.configService != nil && input.TenantID > 0 && input.ProjectID > 0 {
		provider, err := s.configService.ResolveTTSProvider(ctx, input.TenantID, input.ProjectID)
		if err == nil {
			s.logger.Debug("tts provider resolved from project config",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.String("model", ttsDefaultModel(provider)),
				zap.Bool("configured", ttsProviderConfigured(provider)),
			)
			return provider, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("tts provider project config resolve failed",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Error(err),
			)
			return nil, err
		}
	}
	s.logger.Debug("tts provider resolved from global config",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.String("model", ttsDefaultModel(s.ttsProvider)),
		zap.Bool("configured", ttsProviderConfigured(s.ttsProvider)),
	)
	return s.ttsProvider, nil
}

func (s *ProviderService) resolveLLMProviderForSummary(ctx context.Context, input ProviderScopeInput) (llm.Provider, string) {
	if s.configService != nil && input.TenantID > 0 && input.ProjectID > 0 {
		provider, err := s.configService.ResolveLLMProvider(ctx, input.TenantID, input.ProjectID)
		if err == nil {
			return provider, "project"
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "project"
		}
	}
	return s.llmProvider, "global"
}

func (s *ProviderService) resolveASRProviderForSummary(ctx context.Context, input ProviderScopeInput) (asr.Provider, string) {
	if s.configService != nil && input.TenantID > 0 && input.ProjectID > 0 {
		provider, err := s.configService.ResolveASRProvider(ctx, input.TenantID, input.ProjectID)
		if err == nil {
			return provider, "project"
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "project"
		}
	}
	return s.asrProvider, "global"
}

func (s *ProviderService) resolveTTSProviderForSummary(ctx context.Context, input ProviderScopeInput) (tts.Provider, string) {
	if s.configService != nil && input.TenantID > 0 && input.ProjectID > 0 {
		provider, err := s.configService.ResolveTTSProvider(ctx, input.TenantID, input.ProjectID)
		if err == nil {
			return provider, "project"
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "project"
		}
	}
	return s.ttsProvider, "global"
}

func providerName(provider llm.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.Name()
}

func providerConfigured(provider llm.Provider) bool {
	return provider != nil && provider.Configured()
}

func defaultChatModel(provider llm.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.DefaultChatModel()
}

func asrProviderName(provider asr.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.Name()
}

func asrProviderConfigured(provider asr.Provider) bool {
	return provider != nil && provider.Configured()
}

func asrDefaultModel(provider asr.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.DefaultModel()
}

func ttsProviderName(provider tts.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.Name()
}

func ttsProviderConfigured(provider tts.Provider) bool {
	return provider != nil && provider.Configured()
}

func ttsDefaultModel(provider tts.Provider) string {
	if provider == nil {
		return ""
	}
	return provider.DefaultModel()
}

func ttsDefaultVoice(provider tts.Provider) string {
	if describer, ok := provider.(interface{ DefaultVoice() string }); ok {
		return describer.DefaultVoice()
	}
	return ""
}

func ttsDefaultFormat(provider tts.Provider) string {
	if describer, ok := provider.(interface{ DefaultFormat() string }); ok {
		return describer.DefaultFormat()
	}
	return ""
}
