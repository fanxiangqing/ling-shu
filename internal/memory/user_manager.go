package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	userMemoryRecallLimit = 8
	episodeRetention      = 90 * 24 * time.Hour
)

type UserManager struct {
	store       UserMemoryStore
	interpreter UserMemoryInterpreter
	semantic    UserMemorySemanticIndex
}

func NewUserManager(store UserMemoryStore, interpreter UserMemoryInterpreter) *UserManager {
	if interpreter == nil {
		interpreter = NewRuleUserMemoryInterpreter()
	}
	return &UserManager{store: store, interpreter: interpreter}
}

func (m *UserManager) SetSemanticIndex(index UserMemorySemanticIndex) {
	if m != nil {
		m.semantic = index
	}
}

func (m *UserManager) Prepare(ctx context.Context, scope UserScope, question string, now time.Time, timezone string) (UserMemoryContext, error) {
	prepared := UserMemoryContext{}
	if m == nil || !scope.Valid() || scope.ProjectID == 0 {
		return prepared, nil
	}
	operation, operationErr := m.interpreter.Interpret(ctx, scope, question, now, timezone)
	if operationErr == nil {
		prepared.Operation = NormalizeUserMemoryOperation(operation, question)
	}
	if m.store == nil {
		return prepared, operationErr
	}
	memories, err := m.store.ListApplicable(ctx, scope, now, 100)
	if err != nil {
		return prepared, firstUserMemoryError(operationErr, err)
	}
	prepared.Memories = memories
	var semanticScores map[uint64]float32
	if m.semantic != nil {
		semanticScores, _ = m.semantic.Recall(ctx, scope, question, userMemoryRecallLimit*2)
	}
	prepared.PromptItems, prepared.AppliedIDs = selectRelevantUserMemories(question, memories, semanticScores, now, userMemoryRecallLimit)
	for _, item := range memories {
		if !item.IsUsable(now) {
			continue
		}
		value := userMemoryStringValue(item.Value)
		switch item.Key {
		case "visualization.default_type":
			if prepared.DefaultChart == "" {
				prepared.DefaultChart = value
			}
		case "response.detail_level":
			if prepared.DetailLevel == "" {
				prepared.DetailLevel = value
			}
		case "time.timezone":
			if prepared.Timezone == "" {
				prepared.Timezone = value
			}
		}
	}
	if refersToPastConversation(question) {
		prepared.Episodes, _ = m.store.ListEpisodes(ctx, scope, 5)
	}
	if len(prepared.AppliedIDs) > 0 {
		_ = m.store.MarkRecalled(ctx, scope, prepared.AppliedIDs, true)
	}
	return prepared, operationErr
}

func (m *UserManager) ExecuteOperation(ctx context.Context, scope UserScope, operation UserMemoryOperation, sessionID uint64, messageID uint64, now time.Time) (UserMemoryOperationResult, error) {
	if m == nil || m.store == nil {
		return UserMemoryOperationResult{}, errors.New("长期记忆存储不可用")
	}
	operation = NormalizeUserMemoryOperation(operation, operation.Content)
	if err := ValidateUserMemoryOperation(operation); err != nil {
		return UserMemoryOperationResult{}, err
	}
	switch operation.Action {
	case UserMemoryActionList:
		items, _, err := m.store.List(ctx, UserMemoryListFilter{Scope: scope, Statuses: []string{UserMemoryStatusActive, UserMemoryStatusCandidate}, Limit: 100})
		if err != nil {
			return UserMemoryOperationResult{}, err
		}
		return UserMemoryOperationResult{Answer: formatUserMemoryList(items), Memories: items}, nil
	case UserMemoryActionRemember:
		item := BuildUserMemory(scope, operation, UserMemorySourceExplicit, sessionID, messageID, now)
		evidence := UserMemoryEvidence{Scope: item.Scope, SessionID: sessionID, MessageID: messageID, EvidenceType: UserMemorySourceExplicit, EvidenceSummary: compactUserMemoryText(item.Content, 180), CreatedAt: now}
		event := UserMemoryEvent{Scope: item.Scope, Operation: "created", MemoryKey: item.Key, Metadata: map[string]any{"kind": item.Kind, "scope": item.ScopeLevel()}, ActorUserID: scope.UserID, CreatedAt: now}
		saved, err := m.store.Upsert(ctx, item, evidence, event)
		if err != nil {
			return UserMemoryOperationResult{}, err
		}
		m.enqueueVectorJob(ctx, saved, "memory_index", now)
		return UserMemoryOperationResult{Answer: fmt.Sprintf("已记住：%s（%s）", saved.Content, userMemoryScopeLabel(saved)), Memory: &saved}, nil
	case UserMemoryActionForget:
		target, err := m.resolveOperationTarget(ctx, scope, operation)
		if err != nil {
			return UserMemoryOperationResult{}, err
		}
		event := UserMemoryEvent{Scope: target.Scope, Operation: "deleted", MemoryKey: target.Key, Metadata: map[string]any{"kind": target.Kind}, ActorUserID: scope.UserID, CreatedAt: now}
		if err := m.store.Delete(ctx, scope, target.ID, event); err != nil {
			return UserMemoryOperationResult{}, err
		}
		m.enqueueVectorJob(ctx, target, "memory_delete_index", now)
		return UserMemoryOperationResult{Answer: "已忘记：" + target.Content, Deleted: 1}, nil
	case UserMemoryActionClear:
		includeTenant := operation.ScopeLevel == UserMemoryScopeTenant
		items, _, _ := m.store.List(ctx, UserMemoryListFilter{Scope: scope, Limit: 500})
		event := UserMemoryEvent{Scope: scope, Operation: "cleared", Metadata: map[string]any{"include_tenant": includeTenant}, ActorUserID: scope.UserID, CreatedAt: now}
		count, err := m.store.Clear(ctx, scope, includeTenant, event)
		if err != nil {
			return UserMemoryOperationResult{}, err
		}
		for _, item := range items {
			if item.Scope.ProjectID == scope.ProjectID || includeTenant {
				m.enqueueVectorJob(ctx, item, "memory_delete_index", now)
			}
		}
		return UserMemoryOperationResult{Answer: fmt.Sprintf("已清除 %d 条长期记忆。", count), Deleted: count}, nil
	case UserMemoryActionConfirm, UserMemoryActionReject:
		target, err := m.resolveOperationTarget(ctx, scope, operation)
		if err != nil {
			return UserMemoryOperationResult{}, err
		}
		status := UserMemoryStatusActive
		eventName := "confirmed"
		answer := "已确认记忆：" + target.Content
		if operation.Action == UserMemoryActionReject {
			status = UserMemoryStatusRevoked
			eventName = "rejected"
			answer = "已忽略候选记忆：" + target.Content
		}
		event := UserMemoryEvent{Scope: target.Scope, Operation: eventName, MemoryKey: target.Key, ActorUserID: scope.UserID, CreatedAt: now}
		updated, err := m.store.SetStatus(ctx, scope, target.ID, status, event)
		if err != nil {
			return UserMemoryOperationResult{}, err
		}
		jobType := "memory_index"
		if operation.Action == UserMemoryActionReject {
			jobType = "memory_delete_index"
		}
		m.enqueueVectorJob(ctx, updated, jobType, now)
		return UserMemoryOperationResult{Answer: answer, Memory: &updated}, nil
	default:
		return UserMemoryOperationResult{}, errors.New("不支持的记忆操作")
	}
}

func (m *UserManager) RecordTurn(ctx context.Context, input UserTurnInput) error {
	if m == nil || m.store == nil || !input.Scope.Valid() || input.Scope.ProjectID == 0 || input.SessionID == 0 {
		return nil
	}
	now := input.OccurredAt
	if now.IsZero() {
		now = time.Now()
	}
	expiresAt := now.Add(episodeRetention)
	sensitiveTurn := UserMemorySensitivity(input.Question) == UserMemorySensitivityRestricted
	summary := compactEpisodeSummary(input.Question, input.Intent)
	topics := inferEpisodeTopics(input.Question)
	if sensitiveTurn {
		summary = "该会话包含敏感输入，未保存可召回摘要。"
		topics = nil
	}
	episode := SessionEpisode{
		Scope: input.Scope, SessionID: input.SessionID,
		Summary: summary,
		Topics:  topics, QueryExecutionIDs: append([]uint64(nil), input.QueryExecutionIDs...),
		Metadata: map[string]any{"intent": input.Intent, "timezone": input.Timezone}, ExpiresAt: &expiresAt,
	}
	if _, err := m.store.UpsertEpisode(ctx, episode); err != nil {
		return err
	}
	if sensitiveTurn {
		return nil
	}
	if candidate, ok := inferredMemoryCandidate(input.Question); ok {
		item := BuildUserMemory(input.Scope, candidate, UserMemorySourceInferred, input.SessionID, input.UserMessageID, now)
		evidence := UserMemoryEvidence{Scope: item.Scope, SessionID: input.SessionID, MessageID: input.UserMessageID, EvidenceType: UserMemorySourceInferred, EvidenceSummary: compactUserMemoryText(input.Question, 180), CreatedAt: now}
		event := UserMemoryEvent{Scope: item.Scope, Operation: "candidate", MemoryKey: item.Key, Metadata: map[string]any{"kind": item.Kind}, ActorUserID: input.Scope.UserID, CreatedAt: now}
		if _, err := m.store.Upsert(ctx, item, evidence, event); err != nil {
			return err
		}
	}
	payload := map[string]any{"question": input.Question, "timezone": input.Timezone}
	return m.store.EnqueueJob(ctx, UserMemoryJob{
		Scope: input.Scope, SessionID: input.SessionID, MessageID: input.AssistantMessageID,
		JobType: "memory_consolidate", Payload: payload, Status: "pending", RunAfter: now.Add(5 * time.Minute),
	})
}

func (m *UserManager) List(ctx context.Context, filter UserMemoryListFilter) ([]UserMemory, int64, error) {
	if m == nil || m.store == nil {
		return nil, 0, nil
	}
	return m.store.List(ctx, filter)
}

func (m *UserManager) Save(ctx context.Context, scope UserScope, operation UserMemoryOperation, now time.Time) (UserMemory, error) {
	operation.Action = UserMemoryActionRemember
	result, err := m.ExecuteOperation(ctx, scope, operation, 0, 0, now)
	if err != nil || result.Memory == nil {
		return UserMemory{}, err
	}
	return *result.Memory, nil
}

func (m *UserManager) Update(ctx context.Context, scope UserScope, id uint64, operation UserMemoryOperation, now time.Time) (UserMemory, error) {
	if m == nil || m.store == nil {
		return UserMemory{}, errors.New("长期记忆存储不可用")
	}
	item, err := m.store.Get(ctx, scope, id)
	if err != nil {
		return UserMemory{}, err
	}
	operation.Action = UserMemoryActionRemember
	operation = NormalizeUserMemoryOperation(operation, operation.Content)
	operation.ScopeLevel = item.ScopeLevel()
	if err := ValidateUserMemoryOperation(operation); err != nil {
		return UserMemory{}, err
	}
	updated := BuildUserMemory(item.Scope, operation, UserMemorySourceExplicit, item.SourceSessionID, item.SourceMessageID, now)
	updated.ID = item.ID
	updated.CreatedAt = item.CreatedAt
	updated.Version = item.Version
	event := UserMemoryEvent{Scope: item.Scope, Operation: "updated", MemoryKey: updated.Key, Metadata: map[string]any{"kind": updated.Kind}, ActorUserID: scope.UserID, CreatedAt: now}
	saved, err := m.store.Update(ctx, updated, event)
	if err == nil {
		m.enqueueVectorJob(ctx, saved, "memory_index", now)
	}
	return saved, err
}

func (m *UserManager) Delete(ctx context.Context, scope UserScope, id uint64, now time.Time) error {
	_, err := m.ExecuteOperation(ctx, scope, UserMemoryOperation{Action: UserMemoryActionForget, TargetID: id}, 0, 0, now)
	return err
}

func (m *UserManager) Clear(ctx context.Context, scope UserScope, includeTenant bool, now time.Time) (int64, error) {
	scopeLevel := UserMemoryScopeProject
	if includeTenant {
		scopeLevel = UserMemoryScopeTenant
	}
	result, err := m.ExecuteOperation(ctx, scope, UserMemoryOperation{Action: UserMemoryActionClear, ScopeLevel: scopeLevel}, 0, 0, now)
	return result.Deleted, err
}

func (m *UserManager) SetCandidateStatus(ctx context.Context, scope UserScope, id uint64, confirm bool, now time.Time) (UserMemory, error) {
	action := UserMemoryActionReject
	if confirm {
		action = UserMemoryActionConfirm
	}
	result, err := m.ExecuteOperation(ctx, scope, UserMemoryOperation{Action: action, TargetID: id}, 0, 0, now)
	if err != nil || result.Memory == nil {
		return UserMemory{}, err
	}
	return *result.Memory, nil
}

func (m *UserManager) ListEpisodes(ctx context.Context, scope UserScope, limit int) ([]SessionEpisode, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}
	return m.store.ListEpisodes(ctx, scope, limit)
}

func (m *UserManager) resolveOperationTarget(ctx context.Context, scope UserScope, operation UserMemoryOperation) (UserMemory, error) {
	if operation.TargetID > 0 {
		return m.store.Get(ctx, scope, operation.TargetID)
	}
	items, _, err := m.store.List(ctx, UserMemoryListFilter{Scope: scope, Statuses: []string{UserMemoryStatusActive, UserMemoryStatusCandidate}, Limit: 100})
	if err != nil {
		return UserMemory{}, err
	}
	target := strings.ToLower(strings.TrimSpace(firstNonEmptyMemory(operation.Key, operation.Content)))
	bestScore := 0
	var best UserMemory
	for _, item := range items {
		score := memoryTextScore(target, strings.ToLower(item.Key+" "+item.Content))
		if score > bestScore {
			bestScore = score
			best = item
		}
	}
	if bestScore == 0 {
		return UserMemory{}, ErrUserMemoryNotFound
	}
	return best, nil
}

func selectRelevantUserMemories(question string, items []UserMemory, semanticScores map[uint64]float32, now time.Time, limit int) ([]UserMemoryPromptItem, []uint64) {
	type scored struct {
		item  UserMemory
		score int
	}
	dedup := map[string]bool{}
	scores := make([]scored, 0, len(items))
	for _, item := range items {
		if !item.IsUsable(now) {
			continue
		}
		identity := item.Key
		if identity == "" {
			identity = item.CanonicalHash
		}
		if dedup[identity] {
			continue
		}
		dedup[identity] = true
		score := memoryTextScore(strings.ToLower(question), strings.ToLower(item.Content))
		if item.Kind == UserMemoryKindPreference || item.Kind == UserMemoryKindInstruction || item.Kind == UserMemoryKindCorrection {
			score += 30
		}
		if item.Scope.ProjectID > 0 {
			score += 10
		}
		score += int(item.Confidence * 10)
		if semanticScore, ok := semanticScores[item.ID]; ok {
			score += 50 + int(semanticScore*50)
		}
		if score > 0 {
			scores = append(scores, scored{item: item, score: score})
		}
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	if limit <= 0 || limit > len(scores) {
		limit = len(scores)
	}
	promptItems := make([]UserMemoryPromptItem, 0, limit)
	ids := make([]uint64, 0, limit)
	chars := 0
	for _, candidate := range scores[:limit] {
		content := compactUserMemoryText(candidate.item.Content, 300)
		if chars+len([]rune(content)) > 1200 {
			break
		}
		chars += len([]rune(content))
		promptItems = append(promptItems, UserMemoryPromptItem{ID: candidate.item.ID, Kind: candidate.item.Kind, ScopeLevel: candidate.item.ScopeLevel(), Content: content})
		ids = append(ids, candidate.item.ID)
	}
	return promptItems, ids
}

func (m *UserManager) enqueueVectorJob(ctx context.Context, item UserMemory, jobType string, now time.Time) {
	if m == nil || m.store == nil || m.semantic == nil || item.ID == 0 || item.Scope.ProjectID == 0 {
		return
	}
	_ = m.store.EnqueueJob(ctx, UserMemoryJob{
		Scope: item.Scope, JobType: jobType, Status: "pending", RunAfter: now,
		Payload: map[string]any{"memory_id": item.ID},
	})
}

func memoryTextScore(query string, content string) int {
	query = strings.TrimSpace(query)
	content = strings.TrimSpace(content)
	if query == "" || content == "" {
		return 0
	}
	if strings.Contains(content, query) || strings.Contains(query, content) {
		return 50
	}
	queryRunes := []rune(query)
	if len(queryRunes) < 2 {
		return 0
	}
	seen := map[string]bool{}
	score := 0
	for index := 0; index < len(queryRunes)-1; index++ {
		gram := string(queryRunes[index : index+2])
		if !seen[gram] && strings.Contains(content, gram) {
			score += 2
			seen[gram] = true
		}
	}
	return score
}

func userMemoryStringValue(value map[string]any) string {
	if raw, ok := value["value"]; ok {
		return strings.TrimSpace(fmt.Sprint(raw))
	}
	return ""
}

func inferredMemoryCandidate(question string) (UserMemoryOperation, bool) {
	text := compactUserMemoryText(question, 1000)
	lower := strings.ToLower(text)
	if hasRememberIntent(lower) || hasForgetIntent(lower) {
		return UserMemoryOperation{}, false
	}
	known := detectKnownPreference(text)
	if known.Kind != UserMemoryKindProfile && known.Kind != UserMemoryKindConvention {
		return UserMemoryOperation{}, false
	}
	return UserMemoryOperation{
		Action: UserMemoryActionRemember, Kind: known.Kind, Key: known.Key,
		Content: text, Value: known.Value, ScopeLevel: UserMemoryScopeProject, Confidence: 0.72,
	}, true
}

func compactEpisodeSummary(question string, intent string) string {
	question = compactUserMemoryText(question, 500)
	intent = compactUserMemoryText(intent, 64)
	if intent == "" {
		return "用户问题：" + question
	}
	return "用户问题：" + question + "；处理意图：" + intent
}

func inferEpisodeTopics(question string) []string {
	question = compactUserMemoryText(question, 120)
	if question == "" {
		return nil
	}
	return []string{question}
}

func refersToPastConversation(question string) bool {
	return containsAny(strings.ToLower(question), "上次", "之前", "继续", "接着", "历史会话", "以前分析")
}

func formatUserMemoryList(items []UserMemory) string {
	if len(items) == 0 {
		return "目前没有保存任何长期记忆。"
	}
	lines := make([]string, 0, len(items)+1)
	lines = append(lines, "我目前记得：")
	for index, item := range items {
		status := ""
		if item.Status == UserMemoryStatusCandidate {
			status = "（待确认）"
		}
		lines = append(lines, strconv.Itoa(index+1)+". "+item.Content+" "+userMemoryScopeLabel(item)+status)
	}
	return strings.Join(lines, "\n")
}

func userMemoryScopeLabel(item UserMemory) string {
	if item.Scope.ProjectID == 0 {
		return "所有项目"
	}
	return "当前项目"
}

func firstNonEmptyMemory(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstUserMemoryError(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
