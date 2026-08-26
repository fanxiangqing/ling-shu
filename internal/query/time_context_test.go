package query

import (
	"testing"
	"time"
)

func TestNewAgentTimeContextUsesRequestedTimezone(t *testing.T) {
	now := time.Date(2026, 8, 26, 6, 39, 24, 0, time.UTC)
	context := NewAgentTimeContext(now, "Asia/Shanghai", "UTC")
	if context.CurrentTime != "2026-08-26 14:39:24" || context.UTCOffset != "+08:00" {
		t.Fatalf("unexpected Shanghai context: %+v", context)
	}
	if context.Timezone != "Asia/Shanghai" || context.Weekday != "星期三" {
		t.Fatalf("unexpected timezone metadata: %+v", context)
	}
}

func TestNewAgentTimeContextRespectsDSTAndFallback(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	context := NewAgentTimeContext(now, "America/New_York", "Asia/Shanghai")
	if context.CurrentTime != "2026-08-26 08:00:00" || context.UTCOffset != "-04:00" {
		t.Fatalf("expected New York daylight time, got %+v", context)
	}
	fallback := NewAgentTimeContext(now, "invalid/timezone", "Asia/Shanghai")
	if fallback.Timezone != "Asia/Shanghai" || fallback.UTCOffset != "+08:00" {
		t.Fatalf("expected configured fallback timezone, got %+v", fallback)
	}
}
