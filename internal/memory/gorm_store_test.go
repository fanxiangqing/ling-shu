package memory

import (
	"context"
	"testing"
)

func TestSessionRecordMatchesScope(t *testing.T) {
	record := sessionStateRecord{SessionID: 4, TenantID: 1, ProjectID: 2, UserID: 3}
	if !sessionRecordMatchesScope(record, Scope{TenantID: 1, ProjectID: 2, SessionID: 4, UserID: 3}) {
		t.Fatal("expected identical scope to match")
	}
	if sessionRecordMatchesScope(record, Scope{TenantID: 9, ProjectID: 2, SessionID: 4, UserID: 3}) {
		t.Fatal("expected cross-tenant scope mismatch")
	}
}

func TestGormStoreRejectsInvalidScopeWithoutDatabase(t *testing.T) {
	store := NewGormStore(nil)
	if _, err := store.LoadSessionState(context.Background(), Scope{}); err == nil {
		t.Fatal("expected invalid scope error")
	}
	if _, err := store.ListRecentArtifacts(context.Background(), Scope{}, 10); err == nil {
		t.Fatal("expected invalid scope error")
	}
	if _, _, err := store.SaveTurn(context.Background(), SessionState{}, nil, -1); err == nil {
		t.Fatal("expected invalid scope error")
	}
}
