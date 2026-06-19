package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"ling-shu/internal/aliyun/nls"
)

var ErrNotConfigured = errors.New("provider nls token credentials or app key is not configured")

const (
	speechSynthesizerNamespace = "SpeechSynthesizer"
	startSynthesisName         = "StartSynthesis"
	synthesisCompletedName     = "SynthesisCompleted"
	taskFailedName             = "TaskFailed"
)

type AliyunConfig struct {
	Token              string
	AccessKeyID        string
	AccessKeySecret    string
	TokenEndpoint      string
	TokenRegionID      string
	TokenRefreshBefore time.Duration
	AppKey             string
	WebsocketURL       string
	Model              string
	Voice              string
	Format             string
	SampleRate         int
	Volume             int
	SpeechRate         int
	PitchRate          int
	EnableSubtitle     bool
	Timeout            time.Duration
}

type AliyunProvider struct {
	tokenProvider  nls.TokenProvider
	appKey         string
	websocketURL   string
	model          string
	voice          string
	format         string
	sampleRate     int
	volume         int
	speechRate     int
	pitchRate      int
	enableSubtitle bool
	timeout        time.Duration
	dial           func(ctx context.Context) (*nls.Client, error)
}

func NewAliyunProvider(cfg AliyunConfig) *AliyunProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	tokenProvider := nls.NewTokenProvider(nls.TokenProviderConfig{
		StaticToken:     cfg.Token,
		AccessKeyID:     cfg.AccessKeyID,
		AccessKeySecret: cfg.AccessKeySecret,
		Endpoint:        cfg.TokenEndpoint,
		RegionID:        cfg.TokenRegionID,
		RefreshBefore:   cfg.TokenRefreshBefore,
		Timeout:         timeout,
	})
	websocketURL := cfg.WebsocketURL
	if websocketURL == "" {
		websocketURL = nls.DefaultWebsocketURL
	}
	model := cfg.Model
	if model == "" {
		model = "nls-tts"
	}
	voice := cfg.Voice
	if voice == "" {
		voice = "aixia"
	}
	format := strings.ToLower(cfg.Format)
	if format == "" {
		format = "mp3"
	}
	sampleRate := cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	volume := cfg.Volume
	if volume <= 0 {
		volume = 50
	}

	provider := &AliyunProvider{
		tokenProvider:  tokenProvider,
		appKey:         cfg.AppKey,
		websocketURL:   websocketURL,
		model:          model,
		voice:          voice,
		format:         format,
		sampleRate:     sampleRate,
		volume:         volume,
		speechRate:     cfg.SpeechRate,
		pitchRate:      cfg.PitchRate,
		enableSubtitle: cfg.EnableSubtitle,
		timeout:        timeout,
	}
	provider.dial = func(ctx context.Context) (*nls.Client, error) {
		token, err := provider.tokenProvider.Token(ctx)
		if err != nil {
			return nil, err
		}
		return nls.Dial(ctx, provider.websocketURL, token, provider.timeout)
	}
	return provider
}

func (p *AliyunProvider) Name() string {
	return ProviderAliyun
}

func (p *AliyunProvider) Configured() bool {
	return strings.TrimSpace(p.appKey) != "" && p.tokenProvider != nil && p.tokenProvider.Configured()
}

func (p *AliyunProvider) DefaultModel() string {
	return p.model
}

func (p *AliyunProvider) DefaultVoice() string {
	return p.voice
}

func (p *AliyunProvider) DefaultFormat() string {
	return p.format
}

func (p *AliyunProvider) Synthesize(ctx context.Context, req SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("text is required")
	}

	var audio bytes.Buffer
	response := &SynthesizeResponse{ContentType: contentTypeForFormat(p.requestFormat(req))}
	err := p.streamSynthesis(ctx, req, func(event SynthesizeStreamEvent, audioChunk []byte) error {
		if event.TaskID != "" {
			response.RawRequestID = event.TaskID
		}
		if event.ContentType != "" {
			response.ContentType = event.ContentType
		}
		if len(audioChunk) > 0 {
			_, _ = audio.Write(audioChunk)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if audio.Len() > 0 {
		response.AudioBase64 = base64.StdEncoding.EncodeToString(audio.Bytes())
	}
	return response, nil
}

func (p *AliyunProvider) StreamSynthesize(ctx context.Context, req SynthesizeRequest, onEvent func(SynthesizeStreamEvent) error) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	if onEvent == nil {
		return errors.New("on_event is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return errors.New("text is required")
	}

	return p.streamSynthesis(ctx, req, func(event SynthesizeStreamEvent, audioChunk []byte) error {
		if len(audioChunk) > 0 {
			event.AudioBase64Chunk = base64.StdEncoding.EncodeToString(audioChunk)
		}
		return onEvent(event)
	})
}

func (p *AliyunProvider) streamSynthesis(ctx context.Context, req SynthesizeRequest, onEvent func(SynthesizeStreamEvent, []byte) error) error {
	client, err := p.dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	stopClose := context.AfterFunc(ctx, func() {
		_ = client.Close()
	})
	defer stopClose()

	taskID := nls.NewID()
	format := p.requestFormat(req)
	if err := client.WriteJSON(ctx, p.startSynthesisMessage(req, taskID)); err != nil {
		return err
	}

	for {
		messageType, data, err := client.ReadFrame(ctx)
		if err != nil {
			return err
		}
		if nls.IsBinaryMessage(messageType) {
			if len(data) == 0 {
				continue
			}
			if err := onEvent(SynthesizeStreamEvent{
				TaskID:      taskID,
				Event:       "AudioChunk",
				ContentType: contentTypeForFormat(format),
			}, data); err != nil {
				return err
			}
			continue
		}
		if !nls.IsTextMessage(messageType) {
			continue
		}

		message, err := nls.ParseInbound(data)
		if err != nil {
			return err
		}
		event := synthesizeEventFromMessage(message, contentTypeForFormat(format))
		switch message.Header.Name {
		case synthesisCompletedName:
			event.Done = true
			return onEvent(event, nil)
		case taskFailedName:
			return nls.HeaderError(message.Header)
		default:
			if err := onEvent(event, nil); err != nil {
				return err
			}
		}
	}
}

func (p *AliyunProvider) startSynthesisMessage(req SynthesizeRequest, taskID string) nls.OutboundMessage {
	payload := map[string]any{
		"text":        req.Text,
		"voice":       p.requestVoice(req),
		"format":      p.requestFormat(req),
		"sample_rate": p.sampleRate,
		"volume":      p.volume,
		"speech_rate": p.speechRate,
		"pitch_rate":  p.pitchRate,
	}
	if p.enableSubtitle {
		payload["enable_subtitle"] = true
	}
	return nls.NewControlMessage(p.appKey, speechSynthesizerNamespace, startSynthesisName, taskID, payload)
}

func (p *AliyunProvider) requestVoice(req SynthesizeRequest) string {
	if req.Voice != "" {
		return req.Voice
	}
	return p.voice
}

func (p *AliyunProvider) requestFormat(req SynthesizeRequest) string {
	if req.Format != "" {
		return strings.ToLower(req.Format)
	}
	return p.format
}

func synthesizeEventFromMessage(message *nls.InboundMessage, contentType string) SynthesizeStreamEvent {
	return SynthesizeStreamEvent{
		TaskID:      message.Header.TaskID,
		Event:       message.Header.Name,
		Status:      nls.HeaderStatusText(message.Header),
		StatusCode:  message.Header.Status,
		ContentType: contentType,
	}
}

func contentTypeForFormat(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "pcm":
		return "audio/pcm"
	case "mp3":
		return "audio/mpeg"
	default:
		return fmt.Sprintf("audio/%s", strings.ToLower(format))
	}
}
