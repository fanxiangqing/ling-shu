package query

import (
	"strings"
	"time"
)

const DefaultAgentTimezone = "Asia/Shanghai"

var chineseWeekdays = map[time.Weekday]string{
	time.Sunday: "星期日", time.Monday: "星期一", time.Tuesday: "星期二", time.Wednesday: "星期三",
	time.Thursday: "星期四", time.Friday: "星期五", time.Saturday: "星期六",
}

func NewAgentTimeContext(now time.Time, requestedTimezone string, fallbackTimezone string) AgentTimeContext {
	timezone, location := resolveAgentLocation(requestedTimezone, fallbackTimezone)
	local := now.In(location)
	_, offsetSeconds := local.Zone()
	return AgentTimeContext{
		CurrentTime: local.Format("2006-01-02 15:04:05"),
		Timezone:    timezone,
		UTCOffset:   formatUTCOffset(offsetSeconds),
		UTCTime:     now.UTC().Format(time.RFC3339),
		Weekday:     chineseWeekdays[local.Weekday()],
	}
}

func ValidAgentTimezone(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	_, err := time.LoadLocation(value)
	return err == nil
}

func resolveAgentLocation(requested string, fallback string) (string, *time.Location) {
	for _, value := range []string{requested, fallback, DefaultAgentTimezone, "UTC"} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if location, err := time.LoadLocation(value); err == nil {
			return value, location
		}
	}
	return "UTC", time.UTC
}

func formatUTCOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return sign + twoDigits(hours) + ":" + twoDigits(minutes)
}

func twoDigits(value int) string {
	return string([]byte{'0' + byte(value/10), '0' + byte(value%10)})
}
