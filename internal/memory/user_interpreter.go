package memory

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type UserMemoryInterpreter interface {
	Interpret(ctx context.Context, scope UserScope, question string, now time.Time, timezone string) (UserMemoryOperation, error)
}

type RuleUserMemoryInterpreter struct{}

func NewRuleUserMemoryInterpreter() RuleUserMemoryInterpreter {
	return RuleUserMemoryInterpreter{}
}

func (RuleUserMemoryInterpreter) Interpret(_ context.Context, _ UserScope, question string, _ time.Time, _ string) (UserMemoryOperation, error) {
	text := compactUserMemoryText(question, 1000)
	lower := strings.ToLower(text)
	scopeLevel := UserMemoryScopeProject
	if containsAny(lower, "所有项目", "全部项目", "跨项目", "任何项目") {
		scopeLevel = UserMemoryScopeTenant
	}
	switch {
	case containsAny(lower, "你记得我什么", "你记住了什么", "查看我的记忆", "列出我的记忆", "我的长期记忆"):
		return UserMemoryOperation{Action: UserMemoryActionList, ScopeLevel: scopeLevel, Confidence: 1}, nil
	case containsAny(lower, "清除所有记忆", "清空所有记忆", "忘掉关于我的一切"):
		return UserMemoryOperation{Action: UserMemoryActionClear, ScopeLevel: UserMemoryScopeTenant, Confidence: 1}, nil
	case containsAny(lower, "清除这个项目的记忆", "清空项目记忆", "忘掉这个项目里的记忆"):
		return UserMemoryOperation{Action: UserMemoryActionClear, ScopeLevel: UserMemoryScopeProject, Confidence: 1}, nil
	case hasRememberIntent(lower):
		content := explicitMemoryContent(text)
		preference := detectKnownPreference(content)
		return NormalizeUserMemoryOperation(UserMemoryOperation{
			Action: UserMemoryActionRemember, Kind: preference.Kind, Key: preference.Key,
			Content: content, Value: preference.Value, ScopeLevel: scopeLevel, Confidence: 1,
		}, question), nil
	case hasForgetIntent(lower):
		return UserMemoryOperation{
			Action: UserMemoryActionForget, Content: forgetMemoryTarget(text),
			ScopeLevel: scopeLevel, Confidence: 1,
		}, nil
	default:
		return UserMemoryOperation{Action: UserMemoryActionNone}, nil
	}
}

func hasRememberIntent(value string) bool {
	if containsAny(value, "记住", "帮我记住") {
		return true
	}
	return strings.Contains(value, "以后") && containsAny(value, "默认", "都用", "都按", "回答", "展示", "返回")
}

func hasForgetIntent(value string) bool {
	return containsAny(value, "忘记", "忘掉", "删除这条记忆", "删除我的")
}

func explicitMemoryContent(value string) string {
	replacer := strings.NewReplacer(
		"请记住", "", "帮我记住", "", "记住：", "", "记住:", "", "记住", "",
		"在所有项目中", "", "所有项目里", "", "所有项目", "",
	)
	content := strings.TrimSpace(replacer.Replace(value))
	return strings.Trim(content, "，。；;：: ")
}

func forgetMemoryTarget(value string) string {
	replacer := strings.NewReplacer(
		"请忘记", "", "忘记", "", "忘掉", "", "删除这条记忆", "", "删除我的", "",
		"所有项目里的", "", "这个项目里的", "",
	)
	return strings.Trim(replacer.Replace(value), "，。；;：: ")
}

func ParseUserMemoryOperationJSON(raw string) (UserMemoryOperation, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = regexp.MustCompile("(?s)^```(?:json)?\\s*|\\s*```$").ReplaceAllString(raw, "")
	}
	var operation UserMemoryOperation
	err := json.Unmarshal([]byte(raw), &operation)
	return operation, err
}
