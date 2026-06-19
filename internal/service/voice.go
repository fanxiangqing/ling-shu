package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"ling-shu/internal/asr"
	"ling-shu/internal/query"
	"ling-shu/internal/tts"

	"go.uber.org/zap"
)

const (
	VoiceStreamStageStatus = "status"
	VoiceStreamStageASR    = "asr"
	VoiceStreamStageChat   = "chat"
	VoiceStreamStageTTS    = "tts"
)

type VoiceService struct {
	providerService *ProviderService
	chatService     *ChatService
	logger          *zap.Logger
}

type VoiceChatInput struct {
	TenantID              uint64
	ProjectID             uint64
	SessionID             uint64
	UserID                uint64
	AudioURL              string
	Language              string
	AutoExecute           bool
	MaxRows               int
	DatasourceID          uint64
	SelectedDatasourceIDs []uint64
	Voice                 string
	Format                string
	RequestID             string
	IP                    string
	UserAgent             string
	AuditOrigin           AuditOrigin
}

type RealtimeVoiceChatInput struct {
	TenantID              uint64
	ProjectID             uint64
	SessionID             uint64
	UserID                uint64
	Language              string
	AutoExecute           bool
	MaxRows               int
	DatasourceID          uint64
	SelectedDatasourceIDs []uint64
	Voice                 string
	Format                string
	RequestID             string
	IP                    string
	UserAgent             string
	AuditOrigin           AuditOrigin
}

type VoiceChatResult struct {
	Transcript *asr.TranscribeResponse `json:"transcript"`
	Chat       *SendChatMessageResult  `json:"chat"`
	Speech     *tts.SynthesizeResponse `json:"speech,omitempty"`
	SpeechText string                  `json:"speech_text,omitempty"`
}

type VoiceChatStreamEvent struct {
	Stage      string                     `json:"stage"`
	Message    string                     `json:"message,omitempty"`
	Transcript *asr.TranscribeStreamEvent `json:"transcript,omitempty"`
	Agent      *query.AgentEvent          `json:"agent,omitempty"`
	Speech     *tts.SynthesizeStreamEvent `json:"speech,omitempty"`
	Done       bool                       `json:"done,omitempty"`
}

func NewVoiceService(providerService *ProviderService, chatService *ChatService) *VoiceService {
	return &VoiceService{
		providerService: providerService,
		chatService:     chatService,
		logger:          zap.NewNop(),
	}
}

func (s *VoiceService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *VoiceService) Chat(ctx context.Context, input VoiceChatInput) (*VoiceChatResult, error) {
	started := time.Now()
	if s == nil || s.providerService == nil || s.chatService == nil {
		return nil, ErrInvalidInput
	}
	if input.TenantID == 0 || input.ProjectID == 0 || input.SessionID == 0 || input.UserID == 0 || strings.TrimSpace(input.AudioURL) == "" {
		return nil, ErrInvalidInput
	}
	s.logger.Info("voice chat started", voiceChatLogFields(input)...)

	transcript, err := s.providerService.Transcribe(ctx, ProviderTranscribeInput{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		AudioURL:  input.AudioURL,
		Language:  input.Language,
	})
	if err != nil {
		s.logger.Error("voice chat asr failed",
			append(voiceChatLogFields(input),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	content := strings.TrimSpace(transcript.Text)
	if content == "" {
		s.logger.Warn("voice chat empty transcript",
			append(voiceChatLogFields(input),
				zap.Duration("duration", time.Since(started)),
			)...,
		)
		return nil, ErrInvalidInput
	}
	s.logger.Info("voice chat asr finished",
		append(voiceChatLogFields(input),
			zap.Int("transcript_chars", len([]rune(content))),
			zap.String("transcript_hash", sqlHash(content)),
		)...,
	)

	chat, err := s.chatService.SendMessage(ctx, SendChatMessageInput{
		TenantID:              input.TenantID,
		ProjectID:             input.ProjectID,
		SessionID:             input.SessionID,
		UserID:                input.UserID,
		Content:               content,
		DatasourceID:          input.DatasourceID,
		SelectedDatasourceIDs: input.SelectedDatasourceIDs,
		MaxRows:               input.MaxRows,
		AutoExecute:           input.AutoExecute,
		RequestID:             input.RequestID,
		IP:                    input.IP,
		UserAgent:             input.UserAgent,
		AuditOrigin:           input.AuditOrigin,
	})
	if err != nil {
		s.logger.Error("voice chat question failed",
			append(voiceChatLogFields(input),
				zap.String("transcript_hash", sqlHash(content)),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}

	speechText := voiceSpeechText(chat, content)
	speech, err := s.providerService.Synthesize(ctx, ProviderSynthesizeInput{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		Text:      speechText,
		Voice:     input.Voice,
		Format:    input.Format,
	})
	if err != nil && !errors.Is(err, ErrProviderNotConfigured) {
		s.logger.Error("voice chat tts failed",
			append(voiceChatLogFields(input),
				zap.String("transcript_hash", sqlHash(content)),
				zap.Int("speech_text_chars", len([]rune(speechText))),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	if errors.Is(err, ErrProviderNotConfigured) {
		s.logger.Info("voice chat tts skipped",
			append(voiceChatLogFields(input),
				zap.String("reason", "provider_not_configured"),
			)...,
		)
	}
	s.logger.Info("voice chat finished",
		append(voiceChatLogFields(input),
			zap.String("transcript_hash", sqlHash(content)),
			zap.Int("speech_text_chars", len([]rune(speechText))),
			zap.Bool("tts_generated", speech != nil),
			zap.Duration("duration", time.Since(started)),
		)...,
	)

	return &VoiceChatResult{
		Transcript: transcript,
		Chat:       chat,
		Speech:     speech,
		SpeechText: speechText,
	}, nil
}

func (s *VoiceService) StreamChat(ctx context.Context, input VoiceChatInput, emit func(VoiceChatStreamEvent) error) (*VoiceChatResult, error) {
	started := time.Now()
	if s == nil || s.providerService == nil || s.chatService == nil || emit == nil {
		return nil, ErrInvalidInput
	}
	if input.TenantID == 0 || input.ProjectID == 0 || input.SessionID == 0 || input.UserID == 0 || strings.TrimSpace(input.AudioURL) == "" {
		return nil, ErrInvalidInput
	}
	s.logger.Info("voice stream chat started", voiceChatLogFields(input)...)
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "开始识别语音。"}); err != nil {
		return nil, err
	}

	transcript, err := s.streamTranscript(ctx, input, emit)
	if err != nil {
		s.logger.Error("voice stream chat asr failed",
			append(voiceChatLogFields(input),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	content := strings.TrimSpace(transcript.Text)
	if content == "" {
		s.logger.Warn("voice stream chat empty transcript",
			append(voiceChatLogFields(input),
				zap.Duration("duration", time.Since(started)),
			)...,
		)
		return nil, ErrInvalidInput
	}
	s.logger.Info("voice stream chat asr finished",
		append(voiceChatLogFields(input),
			zap.Int("transcript_chars", len([]rune(content))),
			zap.String("transcript_hash", sqlHash(content)),
		)...,
	)
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "语音识别完成，开始问数。"}); err != nil {
		return nil, err
	}

	chat, err := s.chatService.StreamMessage(ctx, SendChatMessageInput{
		TenantID:              input.TenantID,
		ProjectID:             input.ProjectID,
		SessionID:             input.SessionID,
		UserID:                input.UserID,
		Content:               content,
		DatasourceID:          input.DatasourceID,
		SelectedDatasourceIDs: input.SelectedDatasourceIDs,
		MaxRows:               input.MaxRows,
		AutoExecute:           input.AutoExecute,
		RequestID:             input.RequestID,
		IP:                    input.IP,
		UserAgent:             input.UserAgent,
		AuditOrigin:           input.AuditOrigin,
	}, func(event query.AgentEvent) error {
		return emit(VoiceChatStreamEvent{Stage: VoiceStreamStageChat, Agent: &event})
	})
	if err != nil {
		s.logger.Error("voice stream chat question failed",
			append(voiceChatLogFields(input),
				zap.String("transcript_hash", sqlHash(content)),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}

	speechText := voiceSpeechText(chat, content)
	speech, err := s.streamSpeech(ctx, input, speechText, emit)
	if err != nil && !errors.Is(err, ErrProviderNotConfigured) {
		s.logger.Error("voice stream chat tts failed",
			append(voiceChatLogFields(input),
				zap.String("transcript_hash", sqlHash(content)),
				zap.Int("speech_text_chars", len([]rune(speechText))),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	if errors.Is(err, ErrProviderNotConfigured) {
		if emitErr := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "语音播报未启用，已跳过。"}); emitErr != nil {
			return nil, emitErr
		}
		s.logger.Info("voice stream chat tts skipped",
			append(voiceChatLogFields(input),
				zap.String("reason", "provider_not_configured"),
			)...,
		)
	}
	result := &VoiceChatResult{
		Transcript: transcript,
		Chat:       chat,
		Speech:     speech,
		SpeechText: speechText,
	}
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "语音问数完成。", Done: true}); err != nil {
		return nil, err
	}
	s.logger.Info("voice stream chat finished",
		append(voiceChatLogFields(input),
			zap.String("transcript_hash", sqlHash(content)),
			zap.Int("speech_text_chars", len([]rune(speechText))),
			zap.Bool("tts_generated", speech != nil),
			zap.Duration("duration", time.Since(started)),
		)...,
	)
	return result, nil
}

func (s *VoiceService) StreamRealtimeChat(ctx context.Context, input RealtimeVoiceChatInput, audio <-chan []byte, emit func(VoiceChatStreamEvent) error) (*VoiceChatResult, error) {
	started := time.Now()
	if s == nil || s.providerService == nil || s.chatService == nil || audio == nil || emit == nil {
		return nil, ErrInvalidInput
	}
	if input.TenantID == 0 || input.ProjectID == 0 || input.SessionID == 0 || input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	s.logger.Info("voice realtime chat started", realtimeVoiceChatLogFields(input)...)
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "开始实时识别语音。"}); err != nil {
		return nil, err
	}

	transcript, err := s.streamRealtimeTranscript(ctx, input, audio, emit)
	if err != nil {
		s.logger.Error("voice realtime chat asr failed",
			append(realtimeVoiceChatLogFields(input),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	content := strings.TrimSpace(transcript.Text)
	if content == "" {
		s.logger.Warn("voice realtime chat empty transcript",
			append(realtimeVoiceChatLogFields(input),
				zap.Duration("duration", time.Since(started)),
			)...,
		)
		return nil, ErrInvalidInput
	}
	s.logger.Info("voice realtime chat asr finished",
		append(realtimeVoiceChatLogFields(input),
			zap.Int("transcript_chars", len([]rune(content))),
			zap.String("transcript_hash", sqlHash(content)),
		)...,
	)
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "实时识别完成，开始问数。"}); err != nil {
		return nil, err
	}

	chat, err := s.chatService.StreamMessage(ctx, SendChatMessageInput{
		TenantID:              input.TenantID,
		ProjectID:             input.ProjectID,
		SessionID:             input.SessionID,
		UserID:                input.UserID,
		Content:               content,
		DatasourceID:          input.DatasourceID,
		SelectedDatasourceIDs: input.SelectedDatasourceIDs,
		MaxRows:               input.MaxRows,
		AutoExecute:           input.AutoExecute,
		RequestID:             input.RequestID,
		IP:                    input.IP,
		UserAgent:             input.UserAgent,
		AuditOrigin:           input.AuditOrigin,
	}, func(event query.AgentEvent) error {
		return emit(VoiceChatStreamEvent{Stage: VoiceStreamStageChat, Agent: &event})
	})
	if err != nil {
		s.logger.Error("voice realtime chat question failed",
			append(realtimeVoiceChatLogFields(input),
				zap.String("transcript_hash", sqlHash(content)),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}

	speechText := voiceSpeechText(chat, content)
	speech, err := s.streamRealtimeSpeech(ctx, input, speechText, emit)
	if err != nil && !errors.Is(err, ErrProviderNotConfigured) {
		s.logger.Error("voice realtime chat tts failed",
			append(realtimeVoiceChatLogFields(input),
				zap.String("transcript_hash", sqlHash(content)),
				zap.Int("speech_text_chars", len([]rune(speechText))),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	if errors.Is(err, ErrProviderNotConfigured) {
		if emitErr := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "语音播报未启用，已跳过。"}); emitErr != nil {
			return nil, emitErr
		}
		s.logger.Info("voice realtime chat tts skipped",
			append(realtimeVoiceChatLogFields(input),
				zap.String("reason", "provider_not_configured"),
			)...,
		)
	}
	result := &VoiceChatResult{
		Transcript: transcript,
		Chat:       chat,
		Speech:     speech,
		SpeechText: speechText,
	}
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "实时语音问数完成。", Done: true}); err != nil {
		return nil, err
	}
	s.logger.Info("voice realtime chat finished",
		append(realtimeVoiceChatLogFields(input),
			zap.String("transcript_hash", sqlHash(content)),
			zap.Int("speech_text_chars", len([]rune(speechText))),
			zap.Bool("tts_generated", speech != nil),
			zap.Duration("duration", time.Since(started)),
		)...,
	)
	return result, nil
}

func (s *VoiceService) streamTranscript(ctx context.Context, input VoiceChatInput, emit func(VoiceChatStreamEvent) error) (*asr.TranscribeResponse, error) {
	var (
		text      string
		lastEvent asr.TranscribeStreamEvent
	)
	err := s.providerService.StreamTranscribe(ctx, ProviderTranscribeInput{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		AudioURL:  input.AudioURL,
		Language:  input.Language,
	}, func(event asr.TranscribeStreamEvent) error {
		lastEvent = event
		if strings.TrimSpace(event.Text) != "" {
			text = strings.TrimSpace(event.Text)
		}
		return emit(VoiceChatStreamEvent{Stage: VoiceStreamStageASR, Transcript: &event, Done: event.Done})
	})
	if err != nil {
		return nil, err
	}
	return &asr.TranscribeResponse{
		TaskID:       lastEvent.TaskID,
		Status:       firstNonEmptyService(lastEvent.Status, "success"),
		Text:         text,
		ResultURL:    lastEvent.ResultURL,
		RawRequestID: lastEvent.RawRequestID,
	}, nil
}

func (s *VoiceService) streamRealtimeTranscript(ctx context.Context, input RealtimeVoiceChatInput, audio <-chan []byte, emit func(VoiceChatStreamEvent) error) (*asr.TranscribeResponse, error) {
	var (
		text      string
		lastEvent asr.TranscribeStreamEvent
	)
	err := s.providerService.StreamTranscribeAudio(ctx, ProviderTranscribeAudioInput{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		Language:  input.Language,
	}, audio, func(event asr.TranscribeStreamEvent) error {
		lastEvent = event
		if strings.TrimSpace(event.Text) != "" {
			text = strings.TrimSpace(event.Text)
		}
		return emit(VoiceChatStreamEvent{Stage: VoiceStreamStageASR, Transcript: &event, Done: event.Done})
	})
	if err != nil {
		return nil, err
	}
	return &asr.TranscribeResponse{
		TaskID:       lastEvent.TaskID,
		Status:       firstNonEmptyService(lastEvent.Status, "success"),
		Text:         text,
		ResultURL:    lastEvent.ResultURL,
		RawRequestID: lastEvent.RawRequestID,
	}, nil
}

func (s *VoiceService) streamSpeech(ctx context.Context, input VoiceChatInput, text string, emit func(VoiceChatStreamEvent) error) (*tts.SynthesizeResponse, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if err := emit(VoiceChatStreamEvent{Stage: VoiceStreamStageStatus, Message: "开始生成语音播报。"}); err != nil {
		return nil, err
	}
	var (
		chunks      []string
		audioURL    string
		contentType string
	)
	err := s.providerService.StreamSynthesize(ctx, ProviderSynthesizeInput{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		Text:      text,
		Voice:     input.Voice,
		Format:    input.Format,
	}, func(event tts.SynthesizeStreamEvent) error {
		if strings.TrimSpace(event.AudioBase64Chunk) != "" {
			chunks = append(chunks, event.AudioBase64Chunk)
		}
		if strings.TrimSpace(event.AudioURL) != "" {
			audioURL = strings.TrimSpace(event.AudioURL)
		}
		if strings.TrimSpace(event.ContentType) != "" {
			contentType = strings.TrimSpace(event.ContentType)
		}
		return emit(VoiceChatStreamEvent{Stage: VoiceStreamStageTTS, Speech: &event, Done: event.Done})
	})
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 && audioURL == "" && contentType == "" {
		return nil, nil
	}
	return &tts.SynthesizeResponse{
		AudioURL:    audioURL,
		AudioBase64: combineBase64AudioChunks(chunks),
		ContentType: contentType,
	}, nil
}

func combineBase64AudioChunks(chunks []string) string {
	if len(chunks) == 0 {
		return ""
	}
	merged := make([]byte, 0)
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(chunk)
		if err != nil {
			return strings.Join(chunks, "")
		}
		merged = append(merged, data...)
	}
	if len(merged) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(merged)
}

func (s *VoiceService) streamRealtimeSpeech(ctx context.Context, input RealtimeVoiceChatInput, text string, emit func(VoiceChatStreamEvent) error) (*tts.SynthesizeResponse, error) {
	return s.streamSpeech(ctx, VoiceChatInput{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		Voice:     input.Voice,
		Format:    input.Format,
	}, text, emit)
}

func voiceChatLogFields(input VoiceChatInput) []zap.Field {
	return []zap.Field{
		zap.String("request_id", input.RequestID),
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("session_id", input.SessionID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Uint64s("selected_datasource_ids", input.SelectedDatasourceIDs),
		zap.Int("selected_datasource_count", len(input.SelectedDatasourceIDs)),
		zap.Bool("auto_execute", input.AutoExecute),
		zap.Int("max_rows", input.MaxRows),
		zap.String("audio_url_hash", sqlHash(strings.TrimSpace(input.AudioURL))),
	}
}

func realtimeVoiceChatLogFields(input RealtimeVoiceChatInput) []zap.Field {
	return []zap.Field{
		zap.String("request_id", input.RequestID),
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("session_id", input.SessionID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Uint64s("selected_datasource_ids", input.SelectedDatasourceIDs),
		zap.Int("selected_datasource_count", len(input.SelectedDatasourceIDs)),
		zap.Bool("auto_execute", input.AutoExecute),
		zap.Int("max_rows", input.MaxRows),
	}
}

func voiceSpeechText(chat *SendChatMessageResult, _ string) string {
	if text := cleanVoiceSpeechText(primaryExecutionSpeechText(chat)); text != "" {
		return text
	}
	if text := cleanVoiceSpeechText(agentSpeechText(chat)); text != "" {
		return text
	}
	if chat != nil && chat.Agent != nil && !chat.Agent.Review.Passed && strings.TrimSpace(chat.Agent.Review.BlockedReason) != "" {
		return "这个问题生成的 SQL 没有通过安全审核：" + strings.TrimSpace(chat.Agent.Review.BlockedReason)
	}
	return "这次没有生成可播报的结果。"
}

func primaryExecutionSpeechText(chat *SendChatMessageResult) string {
	if chat == nil || chat.Execution == nil {
		return ""
	}
	return firstNonEmptyService(chat.Execution.Answer, chat.Execution.SpeechSummary)
}

func agentSpeechText(chat *SendChatMessageResult) string {
	if chat == nil || chat.Agent == nil {
		return ""
	}
	switch chat.Agent.Intent {
	case query.AgentIntentChat, query.AgentIntentClarify:
		return firstNonEmptyService(chat.Agent.Answer, chat.Agent.Explanation)
	default:
		return chat.Agent.Answer
	}
}

func cleanVoiceSpeechText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || hasUnresolvedTemplatePlaceholder(text) {
		return ""
	}
	parts := splitSpeechSentences(text)
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = removeChartRecommendationClause(part)
		part = strings.TrimSpace(strings.Trim(part, "。!！?？;；"))
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return strings.Join(cleaned, "。") + "。"
}

func splitSpeechSentences(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '。', '!', '！', '?', '？', ';', '；', '\n', '\r':
			return true
		default:
			return false
		}
	})
	return fields
}

func removeChartRecommendationClause(sentence string) string {
	sentence = strings.TrimSpace(sentence)
	if sentence == "" || !containsChartTerm(sentence) {
		return sentence
	}
	leadingMarkers := []string{"建议使用", "推荐使用", "建议用", "推荐用", "适合使用", "适合用"}
	for _, marker := range leadingMarkers {
		if strings.HasPrefix(sentence, marker) {
			return ""
		}
	}
	markers := []string{"，建议使用", "，推荐使用", "，建议用", "，推荐用", "，适合使用", "，适合用", "，适合", "，可以使用", "，可使用"}
	for _, marker := range markers {
		if index := strings.Index(sentence, marker); index >= 0 && containsChartTerm(sentence[index:]) {
			return strings.TrimSpace(sentence[:index])
		}
	}
	return sentence
}

func containsChartTerm(text string) bool {
	terms := []string{"图表", "饼图", "柱状图", "折线图", "漏斗图", "雷达图", "图展示", "图对比"}
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}
