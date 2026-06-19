package tts

import "context"

const ProviderAliyun = "aliyun"

type Provider interface {
	Name() string
	Configured() bool
	DefaultModel() string
	Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error)
	StreamSynthesize(ctx context.Context, req SynthesizeRequest, onEvent func(SynthesizeStreamEvent) error) error
}

type SynthesizeRequest struct {
	Model  string `json:"model,omitempty"`
	Text   string `json:"text"`
	Voice  string `json:"voice,omitempty"`
	Format string `json:"format,omitempty"`
}

type SynthesizeResponse struct {
	AudioURL     string `json:"audio_url,omitempty"`
	AudioBase64  string `json:"audio_base64,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	RawRequestID string `json:"raw_request_id,omitempty"`
}

type SynthesizeStreamEvent struct {
	AudioBase64Chunk string `json:"audio_base64_chunk,omitempty"`
	AudioURL         string `json:"audio_url,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
	TaskID           string `json:"task_id,omitempty"`
	Event            string `json:"event,omitempty"`
	Status           string `json:"status,omitempty"`
	StatusCode       int    `json:"status_code,omitempty"`
	Done             bool   `json:"done"`
}
