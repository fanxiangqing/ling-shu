package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	maxRowsPattern = regexp.MustCompile(`(?i)(?:最多|上限|限制|返回)\s*(\d{1,4})\s*(?:行|条)?`)
	phonePattern   = regexp.MustCompile(`(?:^|\D)1[3-9]\d{9}(?:\D|$)`)
	idCardPattern  = regexp.MustCompile(`(?:^|\D)\d{17}[0-9Xx](?:\D|$)`)
	ianaPattern    = regexp.MustCompile(`[A-Za-z_]+(?:/[A-Za-z0-9_+\-]+)+`)
)

func NormalizeUserMemoryOperation(operation UserMemoryOperation, question string) UserMemoryOperation {
	operation.Action = strings.ToLower(strings.TrimSpace(operation.Action))
	operation.Kind = normalizeUserMemoryKind(operation.Kind)
	operation.Key = strings.TrimSpace(operation.Key)
	operation.Content = compactUserMemoryText(operation.Content, 1000)
	operation.ScopeLevel = strings.ToLower(strings.TrimSpace(operation.ScopeLevel))
	if operation.ScopeLevel != UserMemoryScopeTenant {
		operation.ScopeLevel = UserMemoryScopeProject
	}
	if operation.Confidence <= 0 || operation.Confidence > 1 {
		operation.Confidence = 1
	}
	if operation.Action == UserMemoryActionRemember && operation.Content == "" {
		operation.Content = explicitMemoryContent(question)
	}
	if operation.Action == UserMemoryActionRemember {
		known := detectKnownPreference(operation.Content)
		if operation.Key == "" {
			operation.Key = known.Key
		}
		if len(operation.Value) == 0 {
			operation.Value = known.Value
		}
		if operation.Kind == UserMemoryKindPreference && known.Kind != "" {
			operation.Kind = known.Kind
		}
	}
	return operation
}

func normalizeUserMemoryKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case UserMemoryKindProfile:
		return UserMemoryKindProfile
	case UserMemoryKindConvention:
		return UserMemoryKindConvention
	case UserMemoryKindInstruction:
		return UserMemoryKindInstruction
	case UserMemoryKindCorrection:
		return UserMemoryKindCorrection
	default:
		return UserMemoryKindPreference
	}
}

func UserMemorySensitivity(content string) string {
	lower := strings.ToLower(content)
	restrictedWords := []string{
		"密码", "口令", "token", "api key", "apikey", "secret", "私钥", "private key",
		"身份证", "银行卡", "信用卡", "手机号", "电话号码", "邮箱密码",
	}
	for _, word := range restrictedWords {
		if strings.Contains(lower, word) {
			return UserMemorySensitivityRestricted
		}
	}
	if phonePattern.MatchString(content) || idCardPattern.MatchString(content) {
		return UserMemorySensitivityRestricted
	}
	if strings.Contains(lower, "住址") || strings.Contains(lower, "家庭地址") || strings.Contains(lower, "健康") {
		return UserMemorySensitivitySensitive
	}
	return UserMemorySensitivityNormal
}

func ValidateUserMemoryOperation(operation UserMemoryOperation) error {
	switch operation.Action {
	case UserMemoryActionRemember:
		if strings.TrimSpace(operation.Content) == "" {
			return fmt.Errorf("记忆内容不能为空")
		}
		if UserMemorySensitivity(operation.Content) == UserMemorySensitivityRestricted {
			return fmt.Errorf("这类敏感信息不能保存为长期记忆")
		}
		if containsPromptOverride(operation.Content) {
			return fmt.Errorf("包含系统规则覆盖指令的内容不能保存为长期记忆")
		}
	case UserMemoryActionForget, UserMemoryActionConfirm, UserMemoryActionReject:
		if operation.TargetID == 0 && strings.TrimSpace(operation.Content) == "" && strings.TrimSpace(operation.Key) == "" {
			return fmt.Errorf("请说明要操作哪条记忆")
		}
	case UserMemoryActionList, UserMemoryActionClear:
		return nil
	default:
		return fmt.Errorf("不支持的记忆操作")
	}
	return nil
}

func BuildUserMemory(scope UserScope, operation UserMemoryOperation, sourceType string, sessionID uint64, messageID uint64, now time.Time) UserMemory {
	if operation.ScopeLevel == UserMemoryScopeTenant {
		scope.ProjectID = 0
	}
	status := UserMemoryStatusActive
	confidence := operation.Confidence
	if confidence <= 0 {
		confidence = 1
	}
	key := strings.TrimSpace(operation.Key)
	canonical := strings.ToLower(strings.TrimSpace(operation.Content))
	if key != "" {
		canonical = operation.Kind + ":" + strings.ToLower(key)
	}
	item := UserMemory{
		Scope: scope, Kind: normalizeUserMemoryKind(operation.Kind), Key: key,
		CanonicalHash: userMemoryHash(canonical), Content: operation.Content,
		Value: operation.Value, Applicability: map[string]any{}, Status: status,
		SourceType: sourceType, Confidence: confidence, Salience: 0.7,
		Sensitivity: UserMemorySensitivity(operation.Content), EvidenceCount: 1, Version: 1,
		ObservedAt: &now, ValidFrom: &now, SourceSessionID: sessionID, SourceMessageID: messageID,
		ExpiresAt: operation.ExpiresAt,
	}
	if sourceType == UserMemorySourceExplicit || sourceType == UserMemorySourceConfirmed {
		item.LastConfirmedAt = &now
	}
	return item
}

func userMemoryHash(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func containsPromptOverride(content string) bool {
	lower := strings.ToLower(content)
	patterns := []string{"忽略系统", "忽略以上", "覆盖系统", "ignore system", "ignore previous", "system prompt", "越权"}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func compactUserMemoryText(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

type knownUserPreference struct {
	Kind  string
	Key   string
	Value map[string]any
}

func detectKnownPreference(content string) knownUserPreference {
	lower := strings.ToLower(content)
	charts := []struct {
		words []string
		value string
	}{
		{[]string{"柱状图", "条形图", "bar"}, "bar"},
		{[]string{"折线图", "line"}, "line"},
		{[]string{"饼图", "pie"}, "pie"},
		{[]string{"表格", "table"}, "table"},
	}
	for _, chart := range charts {
		for _, word := range chart.words {
			if strings.Contains(lower, word) && (strings.Contains(lower, "默认") || strings.Contains(lower, "以后") || strings.Contains(lower, "优先")) {
				return knownUserPreference{Kind: UserMemoryKindPreference, Key: "visualization.default_type", Value: map[string]any{"value": chart.value}}
			}
		}
	}
	if strings.Contains(lower, "简洁") || strings.Contains(lower, "简短") {
		return knownUserPreference{Kind: UserMemoryKindPreference, Key: "response.detail_level", Value: map[string]any{"value": "concise"}}
	}
	if strings.Contains(lower, "详细") || strings.Contains(lower, "展开") {
		return knownUserPreference{Kind: UserMemoryKindPreference, Key: "response.detail_level", Value: map[string]any{"value": "detailed"}}
	}
	timezoneAliases := []struct {
		words []string
		value string
	}{
		{[]string{"北京时间", "上海时区", "asia/shanghai"}, "Asia/Shanghai"},
		{[]string{"纽约时间", "纽约时区", "america/new_york"}, "America/New_York"},
		{[]string{"伦敦时间", "伦敦时区", "europe/london"}, "Europe/London"},
		{[]string{"东京时间", "东京时区", "asia/tokyo"}, "Asia/Tokyo"},
	}
	for _, timezone := range timezoneAliases {
		for _, word := range timezone.words {
			if strings.Contains(lower, word) {
				return knownUserPreference{Kind: UserMemoryKindPreference, Key: "time.timezone", Value: map[string]any{"value": timezone.value}}
			}
		}
	}
	if candidate := ianaPattern.FindString(content); candidate != "" {
		if _, err := time.LoadLocation(candidate); err == nil {
			return knownUserPreference{Kind: UserMemoryKindPreference, Key: "time.timezone", Value: map[string]any{"value": candidate}}
		}
	}
	if matches := maxRowsPattern.FindStringSubmatch(content); len(matches) == 2 {
		if value, err := strconv.Atoi(matches[1]); err == nil && value > 0 && value <= 1000 {
			return knownUserPreference{Kind: UserMemoryKindPreference, Key: "query.max_rows", Value: map[string]any{"value": value}}
		}
	}
	if strings.Contains(lower, "我负责") || strings.Contains(lower, "我的职责") || strings.Contains(lower, "我是") {
		return knownUserPreference{Kind: UserMemoryKindProfile}
	}
	if strings.Contains(lower, "我说") || strings.Contains(lower, "对我来说") || strings.Contains(lower, "我的习惯") {
		return knownUserPreference{Kind: UserMemoryKindConvention}
	}
	return knownUserPreference{}
}
