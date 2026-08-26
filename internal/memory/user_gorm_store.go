package memory

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUserMemoryNotFound = errors.New("user memory not found")

type GormUserMemoryStore struct {
	db *gorm.DB
}

func NewGormUserMemoryStore(db *gorm.DB) *GormUserMemoryStore {
	return &GormUserMemoryStore{db: db}
}

type userMemoryRecord struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement;column:id"`
	TenantID          uint64     `gorm:"column:tenant_id;not null"`
	ProjectID         uint64     `gorm:"column:project_id;not null"`
	UserID            uint64     `gorm:"column:user_id;not null"`
	Kind              string     `gorm:"column:kind;size:32;not null"`
	MemoryKey         string     `gorm:"column:memory_key;size:191"`
	CanonicalHash     string     `gorm:"column:canonical_hash;size:64;not null"`
	Content           string     `gorm:"column:content;type:text;not null"`
	ValueJSON         string     `gorm:"column:value_json;type:json;not null"`
	ApplicabilityJSON string     `gorm:"column:applicability_json;type:json;not null"`
	Status            string     `gorm:"column:status;size:32;not null"`
	SourceType        string     `gorm:"column:source_type;size:32;not null"`
	Confidence        float64    `gorm:"column:confidence;type:decimal(6,5);not null"`
	Salience          float64    `gorm:"column:salience;type:decimal(6,5);not null"`
	Sensitivity       string     `gorm:"column:sensitivity;size:32;not null"`
	EvidenceCount     int        `gorm:"column:evidence_count;not null"`
	Version           uint64     `gorm:"column:version;not null"`
	ObservedAt        *time.Time `gorm:"column:observed_at"`
	ValidFrom         *time.Time `gorm:"column:valid_from"`
	ExpiresAt         *time.Time `gorm:"column:expires_at"`
	LastConfirmedAt   *time.Time `gorm:"column:last_confirmed_at"`
	LastRecalledAt    *time.Time `gorm:"column:last_recalled_at"`
	LastAppliedAt     *time.Time `gorm:"column:last_applied_at"`
	ApplyCount        int        `gorm:"column:apply_count;not null"`
	SourceSessionID   uint64     `gorm:"column:source_session_id"`
	SourceMessageID   uint64     `gorm:"column:source_message_id"`
	EmbeddingProvider string     `gorm:"column:embedding_provider;size:64"`
	EmbeddingModel    string     `gorm:"column:embedding_model;size:128"`
	VectorID          string     `gorm:"column:vector_id;size:128"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (userMemoryRecord) TableName() string { return "user_memories" }

type userMemoryEvidenceRecord struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	MemoryID        uint64    `gorm:"column:memory_id;not null"`
	TenantID        uint64    `gorm:"column:tenant_id;not null"`
	ProjectID       uint64    `gorm:"column:project_id;not null"`
	UserID          uint64    `gorm:"column:user_id;not null"`
	SessionID       uint64    `gorm:"column:session_id"`
	MessageID       uint64    `gorm:"column:message_id"`
	EvidenceType    string    `gorm:"column:evidence_type;size:32;not null"`
	EvidenceSummary string    `gorm:"column:evidence_summary;size:512"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (userMemoryEvidenceRecord) TableName() string { return "user_memory_evidence" }

type userMemoryEventRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	MemoryID     uint64    `gorm:"column:memory_id"`
	TenantID     uint64    `gorm:"column:tenant_id;not null"`
	ProjectID    uint64    `gorm:"column:project_id;not null"`
	UserID       uint64    `gorm:"column:user_id;not null"`
	Operation    string    `gorm:"column:operation;size:32;not null"`
	MemoryKey    string    `gorm:"column:memory_key;size:191"`
	MetadataJSON string    `gorm:"column:metadata_json;type:json;not null"`
	ActorUserID  uint64    `gorm:"column:actor_user_id;not null"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (userMemoryEventRecord) TableName() string { return "user_memory_events" }

type sessionEpisodeRecord struct {
	ID                    uint64     `gorm:"primaryKey;autoIncrement;column:id"`
	TenantID              uint64     `gorm:"column:tenant_id;not null"`
	ProjectID             uint64     `gorm:"column:project_id;not null"`
	UserID                uint64     `gorm:"column:user_id;not null"`
	SessionID             uint64     `gorm:"column:session_id;not null"`
	Summary               string     `gorm:"column:summary;type:text;not null"`
	TopicsJSON            string     `gorm:"column:topics_json;type:json;not null"`
	DecisionsJSON         string     `gorm:"column:decisions_json;type:json;not null"`
	OpenLoopsJSON         string     `gorm:"column:open_loops_json;type:json;not null"`
	QueryExecutionIDsJSON string     `gorm:"column:query_execution_ids_json;type:json;not null"`
	MetadataJSON          string     `gorm:"column:metadata_json;type:json;not null"`
	ExpiresAt             *time.Time `gorm:"column:expires_at"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (sessionEpisodeRecord) TableName() string { return "chat_session_episodes" }

type userMemoryJobRecord struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	TenantID    uint64    `gorm:"column:tenant_id;not null"`
	ProjectID   uint64    `gorm:"column:project_id;not null"`
	UserID      uint64    `gorm:"column:user_id;not null"`
	SessionID   uint64    `gorm:"column:session_id"`
	MessageID   uint64    `gorm:"column:message_id"`
	JobType     string    `gorm:"column:job_type;size:32;not null"`
	PayloadJSON string    `gorm:"column:payload_json;type:json;not null"`
	Status      string    `gorm:"column:status;size:32;not null"`
	Attempts    int       `gorm:"column:attempts;not null"`
	RunAfter    time.Time `gorm:"column:run_after;not null"`
	LastError   string    `gorm:"column:last_error;type:text"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (userMemoryJobRecord) TableName() string { return "memory_jobs" }

func (s *GormUserMemoryStore) List(ctx context.Context, filter UserMemoryListFilter) ([]UserMemory, int64, error) {
	if !filter.Scope.Valid() {
		return nil, 0, errors.New("invalid user memory scope")
	}
	if s == nil || s.db == nil {
		return nil, 0, nil
	}
	query := s.db.WithContext(ctx).Model(&userMemoryRecord{}).
		Where("tenant_id = ? AND user_id = ?", filter.Scope.TenantID, filter.Scope.UserID)
	if filter.Scope.ProjectID > 0 {
		query = query.Where("project_id IN ?", []uint64{0, filter.Scope.ProjectID})
	} else {
		query = query.Where("project_id = 0")
	}
	if len(filter.Statuses) > 0 {
		query = query.Where("status IN ?", filter.Statuses)
	}
	if len(filter.Kinds) > 0 {
		query = query.Where("kind IN ?", filter.Kinds)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var records []userMemoryRecord
	err := query.Order("project_id DESC").Order("status ASC").Order("updated_at DESC").
		Offset(filter.Offset).Limit(limit).Find(&records).Error
	if err != nil {
		return nil, 0, err
	}
	items, err := decodeUserMemoryRecords(records)
	return items, total, err
}

func (s *GormUserMemoryStore) ListApplicable(ctx context.Context, scope UserScope, now time.Time, limit int) ([]UserMemory, error) {
	if !scope.Valid() || scope.ProjectID == 0 {
		return nil, errors.New("invalid applicable user memory scope")
	}
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var records []userMemoryRecord
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND project_id IN ? AND status = ?", scope.TenantID, scope.UserID, []uint64{0, scope.ProjectID}, UserMemoryStatusActive).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("project_id DESC").Order("confidence DESC").Order("salience DESC").Order("updated_at DESC").
		Limit(limit).Find(&records).Error
	if err != nil {
		return nil, err
	}
	return decodeUserMemoryRecords(records)
}

func (s *GormUserMemoryStore) Get(ctx context.Context, scope UserScope, id uint64) (UserMemory, error) {
	if !scope.Valid() || id == 0 {
		return UserMemory{}, errors.New("invalid user memory lookup")
	}
	if s == nil || s.db == nil {
		return UserMemory{}, ErrUserMemoryNotFound
	}
	var record userMemoryRecord
	err := s.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, scope.TenantID, scope.UserID).
		Where("project_id IN ?", []uint64{0, scope.ProjectID}).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UserMemory{}, ErrUserMemoryNotFound
	}
	if err != nil {
		return UserMemory{}, err
	}
	return decodeUserMemoryRecord(record)
}

func (s *GormUserMemoryStore) Upsert(ctx context.Context, item UserMemory, evidence UserMemoryEvidence, event UserMemoryEvent) (UserMemory, error) {
	if !item.Scope.Valid() || item.CanonicalHash == "" {
		return UserMemory{}, errors.New("invalid user memory")
	}
	if s == nil || s.db == nil {
		return item, nil
	}
	record, err := encodeUserMemoryRecord(item)
	if err != nil {
		return UserMemory{}, err
	}
	var saved UserMemory
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		updates := map[string]any{
			"kind": item.Kind, "memory_key": item.Key, "content": item.Content,
			"value_json": record.ValueJSON, "applicability_json": record.ApplicabilityJSON,
			"source_type": item.SourceType, "confidence": item.Confidence, "salience": item.Salience,
			"sensitivity": item.Sensitivity, "status": item.Status,
			"evidence_count": gorm.Expr("evidence_count + 1"), "version": gorm.Expr("version + 1"),
			"observed_at": item.ObservedAt, "valid_from": item.ValidFrom, "expires_at": item.ExpiresAt,
			"last_confirmed_at": item.LastConfirmedAt, "source_session_id": item.SourceSessionID,
			"source_message_id": item.SourceMessageID, "updated_at": now,
		}
		if item.SourceType == UserMemorySourceInferred {
			preserveActive := func(column string, incoming any) clause.Expr {
				return gorm.Expr("CASE WHEN status = ? THEN "+column+" ELSE ? END", UserMemoryStatusActive, incoming)
			}
			updates["kind"] = preserveActive("kind", item.Kind)
			updates["memory_key"] = preserveActive("memory_key", item.Key)
			updates["content"] = preserveActive("content", item.Content)
			updates["value_json"] = preserveActive("value_json", record.ValueJSON)
			updates["applicability_json"] = preserveActive("applicability_json", record.ApplicabilityJSON)
			updates["source_type"] = preserveActive("source_type", item.SourceType)
			updates["confidence"] = preserveActive("confidence", item.Confidence)
			updates["salience"] = preserveActive("salience", item.Salience)
			updates["sensitivity"] = preserveActive("sensitivity", item.Sensitivity)
			updates["status"] = gorm.Expr("CASE WHEN status = ? THEN status ELSE ? END", UserMemoryStatusActive, item.Status)
			updates["valid_from"] = preserveActive("valid_from", item.ValidFrom)
			updates["expires_at"] = preserveActive("expires_at", item.ExpiresAt)
			updates["last_confirmed_at"] = preserveActive("last_confirmed_at", item.LastConfirmedAt)
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "project_id"}, {Name: "user_id"}, {Name: "canonical_hash"}},
			DoUpdates: clause.Assignments(updates),
		}).Create(&record).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND project_id = ? AND user_id = ? AND canonical_hash = ?", item.Scope.TenantID, item.Scope.ProjectID, item.Scope.UserID, item.CanonicalHash).
			First(&record).Error; err != nil {
			return err
		}
		if evidence.EvidenceType != "" {
			evidence.MemoryID = record.ID
			if err := tx.Create(encodeUserMemoryEvidenceRecord(evidence)).Error; err != nil {
				return err
			}
		}
		event.MemoryID = record.ID
		if event.Operation != "" {
			encodedEvent, err := encodeUserMemoryEventRecord(event)
			if err != nil {
				return err
			}
			if err := tx.Create(&encodedEvent).Error; err != nil {
				return err
			}
		}
		saved, err = decodeUserMemoryRecord(record)
		return err
	})
	return saved, err
}

func (s *GormUserMemoryStore) Update(ctx context.Context, item UserMemory, event UserMemoryEvent) (UserMemory, error) {
	if !item.Scope.Valid() || item.ID == 0 || item.CanonicalHash == "" {
		return UserMemory{}, errors.New("invalid user memory update")
	}
	if s == nil || s.db == nil {
		return item, nil
	}
	record, err := encodeUserMemoryRecord(item)
	if err != nil {
		return UserMemory{}, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&userMemoryRecord{}).
			Where("id = ? AND tenant_id = ? AND project_id = ? AND user_id = ?", item.ID, item.Scope.TenantID, item.Scope.ProjectID, item.Scope.UserID).
			Updates(map[string]any{
				"kind": record.Kind, "memory_key": record.MemoryKey, "canonical_hash": record.CanonicalHash,
				"content": record.Content, "value_json": record.ValueJSON, "applicability_json": record.ApplicabilityJSON,
				"status": record.Status, "confidence": record.Confidence, "salience": record.Salience,
				"sensitivity": record.Sensitivity, "expires_at": record.ExpiresAt,
				"version": gorm.Expr("version + 1"), "updated_at": time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrUserMemoryNotFound
		}
		event.MemoryID = item.ID
		encoded, err := encodeUserMemoryEventRecord(event)
		if err != nil {
			return err
		}
		return tx.Create(&encoded).Error
	})
	if err != nil {
		return UserMemory{}, err
	}
	return s.Get(ctx, item.Scope, item.ID)
}

func (s *GormUserMemoryStore) SetStatus(ctx context.Context, scope UserScope, id uint64, status string, event UserMemoryEvent) (UserMemory, error) {
	item, err := s.Get(ctx, scope, id)
	if err != nil {
		return UserMemory{}, err
	}
	if s == nil || s.db == nil {
		item.Status = status
		return item, nil
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&userMemoryRecord{}).
			Where("id = ? AND tenant_id = ? AND user_id = ?", id, scope.TenantID, scope.UserID).
			UpdateColumns(map[string]any{"status": status, "version": gorm.Expr("version + 1"), "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrUserMemoryNotFound
		}
		event.MemoryID = id
		encoded, err := encodeUserMemoryEventRecord(event)
		if err != nil {
			return err
		}
		return tx.Create(&encoded).Error
	})
	if err != nil {
		return UserMemory{}, err
	}
	return s.Get(ctx, scope, id)
}

func (s *GormUserMemoryStore) Delete(ctx context.Context, scope UserScope, id uint64, event UserMemoryEvent) error {
	if _, err := s.Get(ctx, scope, id); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("memory_id = ?", id).Delete(&userMemoryEvidenceRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND tenant_id = ? AND user_id = ?", id, scope.TenantID, scope.UserID).Delete(&userMemoryRecord{}).Error; err != nil {
			return err
		}
		event.MemoryID = id
		encoded, err := encodeUserMemoryEventRecord(event)
		if err != nil {
			return err
		}
		return tx.Create(&encoded).Error
	})
}

func (s *GormUserMemoryStore) Clear(ctx context.Context, scope UserScope, includeTenant bool, event UserMemoryEvent) (int64, error) {
	if !scope.Valid() {
		return 0, errors.New("invalid user memory scope")
	}
	if s == nil || s.db == nil {
		return 0, nil
	}
	projectIDs := []uint64{scope.ProjectID}
	if includeTenant {
		projectIDs = []uint64{0, scope.ProjectID}
	}
	var affected int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []uint64
		if err := tx.Model(&userMemoryRecord{}).
			Where("tenant_id = ? AND user_id = ? AND project_id IN ?", scope.TenantID, scope.UserID, projectIDs).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			if err := tx.Where("memory_id IN ?", ids).Delete(&userMemoryEvidenceRecord{}).Error; err != nil {
				return err
			}
		}
		result := tx.Where("tenant_id = ? AND user_id = ? AND project_id IN ?", scope.TenantID, scope.UserID, projectIDs).
			Delete(&userMemoryRecord{})
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		encoded, err := encodeUserMemoryEventRecord(event)
		if err != nil {
			return err
		}
		return tx.Create(&encoded).Error
	})
	return affected, err
}

func (s *GormUserMemoryStore) MarkRecalled(ctx context.Context, scope UserScope, ids []uint64, applied bool) error {
	if len(ids) == 0 || s == nil || s.db == nil {
		return nil
	}
	updates := map[string]any{"last_recalled_at": time.Now()}
	if applied {
		updates["last_applied_at"] = time.Now()
		updates["apply_count"] = gorm.Expr("apply_count + 1")
	}
	return s.db.WithContext(ctx).Model(&userMemoryRecord{}).
		Where("id IN ? AND tenant_id = ? AND user_id = ? AND project_id IN ?", ids, scope.TenantID, scope.UserID, []uint64{0, scope.ProjectID}).
		Updates(updates).Error
}

func (s *GormUserMemoryStore) SetVectorMetadata(ctx context.Context, scope UserScope, id uint64, metadata UserMemoryVectorMetadata) error {
	if s == nil || s.db == nil || id == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&userMemoryRecord{}).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, scope.TenantID, scope.UserID).
		Updates(map[string]any{
			"embedding_provider": metadata.Provider,
			"embedding_model":    metadata.Model,
			"vector_id":          metadata.VectorID,
			"updated_at":         time.Now(),
		}).Error
}

func (s *GormUserMemoryStore) UpsertEpisode(ctx context.Context, episode SessionEpisode) (SessionEpisode, error) {
	if !episode.Scope.Valid() || episode.Scope.ProjectID == 0 || episode.SessionID == 0 {
		return SessionEpisode{}, errors.New("invalid session episode")
	}
	if s == nil || s.db == nil {
		return episode, nil
	}
	record, err := encodeSessionEpisodeRecord(episode)
	if err != nil {
		return SessionEpisode{}, err
	}
	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"summary", "topics_json", "decisions_json", "open_loops_json", "query_execution_ids_json", "metadata_json", "expires_at", "updated_at"}),
	}).Create(&record).Error
	if err != nil {
		return SessionEpisode{}, err
	}
	if err := s.db.WithContext(ctx).Where("session_id = ? AND tenant_id = ? AND project_id = ? AND user_id = ?", episode.SessionID, episode.Scope.TenantID, episode.Scope.ProjectID, episode.Scope.UserID).
		First(&record).Error; err != nil {
		return SessionEpisode{}, err
	}
	return decodeSessionEpisodeRecord(record)
}

func (s *GormUserMemoryStore) ListEpisodes(ctx context.Context, scope UserScope, limit int) ([]SessionEpisode, error) {
	if !scope.Valid() || scope.ProjectID == 0 {
		return nil, errors.New("invalid episode scope")
	}
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var records []sessionEpisodeRecord
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND project_id = ? AND user_id = ?", scope.TenantID, scope.ProjectID, scope.UserID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).Order("updated_at DESC").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, err
	}
	items := make([]SessionEpisode, 0, len(records))
	for _, record := range records {
		item, err := decodeSessionEpisodeRecord(record)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *GormUserMemoryStore) EnqueueJob(ctx context.Context, job UserMemoryJob) error {
	if !job.Scope.Valid() || job.JobType == "" {
		return errors.New("invalid memory job")
	}
	if s == nil || s.db == nil {
		return nil
	}
	payload, err := marshalJSON(job.Payload)
	if err != nil {
		return err
	}
	if job.Status == "" {
		job.Status = "pending"
	}
	if job.RunAfter.IsZero() {
		job.RunAfter = time.Now()
	}
	record := userMemoryJobRecord{
		TenantID: job.Scope.TenantID, ProjectID: job.Scope.ProjectID, UserID: job.Scope.UserID,
		SessionID: job.SessionID, MessageID: job.MessageID, JobType: job.JobType,
		PayloadJSON: payload, Status: job.Status, Attempts: job.Attempts, RunAfter: job.RunAfter, LastError: job.LastError,
	}
	return s.db.WithContext(ctx).Create(&record).Error
}

func (s *GormUserMemoryStore) ClaimJobs(ctx context.Context, now time.Time, limit int) ([]UserMemoryJob, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	staleBefore := now.Add(-5 * time.Minute)
	if err := s.db.WithContext(ctx).Model(&userMemoryJobRecord{}).
		Where("status = ? AND updated_at < ? AND attempts < ?", "running", staleBefore, 3).
		Updates(map[string]any{"status": "pending", "run_after": now, "updated_at": now}).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&userMemoryJobRecord{}).
		Where("status = ? AND updated_at < ? AND attempts >= ?", "running", staleBefore, 3).
		Updates(map[string]any{"status": "failed", "last_error": "worker lease expired", "updated_at": now}).Error; err != nil {
		return nil, err
	}
	var records []userMemoryJobRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND run_after <= ?", "pending", now).Order("id ASC").Limit(limit).Find(&records).Error; err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(records))
		for _, record := range records {
			ids = append(ids, record.ID)
		}
		return tx.Model(&userMemoryJobRecord{}).Where("id IN ?", ids).
			Updates(map[string]any{"status": "running", "attempts": gorm.Expr("attempts + 1"), "updated_at": now}).Error
	})
	if err != nil {
		return nil, err
	}
	jobs := make([]UserMemoryJob, 0, len(records))
	for _, record := range records {
		job, err := decodeUserMemoryJobRecord(record)
		if err != nil {
			return nil, err
		}
		job.Status = "running"
		job.Attempts++
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *GormUserMemoryStore) CompleteJob(ctx context.Context, id uint64) error {
	if s == nil || s.db == nil || id == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&userMemoryJobRecord{}).Where("id = ?", id).
		Updates(map[string]any{"status": "completed", "last_error": "", "updated_at": time.Now()}).Error
}

func (s *GormUserMemoryStore) FailJob(ctx context.Context, id uint64, jobErr error, retryAt time.Time) error {
	if s == nil || s.db == nil || id == 0 {
		return nil
	}
	status := "pending"
	var record userMemoryJobRecord
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return err
	}
	if record.Attempts >= 3 {
		status = "failed"
	}
	message := ""
	if jobErr != nil {
		message = compactUserMemoryText(jobErr.Error(), 2000)
	}
	return s.db.WithContext(ctx).Model(&userMemoryJobRecord{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "last_error": message, "run_after": retryAt, "updated_at": time.Now()}).Error
}

func encodeUserMemoryRecord(item UserMemory) (userMemoryRecord, error) {
	value, err := marshalJSON(item.Value)
	if err != nil {
		return userMemoryRecord{}, err
	}
	applicability, err := marshalJSON(item.Applicability)
	if err != nil {
		return userMemoryRecord{}, err
	}
	return userMemoryRecord{
		ID: item.ID, TenantID: item.Scope.TenantID, ProjectID: item.Scope.ProjectID, UserID: item.Scope.UserID,
		Kind: item.Kind, MemoryKey: item.Key, CanonicalHash: item.CanonicalHash, Content: item.Content,
		ValueJSON: value, ApplicabilityJSON: applicability, Status: item.Status, SourceType: item.SourceType,
		Confidence: item.Confidence, Salience: item.Salience, Sensitivity: item.Sensitivity,
		EvidenceCount: maxInt(item.EvidenceCount, 1), Version: maxUint64(item.Version, 1), ObservedAt: item.ObservedAt,
		ValidFrom: item.ValidFrom, ExpiresAt: item.ExpiresAt, LastConfirmedAt: item.LastConfirmedAt,
		LastRecalledAt: item.LastRecalledAt, LastAppliedAt: item.LastAppliedAt, ApplyCount: item.ApplyCount,
		SourceSessionID: item.SourceSessionID, SourceMessageID: item.SourceMessageID,
		EmbeddingProvider: item.EmbeddingProvider, EmbeddingModel: item.EmbeddingModel, VectorID: item.VectorID,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}, nil
}

func decodeUserMemoryRecords(records []userMemoryRecord) ([]UserMemory, error) {
	items := make([]UserMemory, 0, len(records))
	for _, record := range records {
		item, err := decodeUserMemoryRecord(record)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func decodeUserMemoryRecord(record userMemoryRecord) (UserMemory, error) {
	item := UserMemory{
		ID: record.ID, Scope: UserScope{TenantID: record.TenantID, ProjectID: record.ProjectID, UserID: record.UserID},
		Kind: record.Kind, Key: record.MemoryKey, CanonicalHash: record.CanonicalHash, Content: record.Content,
		Status: record.Status, SourceType: record.SourceType, Confidence: record.Confidence, Salience: record.Salience,
		Sensitivity: record.Sensitivity, EvidenceCount: record.EvidenceCount, Version: record.Version,
		ObservedAt: record.ObservedAt, ValidFrom: record.ValidFrom, ExpiresAt: record.ExpiresAt,
		LastConfirmedAt: record.LastConfirmedAt, LastRecalledAt: record.LastRecalledAt, LastAppliedAt: record.LastAppliedAt,
		ApplyCount: record.ApplyCount, SourceSessionID: record.SourceSessionID, SourceMessageID: record.SourceMessageID,
		EmbeddingProvider: record.EmbeddingProvider, EmbeddingModel: record.EmbeddingModel, VectorID: record.VectorID,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	if err := unmarshalJSONObject(record.ValueJSON, &item.Value); err != nil {
		return UserMemory{}, err
	}
	if err := unmarshalJSONObject(record.ApplicabilityJSON, &item.Applicability); err != nil {
		return UserMemory{}, err
	}
	return item, nil
}

func encodeUserMemoryEvidenceRecord(item UserMemoryEvidence) *userMemoryEvidenceRecord {
	return &userMemoryEvidenceRecord{
		MemoryID: item.MemoryID, TenantID: item.Scope.TenantID, ProjectID: item.Scope.ProjectID, UserID: item.Scope.UserID,
		SessionID: item.SessionID, MessageID: item.MessageID, EvidenceType: item.EvidenceType,
		EvidenceSummary: item.EvidenceSummary, CreatedAt: item.CreatedAt,
	}
}

func encodeUserMemoryEventRecord(item UserMemoryEvent) (userMemoryEventRecord, error) {
	metadata, err := marshalJSON(item.Metadata)
	if err != nil {
		return userMemoryEventRecord{}, err
	}
	return userMemoryEventRecord{
		MemoryID: item.MemoryID, TenantID: item.Scope.TenantID, ProjectID: item.Scope.ProjectID, UserID: item.Scope.UserID,
		Operation: item.Operation, MemoryKey: item.MemoryKey, MetadataJSON: metadata,
		ActorUserID: item.ActorUserID, CreatedAt: item.CreatedAt,
	}, nil
}

func encodeSessionEpisodeRecord(item SessionEpisode) (sessionEpisodeRecord, error) {
	topics, err := json.Marshal(item.Topics)
	if err != nil {
		return sessionEpisodeRecord{}, err
	}
	decisions, err := json.Marshal(item.Decisions)
	if err != nil {
		return sessionEpisodeRecord{}, err
	}
	openLoops, err := json.Marshal(item.OpenLoops)
	if err != nil {
		return sessionEpisodeRecord{}, err
	}
	queryIDs, err := json.Marshal(item.QueryExecutionIDs)
	if err != nil {
		return sessionEpisodeRecord{}, err
	}
	metadata, err := marshalJSON(item.Metadata)
	if err != nil {
		return sessionEpisodeRecord{}, err
	}
	return sessionEpisodeRecord{
		ID: item.ID, TenantID: item.Scope.TenantID, ProjectID: item.Scope.ProjectID, UserID: item.Scope.UserID,
		SessionID: item.SessionID, Summary: item.Summary, TopicsJSON: string(topics), DecisionsJSON: string(decisions),
		OpenLoopsJSON: string(openLoops), QueryExecutionIDsJSON: string(queryIDs), MetadataJSON: metadata,
		ExpiresAt: item.ExpiresAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}, nil
}

func decodeSessionEpisodeRecord(record sessionEpisodeRecord) (SessionEpisode, error) {
	item := SessionEpisode{
		ID: record.ID, Scope: UserScope{TenantID: record.TenantID, ProjectID: record.ProjectID, UserID: record.UserID},
		SessionID: record.SessionID, Summary: record.Summary, ExpiresAt: record.ExpiresAt,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	if err := json.Unmarshal([]byte(record.TopicsJSON), &item.Topics); err != nil {
		return SessionEpisode{}, err
	}
	if err := json.Unmarshal([]byte(record.DecisionsJSON), &item.Decisions); err != nil {
		return SessionEpisode{}, err
	}
	if err := json.Unmarshal([]byte(record.OpenLoopsJSON), &item.OpenLoops); err != nil {
		return SessionEpisode{}, err
	}
	if err := json.Unmarshal([]byte(record.QueryExecutionIDsJSON), &item.QueryExecutionIDs); err != nil {
		return SessionEpisode{}, err
	}
	if err := unmarshalJSONObject(record.MetadataJSON, &item.Metadata); err != nil {
		return SessionEpisode{}, err
	}
	return item, nil
}

func decodeUserMemoryJobRecord(record userMemoryJobRecord) (UserMemoryJob, error) {
	job := UserMemoryJob{
		ID: record.ID, Scope: UserScope{TenantID: record.TenantID, ProjectID: record.ProjectID, UserID: record.UserID},
		SessionID: record.SessionID, MessageID: record.MessageID, JobType: record.JobType,
		Status: record.Status, Attempts: record.Attempts, RunAfter: record.RunAfter, LastError: record.LastError,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
	if err := unmarshalJSONObject(record.PayloadJSON, &job.Payload); err != nil {
		return UserMemoryJob{}, err
	}
	return job, nil
}

func marshalJSON(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	return string(encoded), err
}

func unmarshalJSONObject(value string, target *map[string]any) error {
	if value == "" || value == "null" {
		*target = map[string]any{}
		return nil
	}
	return json.Unmarshal([]byte(value), target)
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func maxUint64(value, fallback uint64) uint64 {
	if value > 0 {
		return value
	}
	return fallback
}
