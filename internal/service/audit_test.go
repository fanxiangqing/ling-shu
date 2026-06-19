package service

import (
	"context"
	"errors"
	"testing"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
)

func TestAuditServiceRecord(t *testing.T) {
	repo := &auditFakeRepository{}
	service := NewAuditService(repo)

	err := service.Record(context.Background(), auditpkg.Event{
		TenantID:     1,
		ProjectID:    2,
		UserID:       3,
		EventType:    auditpkg.EventQueryExecute,
		ResourceType: auditpkg.ResourceQueryExecution,
		ResourceID:   9,
		RequestID:    "rid",
		Payload: map[string]any{
			"status": "success",
		},
	})
	if err != nil {
		t.Fatalf("record audit: %v", err)
	}
	if len(repo.logs) != 1 {
		t.Fatalf("expected one log, got %d", len(repo.logs))
	}
	if repo.logs[0].PayloadJSON == nil || *repo.logs[0].PayloadJSON != `{"status":"success"}` {
		t.Fatalf("unexpected payload: %+v", repo.logs[0].PayloadJSON)
	}
}

func TestAuditServiceRecordRejectsMissingEventType(t *testing.T) {
	service := NewAuditService(&auditFakeRepository{})

	err := service.Record(context.Background(), auditpkg.Event{TenantID: 1})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

type auditFakeRepository struct {
	logs       []model.AuditLog
	lastFilter repository.AuditLogFilter
}

func (r *auditFakeRepository) Create(ctx context.Context, log *model.AuditLog) error {
	log.ID = uint64(len(r.logs) + 1)
	r.logs = append(r.logs, *log)
	return nil
}

func (r *auditFakeRepository) List(ctx context.Context, filter repository.AuditLogFilter, page repository.Page) ([]model.AuditLog, int64, error) {
	r.lastFilter = filter
	return r.logs, int64(len(r.logs)), nil
}

type recordingAuditRecorder struct {
	events []auditpkg.Event
}

func (r *recordingAuditRecorder) Record(ctx context.Context, event auditpkg.Event) error {
	r.events = append(r.events, event)
	return nil
}
