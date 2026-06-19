package tts

import (
	"encoding/json"
	"testing"

	"ling-shu/internal/aliyun/nls"
)

func TestStartSynthesisMessage(t *testing.T) {
	provider := NewAliyunProvider(AliyunConfig{
		Token:          "token",
		AppKey:         "app-key",
		Voice:          "xiaoyun",
		Format:         "mp3",
		SampleRate:     16000,
		Volume:         60,
		SpeechRate:     120,
		PitchRate:      -30,
		EnableSubtitle: true,
	})

	message := provider.startSynthesisMessage(SynthesizeRequest{
		Text:   "今天销售额是多少？",
		Voice:  "aixia",
		Format: "wav",
	}, "task-1")
	if message.Header.AppKey != "app-key" {
		t.Fatalf("expected app key in header")
	}
	if message.Header.Namespace != speechSynthesizerNamespace {
		t.Fatalf("expected namespace %s, got %s", speechSynthesizerNamespace, message.Header.Namespace)
	}
	if message.Header.Name != startSynthesisName {
		t.Fatalf("expected name %s, got %s", startSynthesisName, message.Header.Name)
	}
	if message.Header.TaskID != "task-1" {
		t.Fatalf("expected task id")
	}

	payload, ok := message.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload")
	}
	assertPayloadValue(t, payload, "text", "今天销售额是多少？")
	assertPayloadValue(t, payload, "voice", "aixia")
	assertPayloadValue(t, payload, "format", "wav")
	assertPayloadValue(t, payload, "sample_rate", 16000)
	assertPayloadValue(t, payload, "volume", 60)
	assertPayloadValue(t, payload, "speech_rate", 120)
	assertPayloadValue(t, payload, "pitch_rate", -30)
	assertPayloadValue(t, payload, "enable_subtitle", true)
}

func TestSynthesizeEventFromCompletedMessage(t *testing.T) {
	payload := json.RawMessage(`{}`)
	event := synthesizeEventFromMessage(&nls.InboundMessage{
		Header: nls.Header{
			Namespace:     speechSynthesizerNamespace,
			Name:          synthesisCompletedName,
			TaskID:        "task-1",
			Status:        nls.StatusSuccess,
			StatusMessage: "GATEWAY|SUCCESS|Success.",
		},
		Payload: payload,
	}, "audio/mpeg")
	if event.Event != synthesisCompletedName {
		t.Fatalf("expected completed event, got %s", event.Event)
	}
	if event.TaskID != "task-1" {
		t.Fatalf("expected task id")
	}
	if event.ContentType != "audio/mpeg" {
		t.Fatalf("expected content type")
	}
	if event.Status != "GATEWAY|SUCCESS|Success." {
		t.Fatalf("expected status text, got %s", event.Status)
	}
}

func assertPayloadValue(t *testing.T, payload map[string]any, key string, expected any) {
	t.Helper()
	if payload[key] != expected {
		t.Fatalf("expected payload[%s]=%v, got %v", key, expected, payload[key])
	}
}
