package asr

import "context"

const ProviderAliyun = "aliyun"

type Provider interface {
	Name() string
	Configured() bool
	DefaultModel() string
	Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error)
	StreamTranscribe(ctx context.Context, req TranscribeRequest, onEvent func(TranscribeStreamEvent) error) error
	GetTask(ctx context.Context, taskID string) (*TranscribeResponse, error)
}

type AudioStreamProvider interface {
	StreamTranscribeAudio(ctx context.Context, req TranscribeRequest, audio <-chan []byte, onEvent func(TranscribeStreamEvent) error) error
}

type TranscribeRequest struct {
	Model    string `json:"model,omitempty"`
	AudioURL string `json:"audio_url"`
	Language string `json:"language,omitempty"`
}

type TranscribeResponse struct {
	TaskID       string `json:"task_id,omitempty"`
	Status       string `json:"status,omitempty"`
	Text         string `json:"text,omitempty"`
	ResultURL    string `json:"result_url,omitempty"`
	RawRequestID string `json:"raw_request_id,omitempty"`
}

type TranscribeStreamEvent struct {
	TaskID       string           `json:"task_id,omitempty"`
	RawRequestID string           `json:"raw_request_id,omitempty"`
	Event        string           `json:"event,omitempty"`
	Status       string           `json:"status,omitempty"`
	StatusCode   int              `json:"status_code,omitempty"`
	Text         string           `json:"text,omitempty"`
	ResultURL    string           `json:"result_url,omitempty"`
	Index        int              `json:"index,omitempty"`
	Time         int              `json:"time,omitempty"`
	BeginTime    int              `json:"begin_time,omitempty"`
	Confidence   float64          `json:"confidence,omitempty"`
	Words        []TranscribeWord `json:"words,omitempty"`
	Done         bool             `json:"done"`
}

type TranscribeWord struct {
	Text      string `json:"text,omitempty"`
	StartTime int    `json:"startTime,omitempty"`
	EndTime   int    `json:"endTime,omitempty"`
}
