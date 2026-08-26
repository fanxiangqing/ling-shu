package memory

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormStore struct {
	db *gorm.DB
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{db: db}
}

type sessionStateRecord struct {
	SessionID         uint64    `gorm:"primaryKey;column:session_id"`
	TenantID          uint64    `gorm:"column:tenant_id;not null"`
	ProjectID         uint64    `gorm:"column:project_id;not null"`
	UserID            uint64    `gorm:"column:user_id;not null"`
	Summary           string    `gorm:"column:summary;type:text"`
	LastIntent        string    `gorm:"column:last_intent;size:64"`
	ActiveArtifactIDs string    `gorm:"column:active_artifact_ids;type:json"`
	FocusArtifactID   uint64    `gorm:"column:focus_artifact_id"`
	Version           uint64    `gorm:"column:version;not null;default:1"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (sessionStateRecord) TableName() string { return "chat_session_contexts" }

type artifactRecord struct {
	ID                     uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	TenantID               uint64    `gorm:"column:tenant_id;not null"`
	ProjectID              uint64    `gorm:"column:project_id;not null"`
	SessionID              uint64    `gorm:"column:session_id;not null"`
	UserID                 uint64    `gorm:"column:user_id;not null"`
	SourceMessageID        uint64    `gorm:"column:source_message_id;not null"`
	SourceQueryExecutionID uint64    `gorm:"column:source_query_execution_id"`
	Kind                   string    `gorm:"column:kind;size:32;not null"`
	Purpose                string    `gorm:"column:purpose;size:512"`
	Status                 string    `gorm:"column:status;size:32;not null"`
	Completeness           string    `gorm:"column:completeness;size:32;not null"`
	PayloadJSON            string    `gorm:"column:payload_json;type:json;not null"`
	SemanticsJSON          string    `gorm:"column:semantics_json;type:json;not null"`
	LineageJSON            string    `gorm:"column:lineage_json;type:json;not null"`
	CreatedAt              time.Time `gorm:"column:created_at"`
}

func (artifactRecord) TableName() string { return "chat_artifacts" }

func (s *GormStore) LoadSessionState(ctx context.Context, scope Scope) (SessionState, error) {
	empty := SessionState{Scope: scope}
	if !scope.Valid() {
		return empty, errors.New("invalid memory scope")
	}
	if s == nil || s.db == nil {
		return empty, nil
	}
	var record sessionStateRecord
	err := s.db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ? AND project_id = ? AND user_id = ?", scope.SessionID, scope.TenantID, scope.ProjectID, scope.UserID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	return decodeSessionState(record)
}

func (s *GormStore) ListRecentArtifacts(ctx context.Context, scope Scope, limit int) ([]Artifact, error) {
	if !scope.Valid() {
		return nil, errors.New("invalid memory scope")
	}
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = recentArtifactLimit
	}
	if limit > 50 {
		limit = 50
	}
	var records []artifactRecord
	err := s.db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ? AND project_id = ? AND user_id = ? AND status = ?", scope.SessionID, scope.TenantID, scope.ProjectID, scope.UserID, ArtifactStatusActive).
		Order("id DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	out := make([]Artifact, 0, len(records))
	for _, record := range records {
		artifact, err := decodeArtifact(record)
		if err != nil {
			return nil, err
		}
		out = append(out, artifact)
	}
	return out, nil
}

func (s *GormStore) SaveTurn(ctx context.Context, state SessionState, artifacts []Artifact, focusIndex int) (SessionState, []Artifact, error) {
	if !state.Scope.Valid() {
		return state, nil, errors.New("invalid memory scope")
	}
	if s == nil || s.db == nil {
		return state, artifacts, nil
	}
	var savedState SessionState
	savedArtifacts := append([]Artifact(nil), artifacts...)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing sessionStateRecord
		err := tx.Where("session_id = ?", state.Scope.SessionID).First(&existing).Error
		if err == nil && !sessionRecordMatchesScope(existing, state.Scope) {
			return errors.New("memory session scope mismatch")
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		for index := range savedArtifacts {
			record, err := encodeArtifact(savedArtifacts[index])
			if err != nil {
				return err
			}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
			savedArtifacts[index].ID = record.ID
		}
		if len(savedArtifacts) > 0 {
			state.ActiveArtifactIDs = make([]uint64, 0, len(savedArtifacts))
			for _, artifact := range savedArtifacts {
				state.ActiveArtifactIDs = append(state.ActiveArtifactIDs, artifact.ID)
			}
			if focusIndex >= 0 && focusIndex < len(savedArtifacts) {
				state.FocusArtifactID = savedArtifacts[focusIndex].ID
			}
		}
		record, err := encodeSessionState(state)
		if err != nil {
			return err
		}
		record.Version = 1
		err = tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "session_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"tenant_id":           record.TenantID,
				"project_id":          record.ProjectID,
				"user_id":             record.UserID,
				"summary":             record.Summary,
				"last_intent":         record.LastIntent,
				"active_artifact_ids": record.ActiveArtifactIDs,
				"focus_artifact_id":   record.FocusArtifactID,
				"version":             gorm.Expr("version + 1"),
				"updated_at":          time.Now(),
			}),
		}).Create(&record).Error
		if err != nil {
			return err
		}
		savedState, err = s.loadSessionStateWithDB(ctx, tx, state.Scope)
		return err
	})
	if err != nil {
		return state, nil, err
	}
	return savedState, savedArtifacts, nil
}

func sessionRecordMatchesScope(record sessionStateRecord, scope Scope) bool {
	return record.SessionID == scope.SessionID &&
		record.TenantID == scope.TenantID &&
		record.ProjectID == scope.ProjectID &&
		record.UserID == scope.UserID
}

func (s *GormStore) loadSessionStateWithDB(ctx context.Context, db *gorm.DB, scope Scope) (SessionState, error) {
	var record sessionStateRecord
	err := db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ? AND project_id = ? AND user_id = ?", scope.SessionID, scope.TenantID, scope.ProjectID, scope.UserID).
		First(&record).Error
	if err != nil {
		return SessionState{Scope: scope}, err
	}
	return decodeSessionState(record)
}

func encodeSessionState(state SessionState) (sessionStateRecord, error) {
	active, err := json.Marshal(state.ActiveArtifactIDs)
	if err != nil {
		return sessionStateRecord{}, err
	}
	return sessionStateRecord{
		SessionID:         state.Scope.SessionID,
		TenantID:          state.Scope.TenantID,
		ProjectID:         state.Scope.ProjectID,
		UserID:            state.Scope.UserID,
		Summary:           state.Summary,
		LastIntent:        state.LastIntent,
		ActiveArtifactIDs: string(active),
		FocusArtifactID:   state.FocusArtifactID,
		Version:           state.Version,
	}, nil
}

func decodeSessionState(record sessionStateRecord) (SessionState, error) {
	state := SessionState{
		Scope: Scope{
			TenantID:  record.TenantID,
			ProjectID: record.ProjectID,
			SessionID: record.SessionID,
			UserID:    record.UserID,
		},
		Summary:         record.Summary,
		LastIntent:      record.LastIntent,
		FocusArtifactID: record.FocusArtifactID,
		Version:         record.Version,
		UpdatedAt:       record.UpdatedAt,
	}
	if record.ActiveArtifactIDs != "" {
		if err := json.Unmarshal([]byte(record.ActiveArtifactIDs), &state.ActiveArtifactIDs); err != nil {
			return SessionState{}, err
		}
	}
	return state, nil
}

func encodeArtifact(artifact Artifact) (artifactRecord, error) {
	payload, err := json.Marshal(artifact.Payload)
	if err != nil {
		return artifactRecord{}, err
	}
	semantics, err := json.Marshal(artifact.Semantics)
	if err != nil {
		return artifactRecord{}, err
	}
	lineage, err := json.Marshal(artifact.Lineage)
	if err != nil {
		return artifactRecord{}, err
	}
	queryExecutionID := uint64(0)
	if len(artifact.Lineage.QueryExecutionIDs) > 0 {
		queryExecutionID = artifact.Lineage.QueryExecutionIDs[0]
	}
	return artifactRecord{
		ID:                     artifact.ID,
		TenantID:               artifact.Scope.TenantID,
		ProjectID:              artifact.Scope.ProjectID,
		SessionID:              artifact.Scope.SessionID,
		UserID:                 artifact.Scope.UserID,
		SourceMessageID:        artifact.SourceMessageID,
		SourceQueryExecutionID: queryExecutionID,
		Kind:                   artifact.Kind,
		Purpose:                artifact.Purpose,
		Status:                 artifact.Status,
		Completeness:           artifact.Completeness,
		PayloadJSON:            string(payload),
		SemanticsJSON:          string(semantics),
		LineageJSON:            string(lineage),
		CreatedAt:              artifact.CreatedAt,
	}, nil
}

func decodeArtifact(record artifactRecord) (Artifact, error) {
	artifact := Artifact{
		ID: record.ID,
		Scope: Scope{
			TenantID:  record.TenantID,
			ProjectID: record.ProjectID,
			SessionID: record.SessionID,
			UserID:    record.UserID,
		},
		SourceMessageID: record.SourceMessageID,
		Purpose:         record.Purpose,
		Kind:            record.Kind,
		Status:          record.Status,
		Completeness:    record.Completeness,
		CreatedAt:       record.CreatedAt,
	}
	if err := json.Unmarshal([]byte(record.PayloadJSON), &artifact.Payload); err != nil {
		return Artifact{}, err
	}
	if err := json.Unmarshal([]byte(record.SemanticsJSON), &artifact.Semantics); err != nil {
		return Artifact{}, err
	}
	if err := json.Unmarshal([]byte(record.LineageJSON), &artifact.Lineage); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}
