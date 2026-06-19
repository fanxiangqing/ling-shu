package asr

import (
	"encoding/json"
	"testing"

	"ling-shu/internal/aliyun/nls"
)

func TestStartTranscriptionMessage(t *testing.T) {
	provider := NewAliyunProvider(AliyunConfig{
		Token:                          "token",
		AppKey:                         "app-key",
		Format:                         "pcm",
		SampleRate:                     16000,
		EnableIntermediateResult:       true,
		EnablePunctuationPrediction:    true,
		EnableInverseTextNormalization: true,
		EnableWords:                    true,
	})

	message := provider.startTranscriptionMessage(TranscribeRequest{Language: "zh"}, "task-1")
	if message.Header.AppKey != "app-key" {
		t.Fatalf("expected app key in header")
	}
	if message.Header.Namespace != speechTranscriberNamespace {
		t.Fatalf("expected namespace %s, got %s", speechTranscriberNamespace, message.Header.Namespace)
	}
	if message.Header.Name != startTranscriptionName {
		t.Fatalf("expected name %s, got %s", startTranscriptionName, message.Header.Name)
	}
	if message.Header.TaskID != "task-1" {
		t.Fatalf("expected task id")
	}

	payload, ok := message.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload")
	}
	assertPayloadValue(t, payload, "format", "pcm")
	assertPayloadValue(t, payload, "sample_rate", 16000)
	assertPayloadValue(t, payload, "enable_intermediate_result", true)
	assertPayloadValue(t, payload, "enable_punctuation_prediction", true)
	assertPayloadValue(t, payload, "enable_inverse_text_normalization", true)
	assertPayloadValue(t, payload, "enable_words", true)
}

func TestTranscribeEventFromSentenceEndMessage(t *testing.T) {
	payload := json.RawMessage(`{
		"index": 1,
		"time": 1820,
		"begin_time": 0,
		"result": "北京的天气。",
		"confidence": 0.98,
		"words": [{"text": "北京", "startTime": 630, "endTime": 930}]
	}`)
	event, err := transcribeEventFromMessage(&nls.InboundMessage{
		Header: nls.Header{
			Namespace:  speechTranscriberNamespace,
			Name:       sentenceEndName,
			TaskID:     "task-1",
			Status:     nls.StatusSuccess,
			StatusText: "Gateway:SUCCESS:Success.",
		},
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("parse event: %v", err)
	}
	if event.Event != sentenceEndName {
		t.Fatalf("expected sentence end event, got %s", event.Event)
	}
	if event.Text != "北京的天气。" {
		t.Fatalf("expected text, got %q", event.Text)
	}
	if event.Confidence != 0.98 {
		t.Fatalf("expected confidence, got %f", event.Confidence)
	}
	if len(event.Words) != 1 || event.Words[0].Text != "北京" {
		t.Fatalf("expected word timing")
	}
}

func assertPayloadValue(t *testing.T, payload map[string]any, key string, expected any) {
	t.Helper()
	if payload[key] != expected {
		t.Fatalf("expected payload[%s]=%v, got %v", key, expected, payload[key])
	}
}
