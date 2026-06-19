package asr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ling-shu/internal/aliyun/nls"
)

var (
	ErrNotConfigured        = errors.New("provider nls token credentials or app key is not configured")
	ErrTaskQueryUnsupported = errors.New("aliyun nls realtime asr does not support task query")
)

const (
	speechTranscriberNamespace = "SpeechTranscriber"
	startTranscriptionName     = "StartTranscription"
	stopTranscriptionName      = "StopTranscription"
	transcriptionStartedName   = "TranscriptionStarted"
	resultChangedName          = "TranscriptionResultChanged"
	sentenceEndName            = "SentenceEnd"
	transcriptionCompletedName = "TranscriptionCompleted"
	taskFailedName             = "TaskFailed"
)

type AliyunConfig struct {
	Token                          string
	AccessKeyID                    string
	AccessKeySecret                string
	TokenEndpoint                  string
	TokenRegionID                  string
	TokenRefreshBefore             time.Duration
	AppKey                         string
	WebsocketURL                   string
	Model                          string
	Format                         string
	SampleRate                     int
	EnableIntermediateResult       bool
	EnablePunctuationPrediction    bool
	EnableInverseTextNormalization bool
	EnableWords                    bool
	Timeout                        time.Duration
}

type AliyunProvider struct {
	tokenProvider                  nls.TokenProvider
	appKey                         string
	websocketURL                   string
	model                          string
	format                         string
	sampleRate                     int
	enableIntermediateResult       bool
	enablePunctuationPrediction    bool
	enableInverseTextNormalization bool
	enableWords                    bool
	timeout                        time.Duration
	client                         *http.Client
	dial                           func(ctx context.Context) (*nls.Client, error)
}

func NewAliyunProvider(cfg AliyunConfig) *AliyunProvider {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
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
		model = "nls-realtime-asr"
	}
	format := strings.ToLower(cfg.Format)
	if format == "" {
		format = "pcm"
	}
	sampleRate := cfg.SampleRate
	if sampleRate <= 0 {
		sampleRate = 16000
	}

	provider := &AliyunProvider{
		tokenProvider:                  tokenProvider,
		appKey:                         cfg.AppKey,
		websocketURL:                   websocketURL,
		model:                          model,
		format:                         format,
		sampleRate:                     sampleRate,
		enableIntermediateResult:       cfg.EnableIntermediateResult,
		enablePunctuationPrediction:    cfg.EnablePunctuationPrediction,
		enableInverseTextNormalization: cfg.EnableInverseTextNormalization,
		enableWords:                    cfg.EnableWords,
		timeout:                        timeout,
		client:                         &http.Client{Timeout: timeout},
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

func (p *AliyunProvider) Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error) {
	if !p.Configured() {
		return nil, ErrNotConfigured
	}
	if strings.TrimSpace(req.AudioURL) == "" {
		return nil, errors.New("audio_url is required")
	}

	response := &TranscribeResponse{}
	segments := make([]string, 0, 8)
	latestText := ""
	err := p.StreamTranscribe(ctx, req, func(event TranscribeStreamEvent) error {
		if event.TaskID != "" {
			response.TaskID = event.TaskID
		}
		if event.Done {
			response.Status = "COMPLETED"
		} else if event.Status != "" {
			response.Status = event.Status
		}
		if event.RawRequestID != "" {
			response.RawRequestID = event.RawRequestID
		}
		if event.Text != "" {
			latestText = event.Text
		}
		if event.Event == sentenceEndName && event.Text != "" {
			segments = append(segments, event.Text)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(segments) > 0 {
		response.Text = strings.Join(segments, "")
	} else {
		response.Text = latestText
	}
	return response, nil
}

func (p *AliyunProvider) StreamTranscribe(ctx context.Context, req TranscribeRequest, onEvent func(TranscribeStreamEvent) error) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	if onEvent == nil {
		return errors.New("on_event is required")
	}
	if strings.TrimSpace(req.AudioURL) == "" {
		return errors.New("audio_url is required")
	}

	audio, err := p.openAudio(ctx, req.AudioURL)
	if err != nil {
		return err
	}
	defer audio.Close()

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
	if err := client.WriteJSON(ctx, p.startTranscriptionMessage(req, taskID)); err != nil {
		return err
	}

	audioErrCh := make(chan error, 1)
	audioStarted := false
	startAudio := func() {
		if audioStarted {
			return
		}
		audioStarted = true
		go func() {
			err := p.sendAudioAndStop(ctx, client, audio, taskID)
			if err != nil {
				_ = client.Close()
			}
			audioErrCh <- err
		}()
	}

	for {
		select {
		case audioErr := <-audioErrCh:
			if audioErr != nil {
				return audioErr
			}
		default:
		}

		messageType, data, err := client.ReadFrame(ctx)
		if err != nil {
			select {
			case audioErr := <-audioErrCh:
				if audioErr != nil {
					return audioErr
				}
			default:
			}
			return err
		}
		if !nls.IsTextMessage(messageType) {
			continue
		}

		message, err := nls.ParseInbound(data)
		if err != nil {
			return err
		}
		event, err := transcribeEventFromMessage(message)
		if err != nil {
			return err
		}

		switch message.Header.Name {
		case transcriptionStartedName:
			if err := onEvent(event); err != nil {
				return err
			}
			startAudio()
		case resultChangedName, sentenceEndName:
			if err := onEvent(event); err != nil {
				return err
			}
		case transcriptionCompletedName:
			event.Done = true
			return onEvent(event)
		case taskFailedName:
			return nls.HeaderError(message.Header)
		default:
			if event.Event != "" {
				if err := onEvent(event); err != nil {
					return err
				}
			}
		}
	}
}

func (p *AliyunProvider) StreamTranscribeAudio(ctx context.Context, req TranscribeRequest, audio <-chan []byte, onEvent func(TranscribeStreamEvent) error) error {
	if !p.Configured() {
		return ErrNotConfigured
	}
	if onEvent == nil {
		return errors.New("on_event is required")
	}
	if audio == nil {
		return errors.New("audio stream is required")
	}

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
	if err := client.WriteJSON(ctx, p.startTranscriptionMessage(req, taskID)); err != nil {
		return err
	}

	audioErrCh := make(chan error, 1)
	audioStarted := false
	startAudio := func() {
		if audioStarted {
			return
		}
		audioStarted = true
		go func() {
			err := p.sendAudioChunksAndStop(ctx, client, audio, taskID)
			if err != nil {
				_ = client.Close()
			}
			audioErrCh <- err
		}()
	}

	for {
		select {
		case audioErr := <-audioErrCh:
			if audioErr != nil {
				return audioErr
			}
		default:
		}

		messageType, data, err := client.ReadFrame(ctx)
		if err != nil {
			select {
			case audioErr := <-audioErrCh:
				if audioErr != nil {
					return audioErr
				}
			default:
			}
			return err
		}
		if !nls.IsTextMessage(messageType) {
			continue
		}

		message, err := nls.ParseInbound(data)
		if err != nil {
			return err
		}
		event, err := transcribeEventFromMessage(message)
		if err != nil {
			return err
		}

		switch message.Header.Name {
		case transcriptionStartedName:
			if err := onEvent(event); err != nil {
				return err
			}
			startAudio()
		case resultChangedName, sentenceEndName:
			if err := onEvent(event); err != nil {
				return err
			}
		case transcriptionCompletedName:
			event.Done = true
			return onEvent(event)
		case taskFailedName:
			return nls.HeaderError(message.Header)
		default:
			if event.Event != "" {
				if err := onEvent(event); err != nil {
					return err
				}
			}
		}
	}
}

func (p *AliyunProvider) GetTask(ctx context.Context, taskID string) (*TranscribeResponse, error) {
	return nil, ErrTaskQueryUnsupported
}

func (p *AliyunProvider) openAudio(ctx context.Context, audioURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create audio request: %w", err)
	}
	req.Header.Set("Accept", "*/*")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download audio: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		return nil, fmt.Errorf("download audio failed: status=%d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (p *AliyunProvider) sendAudioAndStop(ctx context.Context, client *nls.Client, audio io.Reader, taskID string) error {
	chunkSize := p.audioChunkSize()
	buf := make([]byte, chunkSize)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		n, readErr := audio.Read(buf)
		if n > 0 {
			if err := client.WriteBinary(ctx, buf[:n]); err != nil {
				return err
			}
			if readErr == nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read audio: %w", readErr)
		}
	}

	return client.WriteJSON(ctx, nls.NewControlMessage(p.appKey, speechTranscriberNamespace, stopTranscriptionName, taskID, nil))
}

func (p *AliyunProvider) sendAudioChunksAndStop(ctx context.Context, client *nls.Client, audio <-chan []byte, taskID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-audio:
			if !ok {
				return client.WriteJSON(ctx, nls.NewControlMessage(p.appKey, speechTranscriberNamespace, stopTranscriptionName, taskID, nil))
			}
			if len(chunk) == 0 {
				continue
			}
			if err := client.WriteBinary(ctx, chunk); err != nil {
				return err
			}
		}
	}
}

func (p *AliyunProvider) audioChunkSize() int {
	if p.format == "pcm" || p.format == "wav" {
		size := p.sampleRate * 2 / 10
		if size >= 1024 {
			return size
		}
	}
	return 16 * 1024
}

func (p *AliyunProvider) startTranscriptionMessage(req TranscribeRequest, taskID string) nls.OutboundMessage {
	payload := map[string]any{
		"format":                            p.format,
		"sample_rate":                       p.sampleRate,
		"enable_intermediate_result":        p.enableIntermediateResult,
		"enable_punctuation_prediction":     p.enablePunctuationPrediction,
		"enable_inverse_text_normalization": p.enableInverseTextNormalization,
	}
	if p.enableWords {
		payload["enable_words"] = true
	}
	if req.Language != "" {
		payload["language_hints"] = []string{req.Language}
	}
	return nls.NewControlMessage(p.appKey, speechTranscriberNamespace, startTranscriptionName, taskID, payload)
}

func transcribeEventFromMessage(message *nls.InboundMessage) (TranscribeStreamEvent, error) {
	event := TranscribeStreamEvent{
		TaskID:       message.Header.TaskID,
		RawRequestID: message.Header.TaskID,
		Event:        message.Header.Name,
		Status:       nls.HeaderStatusText(message.Header),
		StatusCode:   message.Header.Status,
	}
	if len(message.Payload) == 0 {
		return event, nil
	}

	var payload transcriptionPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		return event, err
	}
	event.Text = payload.Result
	event.Index = payload.Index
	event.Time = payload.Time
	event.BeginTime = payload.BeginTime
	event.Confidence = payload.Confidence
	event.Words = payload.Words
	return event, nil
}

type transcriptionPayload struct {
	Index      int              `json:"index"`
	Time       int              `json:"time"`
	BeginTime  int              `json:"begin_time"`
	Result     string           `json:"result"`
	Confidence float64          `json:"confidence"`
	Words      []TranscribeWord `json:"words"`
}
