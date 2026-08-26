package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ling-shu/internal/llm"
)

type UserMemoryLLMResolver func(ctx context.Context, scope UserScope) (llm.Provider, error)

type LLMUserMemoryInterpreter struct {
	resolver UserMemoryLLMResolver
	fallback UserMemoryInterpreter
}

func NewLLMUserMemoryInterpreter(resolver UserMemoryLLMResolver) *LLMUserMemoryInterpreter {
	return &LLMUserMemoryInterpreter{resolver: resolver, fallback: NewRuleUserMemoryInterpreter()}
}

func (i *LLMUserMemoryInterpreter) Interpret(ctx context.Context, scope UserScope, question string, now time.Time, timezone string) (UserMemoryOperation, error) {
	fallback, fallbackErr := i.fallback.Interpret(ctx, scope, question, now, timezone)
	if fallback.Handled() || !likelyUserMemorySignal(question) || i == nil || i.resolver == nil {
		return fallback, fallbackErr
	}
	provider, err := i.resolver(ctx, scope)
	if err != nil || provider == nil || !provider.Configured() {
		return fallback, fallbackErr
	}
	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: userMemoryInterpretPrompt(now, timezone)},
			{Role: "user", Content: question},
		},
		Temperature: float64Ptr(0), MaxTokens: 500,
	})
	if err != nil || resp == nil {
		return fallback, fallbackErr
	}
	operation, err := ParseUserMemoryOperationJSON(resp.Content)
	if err != nil {
		return fallback, fallbackErr
	}
	return NormalizeUserMemoryOperation(operation, question), nil
}

type UserMemoryExtractor interface {
	Extract(ctx context.Context, input UserTurnInput) ([]UserMemoryOperation, error)
}

type LLMUserMemoryExtractor struct {
	resolver UserMemoryLLMResolver
}

func NewLLMUserMemoryExtractor(resolver UserMemoryLLMResolver) *LLMUserMemoryExtractor {
	return &LLMUserMemoryExtractor{resolver: resolver}
}

func (e *LLMUserMemoryExtractor) Extract(ctx context.Context, input UserTurnInput) ([]UserMemoryOperation, error) {
	if e == nil || e.resolver == nil {
		return nil, nil
	}
	provider, err := e.resolver(ctx, input.Scope)
	if err != nil || provider == nil || !provider.Configured() {
		return nil, err
	}
	now := time.Now()
	prompt := userMemoryExtractionPrompt(now, input.Timezone)
	content := "用户：" + compactUserMemoryText(input.Question, 1500)
	if !input.OccurredAt.IsZero() {
		content = "对话发生时间：" + input.OccurredAt.Format(time.RFC3339) + "\n" + content
	}
	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Messages:    []llm.Message{{Role: "system", Content: prompt}, {Role: "user", Content: content}},
		Temperature: float64Ptr(0), MaxTokens: 800,
	})
	if err != nil || resp == nil {
		return nil, err
	}
	var payload struct {
		Memories []UserMemoryOperation `json:"memories"`
	}
	raw := strings.TrimSpace(resp.Content)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	out := make([]UserMemoryOperation, 0, len(payload.Memories))
	for _, operation := range payload.Memories {
		operation.Action = UserMemoryActionRemember
		operation.ScopeLevel = UserMemoryScopeProject
		operation = NormalizeUserMemoryOperation(operation, input.Question)
		if operation.Confidence < 0.65 || ValidateUserMemoryOperation(operation) != nil {
			continue
		}
		out = append(out, operation)
		if len(out) == 3 {
			break
		}
	}
	return out, nil
}

func likelyUserMemorySignal(question string) bool {
	lower := strings.ToLower(question)
	return containsAny(lower, "记", "忘", "以后", "默认", "偏好", "习惯", "我负责", "我的职责", "你知道我")
}

func userMemoryInterpretPrompt(now time.Time, timezone string) string {
	return fmt.Sprintf(`你是用户长期记忆命令解释器。当前时间=%s，用户时区=%s。
只判断用户是否明确要求记住、忘记、列出、清空、确认或拒绝记忆。普通问数必须返回 action=none。
输出单个 JSON，不要 Markdown：
{"action":"none|remember|forget|list|clear|confirm|reject","target_id":0,"kind":"preference|profile|convention|instruction|correction","memory_key":"","content":"","value":{},"scope_level":"project|tenant","confidence":0.0}
项目级是默认；只有用户明确说所有项目或跨项目时才使用 tenant。不要把查询结果、SQL 数值、项目公共指标或敏感凭据保存为记忆。`, now.Format(time.RFC3339), timezone)
}

func userMemoryExtractionPrompt(now time.Time, timezone string) string {
	return fmt.Sprintf(`你是用户长期记忆候选提取器。当前时间=%s，用户时区=%s。
从当前一轮对话中提取最多 3 条稳定的用户个人信息、职责、偏好或长期约定。查询结果、实时数值、临时筛选、SQL、项目公共知识和助手推测都不能提取。
所有结果只是 candidate，不会立即生效。输出 JSON，不要 Markdown：
{"memories":[{"kind":"preference|profile|convention|instruction|correction","memory_key":"","content":"","value":{},"confidence":0.0}]}
没有合适内容时返回 {"memories":[]}。`, now.Format(time.RFC3339), timezone)
}

func float64Ptr(value float64) *float64 {
	return &value
}
