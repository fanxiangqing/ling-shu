package memory

import (
	"context"
	"testing"
)

type recordingStore struct {
	state     SessionState
	artifacts []Artifact
	focus     int
}

func (s *recordingStore) LoadSessionState(ctx context.Context, scope Scope) (SessionState, error) {
	if !s.state.Scope.Valid() {
		s.state.Scope = scope
	}
	return s.state, nil
}

func (s *recordingStore) ListRecentArtifacts(ctx context.Context, scope Scope, limit int) ([]Artifact, error) {
	return s.artifacts, nil
}

func (s *recordingStore) SaveTurn(ctx context.Context, state SessionState, artifacts []Artifact, focusIndex int) (SessionState, []Artifact, error) {
	s.state = state
	s.artifacts = artifacts
	s.focus = focusIndex
	return state, artifacts, nil
}

func TestManagerRecordTurnSelectsInformationRichFocus(t *testing.T) {
	store := &recordingStore{}
	manager := NewManager(store)
	scope := Scope{TenantID: 1, ProjectID: 2, SessionID: 3, UserID: 4}
	err := manager.RecordTurn(context.Background(), TurnInput{
		Scope:           scope,
		SourceMessageID: 8,
		Question:        "项目情况",
		Answer:          "已完成",
		Intent:          "multi_query",
		Executions: []ExecutionSnapshot{
			{Purpose: "最大环号", Status: "success", Columns: []string{"current_ring"}, Rows: []map[string]any{{"current_ring": 1254}}},
			{Purpose: "风险列表", Status: "success", Columns: []string{"风险名称"}, Rows: []map[string]any{{"风险名称": "A"}, {"风险名称": "B"}}},
		},
	})
	if err != nil {
		t.Fatalf("record turn: %v", err)
	}
	if store.focus != 1 {
		t.Fatalf("expected risk list as focus, got %d", store.focus)
	}
	if store.state.LastIntent != "multi_query" || len(store.artifacts) != 2 {
		t.Fatalf("unexpected saved turn: state=%+v artifacts=%+v", store.state, store.artifacts)
	}
}
