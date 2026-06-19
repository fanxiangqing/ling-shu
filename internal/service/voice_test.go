package service

import (
	"context"
	"strings"
	"testing"

	"ling-shu/internal/asr"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
	"ling-shu/internal/tts"
)

func TestVoiceServiceChatRunsASRChatAndTTS(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "今天销售额是多少",
			SQL:          "select sum(amount) from orders LIMIT 200",
			Explanation:  "统计订单金额",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select sum(amount) from orders LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 99, Status: "success"},
			Answer:    "今天销售额是 100 元。",
			Rows:      []map[string]any{{"sales_amount": 100}},
		},
	}
	chatService := NewChatService(chatRepo, agent, executor)
	asrProvider := &voiceFakeASRProvider{text: "今天销售额是多少"}
	ttsProvider := &voiceFakeTTSProvider{audioURL: "https://example.com/answer.mp3"}
	providerService := NewProviderService(serviceFakeLLMProvider{chatModel: "qwen-plus", configured: true}, asrProvider, ttsProvider)
	service := NewVoiceService(providerService, chatService)

	result, err := service.Chat(context.Background(), VoiceChatInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		AudioURL:     "https://example.com/question.wav",
		AutoExecute:  true,
		DatasourceID: 7,
		Voice:        "xiaoyun",
		Format:       "mp3",
	})
	if err != nil {
		t.Fatalf("voice chat: %v", err)
	}
	if result.Transcript.Text != "今天销售额是多少" {
		t.Fatalf("unexpected transcript: %+v", result.Transcript)
	}
	if agent.lastInput.Question != "今天销售额是多少" {
		t.Fatalf("expected transcript as chat question, got %s", agent.lastInput.Question)
	}
	if executor.lastInput.SQL != "select sum(amount) from orders LIMIT 200" {
		t.Fatalf("unexpected executed sql: %s", executor.lastInput.SQL)
	}
	if ttsProvider.lastRequest.Text != "今天销售额是 100 元。" {
		t.Fatalf("expected tts to speak final answer, got %s", ttsProvider.lastRequest.Text)
	}
	if result.Speech == nil || result.Speech.AudioURL != "https://example.com/answer.mp3" {
		t.Fatalf("unexpected speech response: %+v", result.Speech)
	}
}

func TestVoiceServiceChatAllowsTTSDisabled(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "订单数",
			SQL:          "select count(*) from orders LIMIT 200",
			Explanation:  "统计订单数",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from orders LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 100, Status: "success"},
			Answer:    "订单数是 10。",
		},
	}
	chatService := NewChatService(chatRepo, agent, executor)
	providerService := NewProviderService(serviceFakeLLMProvider{chatModel: "qwen-plus", configured: true}, &voiceFakeASRProvider{text: "订单数"}, nil)
	service := NewVoiceService(providerService, chatService)

	result, err := service.Chat(context.Background(), VoiceChatInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		AudioURL:     "https://example.com/question.wav",
		AutoExecute:  true,
		DatasourceID: 7,
	})
	if err != nil {
		t.Fatalf("voice chat with disabled tts: %v", err)
	}
	if result.Speech != nil {
		t.Fatalf("expected no speech when tts is disabled, got %+v", result.Speech)
	}
	if result.SpeechText != "订单数是 10。" {
		t.Fatalf("expected speech text to still be prepared, got %s", result.SpeechText)
	}
}

func TestVoiceServiceStreamChatRunsASRChatAndTTS(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "今天销售额是多少",
			SQL:          "select sum(amount) from orders LIMIT 200",
			Explanation:  "统计订单金额",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select sum(amount) from orders LIMIT 200"},
			Steps: []query.AgentEvent{
				{Type: query.EventAction, Step: 1, Name: "llm.text2sql", Content: "生成查询 SQL"},
			},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 99, Status: "success"},
			Answer:    "今天销售额是 100 元。",
			Rows:      []map[string]any{{"sales_amount": 100}},
		},
	}
	chatService := NewChatService(chatRepo, agent, executor)
	asrProvider := &voiceFakeASRProvider{
		streamEvents: []asr.TranscribeStreamEvent{
			{Event: "TranscriptionResultChanged", Text: "今天销售额", Done: false},
			{Event: "TranscriptionCompleted", Text: "今天销售额是多少", Status: "success", Done: true},
		},
	}
	ttsProvider := &voiceFakeTTSProvider{
		streamEvents: []tts.SynthesizeStreamEvent{
			{AudioBase64Chunk: "YW5z", ContentType: "audio/mpeg"},
			{AudioBase64Chunk: "d2Vy", ContentType: "audio/mpeg", Done: true},
		},
	}
	providerService := NewProviderService(serviceFakeLLMProvider{chatModel: "qwen-plus", configured: true}, asrProvider, ttsProvider)
	service := NewVoiceService(providerService, chatService)

	var events []VoiceChatStreamEvent
	result, err := service.StreamChat(context.Background(), VoiceChatInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		AudioURL:     "https://example.com/question.wav",
		AutoExecute:  true,
		DatasourceID: 7,
		Voice:        "xiaoyun",
		Format:       "mp3",
	}, func(event VoiceChatStreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("stream voice chat: %v", err)
	}
	if result.Transcript.Text != "今天销售额是多少" {
		t.Fatalf("unexpected transcript: %+v", result.Transcript)
	}
	if agent.lastInput.Question != "今天销售额是多少" {
		t.Fatalf("expected transcript as chat question, got %s", agent.lastInput.Question)
	}
	if ttsProvider.lastRequest.Text != "今天销售额是 100 元。" {
		t.Fatalf("expected tts to speak final answer, got %s", ttsProvider.lastRequest.Text)
	}
	if result.Speech == nil || result.Speech.AudioBase64 != "YW5zd2Vy" || result.Speech.ContentType != "audio/mpeg" {
		t.Fatalf("unexpected streamed speech: %+v", result.Speech)
	}
	for _, stage := range []string{VoiceStreamStageStatus, VoiceStreamStageASR, VoiceStreamStageChat, VoiceStreamStageTTS} {
		if !hasVoiceStreamStage(events, stage) {
			t.Fatalf("expected stream stage %s in %+v", stage, events)
		}
	}
}

func TestVoiceServiceStreamChatAllowsTTSDisabled(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "订单数",
			SQL:          "select count(*) from orders LIMIT 200",
			Explanation:  "统计订单数",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from orders LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 100, Status: "success"},
			Answer:    "订单数是 10。",
		},
	}
	chatService := NewChatService(chatRepo, agent, executor)
	providerService := NewProviderService(serviceFakeLLMProvider{chatModel: "qwen-plus", configured: true}, &voiceFakeASRProvider{text: "订单数"}, nil)
	service := NewVoiceService(providerService, chatService)

	var events []VoiceChatStreamEvent
	result, err := service.StreamChat(context.Background(), VoiceChatInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		AudioURL:     "https://example.com/question.wav",
		AutoExecute:  true,
		DatasourceID: 7,
	}, func(event VoiceChatStreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("stream voice chat with disabled tts: %v", err)
	}
	if result.Speech != nil {
		t.Fatalf("expected no speech when tts is disabled, got %+v", result.Speech)
	}
	if !hasVoiceStatusMessage(events, "语音播报未启用") {
		t.Fatalf("expected tts skipped status event, got %+v", events)
	}
}

func TestVoiceServiceStreamRealtimeChatRunsASRChatAndTTS(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "今天销售额是多少",
			SQL:          "select sum(amount) from orders LIMIT 200",
			Explanation:  "统计订单金额",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select sum(amount) from orders LIMIT 200"},
			Steps: []query.AgentEvent{
				{Type: query.EventAction, Step: 1, Name: "llm.text2sql", Content: "生成查询 SQL"},
			},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 99, Status: "success"},
			Answer:    "今天销售额是 100 元。",
			Rows:      []map[string]any{{"sales_amount": 100}},
		},
	}
	chatService := NewChatService(chatRepo, agent, executor)
	asrProvider := &voiceFakeASRProvider{
		streamEvents: []asr.TranscribeStreamEvent{
			{Event: "TranscriptionStarted", Status: "success"},
			{Event: "TranscriptionCompleted", Text: "今天销售额是多少", Status: "success", Done: true},
		},
	}
	ttsProvider := &voiceFakeTTSProvider{
		streamEvents: []tts.SynthesizeStreamEvent{
			{AudioBase64Chunk: "MTAw", ContentType: "audio/mpeg", Done: true},
		},
	}
	providerService := NewProviderService(serviceFakeLLMProvider{chatModel: "qwen-plus", configured: true}, asrProvider, ttsProvider)
	service := NewVoiceService(providerService, chatService)

	audio := make(chan []byte, 2)
	audio <- []byte{1, 2, 3}
	audio <- []byte{4, 5, 6}
	close(audio)

	var events []VoiceChatStreamEvent
	result, err := service.StreamRealtimeChat(context.Background(), RealtimeVoiceChatInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		AutoExecute:  true,
		DatasourceID: 7,
		Voice:        "xiaoyun",
		Format:       "mp3",
	}, audio, func(event VoiceChatStreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("realtime voice chat: %v", err)
	}
	if result.Transcript.Text != "今天销售额是多少" {
		t.Fatalf("unexpected transcript: %+v", result.Transcript)
	}
	if agent.lastInput.Question != "今天销售额是多少" {
		t.Fatalf("expected transcript as chat question, got %s", agent.lastInput.Question)
	}
	if len(asrProvider.audioChunks) != 2 {
		t.Fatalf("expected two audio chunks, got %d", len(asrProvider.audioChunks))
	}
	if result.Speech == nil || result.Speech.AudioBase64 != "MTAw" || result.Speech.ContentType != "audio/mpeg" {
		t.Fatalf("unexpected streamed speech: %+v", result.Speech)
	}
	for _, stage := range []string{VoiceStreamStageStatus, VoiceStreamStageASR, VoiceStreamStageChat, VoiceStreamStageTTS} {
		if !hasVoiceStreamStage(events, stage) {
			t.Fatalf("expected stream stage %s in %+v", stage, events)
		}
	}
}

func TestVoiceSpeechTextUsesVisibleExecutionAnswer(t *testing.T) {
	text := voiceSpeechText(&SendChatMessageResult{
		Agent: &query.AgentResult{
			Intent:      query.AgentIntentQuery,
			Answer:      "当前项目中活跃成员用户数为：{user_count}。",
			Explanation: "当前项目中活跃成员用户数为：{user_count}。",
			Review:      query.ReviewResult{Passed: true},
		},
		Execution: &QueryExecutionResult{
			Answer:        "查询已完成，返回 3 行数据，已按饼图展示。",
			SpeechSummary: "共返回 3 行数据，第一行是：survey_type=online，count=934。",
		},
	}, "我想知道问卷按类型分布情况是什么")

	if text != "查询已完成，返回 3 行数据，已按饼图展示。" {
		t.Fatalf("expected visible execution answer to be spoken, got %q", text)
	}
}

func TestVoiceSpeechTextSkipsQuestionFallback(t *testing.T) {
	text := voiceSpeechText(&SendChatMessageResult{}, "当前项目有多少用户")

	if strings.Contains(text, "当前项目有多少用户") || strings.Contains(text, "已收到你的问题") {
		t.Fatalf("expected no question replay fallback, got %q", text)
	}
}

func TestVoiceSpeechTextRemovesChartRecommendation(t *testing.T) {
	text := voiceSpeechText(&SendChatMessageResult{
		Agent: &query.AgentResult{
			Intent: "multi_query",
			Answer: "灵数数据库用户数为 2080，问卷数据库用户数为 1200，建议使用柱状图对比。",
			Review: query.ReviewResult{Passed: true},
		},
	}, "")

	if text != "灵数数据库用户数为 2080，问卷数据库用户数为 1200。" {
		t.Fatalf("expected chart recommendation to be removed, got %q", text)
	}
}

func TestVoiceSpeechTextKeepsRenderedChartStatement(t *testing.T) {
	text := voiceSpeechText(&SendChatMessageResult{
		Execution: &QueryExecutionResult{
			Answer: "查询已完成，返回 3 行数据，已按饼图展示。",
		},
	}, "")

	if text != "查询已完成，返回 3 行数据，已按饼图展示。" {
		t.Fatalf("expected rendered chart statement to be kept, got %q", text)
	}
}

type voiceFakeASRProvider struct {
	text         string
	streamEvents []asr.TranscribeStreamEvent
	audioChunks  [][]byte
	lastRequest  asr.TranscribeRequest
}

func (p *voiceFakeASRProvider) Name() string         { return "aliyun" }
func (p *voiceFakeASRProvider) Configured() bool     { return true }
func (p *voiceFakeASRProvider) DefaultModel() string { return "nls-realtime-asr" }
func (p *voiceFakeASRProvider) Transcribe(ctx context.Context, req asr.TranscribeRequest) (*asr.TranscribeResponse, error) {
	p.lastRequest = req
	return &asr.TranscribeResponse{Status: "success", Text: p.text}, nil
}
func (p *voiceFakeASRProvider) StreamTranscribe(ctx context.Context, req asr.TranscribeRequest, onEvent func(asr.TranscribeStreamEvent) error) error {
	p.lastRequest = req
	if len(p.streamEvents) > 0 {
		for _, event := range p.streamEvents {
			if err := onEvent(event); err != nil {
				return err
			}
		}
		return nil
	}
	return onEvent(asr.TranscribeStreamEvent{Text: p.text, Done: true})
}
func (p *voiceFakeASRProvider) StreamTranscribeAudio(ctx context.Context, req asr.TranscribeRequest, audio <-chan []byte, onEvent func(asr.TranscribeStreamEvent) error) error {
	p.lastRequest = req
	for chunk := range audio {
		p.audioChunks = append(p.audioChunks, append([]byte(nil), chunk...))
	}
	if len(p.streamEvents) > 0 {
		for _, event := range p.streamEvents {
			if err := onEvent(event); err != nil {
				return err
			}
		}
		return nil
	}
	return onEvent(asr.TranscribeStreamEvent{Text: p.text, Done: true})
}
func (p *voiceFakeASRProvider) GetTask(ctx context.Context, taskID string) (*asr.TranscribeResponse, error) {
	return &asr.TranscribeResponse{TaskID: taskID, Status: "success", Text: p.text}, nil
}

type voiceFakeTTSProvider struct {
	audioURL     string
	streamEvents []tts.SynthesizeStreamEvent
	lastRequest  tts.SynthesizeRequest
}

func (p *voiceFakeTTSProvider) Name() string         { return "aliyun" }
func (p *voiceFakeTTSProvider) Configured() bool     { return true }
func (p *voiceFakeTTSProvider) DefaultModel() string { return "nls-tts" }
func (p *voiceFakeTTSProvider) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (*tts.SynthesizeResponse, error) {
	p.lastRequest = req
	return &tts.SynthesizeResponse{AudioURL: p.audioURL, ContentType: "audio/mpeg"}, nil
}
func (p *voiceFakeTTSProvider) StreamSynthesize(ctx context.Context, req tts.SynthesizeRequest, onEvent func(tts.SynthesizeStreamEvent) error) error {
	p.lastRequest = req
	if len(p.streamEvents) > 0 {
		for _, event := range p.streamEvents {
			if err := onEvent(event); err != nil {
				return err
			}
		}
		return nil
	}
	return onEvent(tts.SynthesizeStreamEvent{AudioURL: p.audioURL, ContentType: "audio/mpeg", Done: true})
}

func hasVoiceStreamStage(events []VoiceChatStreamEvent, stage string) bool {
	for _, event := range events {
		if event.Stage == stage {
			return true
		}
	}
	return false
}

func hasVoiceStatusMessage(events []VoiceChatStreamEvent, messagePart string) bool {
	for _, event := range events {
		if event.Stage == VoiceStreamStageStatus && strings.Contains(event.Message, messagePart) {
			return true
		}
	}
	return false
}
