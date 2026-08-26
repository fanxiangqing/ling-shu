package memory

import (
	"context"
	"strings"

	"ling-shu/internal/model"
)

const recentArtifactLimit = 12

type Manager struct {
	store Store
}

func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Prepare(ctx context.Context, scope Scope, messages []model.ChatMessage, question string) (PreparedContext, error) {
	prepared := PreparedContext{
		Conversation: BuildConversation(messages),
		State:        SessionState{Scope: scope},
	}
	var loadErr error
	if m != nil && m.store != nil {
		state, err := m.store.LoadSessionState(ctx, scope)
		if err != nil {
			loadErr = err
		} else {
			prepared.State = state
		}
		artifacts, err := m.store.ListRecentArtifacts(ctx, scope, recentArtifactLimit)
		if err != nil {
			if loadErr == nil {
				loadErr = err
			}
		} else {
			prepared.Artifacts = artifacts
		}
	}
	if len(prepared.Artifacts) == 0 {
		prepared.Artifacts = ExtractArtifactsFromMessages(scope, messages)
	}
	prepared.Resolution = ResolveFollowUp(question, prepared.State, prepared.Artifacts)
	return prepared, loadErr
}

func (m *Manager) RecordTurn(ctx context.Context, input TurnInput) error {
	if m == nil || m.store == nil || !input.Scope.Valid() {
		return nil
	}
	state, err := m.store.LoadSessionState(ctx, input.Scope)
	if err != nil {
		return err
	}
	state.Scope = input.Scope
	state.Summary = compactSummary(input.Question, input.Answer)
	state.LastIntent = strings.TrimSpace(input.Intent)
	artifacts := ExtractArtifacts(input)
	focusIndex := bestFocusIndex(artifacts)
	_, _, err = m.store.SaveTurn(ctx, state, artifacts, focusIndex)
	return err
}

func compactSummary(question string, answer string) string {
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	summary := "用户问题：" + question
	if answer != "" {
		summary += "；回答：" + answer
	}
	const maxRunes = 1200
	runes := []rune(summary)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return summary
}

func bestFocusIndex(artifacts []Artifact) int {
	bestIndex := -1
	bestScore := -1
	for index, artifact := range artifacts {
		score := 0
		if len(artifact.Payload.Rows) > 1 {
			score += 30
		}
		if len(artifact.Semantics.Dimensions) > 0 {
			score += 20
		}
		if len(artifact.Semantics.Measures) > 0 {
			score += 10
		}
		if artifact.Completeness == CompletenessComplete {
			score += 10
		}
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}
	return bestIndex
}
