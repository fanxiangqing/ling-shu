package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

type AuditService struct {
	auditRepo repository.AuditRepository
	logger    *zap.Logger
}

type ListAuditLogsInput struct {
	TenantID     uint64
	ProjectID    uint64
	UserID       uint64
	EventType    string
	ResourceType string
	ResourceID   uint64
	StartTime    time.Time
	EndTime      time.Time
	Page         int
	PageSize     int
}

func NewAuditService(auditRepo repository.AuditRepository) *AuditService {
	return &AuditService{auditRepo: auditRepo, logger: zap.NewNop()}
}

func (s *AuditService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *AuditService) Record(ctx context.Context, event auditpkg.Event) error {
	eventType := strings.TrimSpace(event.EventType)
	if eventType == "" {
		return ErrInvalidInput
	}
	payloadJSON, err := marshalAuditPayload(event.Payload)
	if err != nil {
		s.logger.Error("audit payload marshal failed",
			append(auditEventLogFields(event), zap.Error(err))...,
		)
		return err
	}
	log := &model.AuditLog{
		TenantID:     event.TenantID,
		ProjectID:    event.ProjectID,
		UserID:       event.UserID,
		EventType:    eventType,
		ResourceType: strings.TrimSpace(event.ResourceType),
		ResourceID:   event.ResourceID,
		RequestID:    strings.TrimSpace(event.RequestID),
		IP:           strings.TrimSpace(event.IP),
		UserAgent:    strings.TrimSpace(event.UserAgent),
		PayloadJSON:  payloadJSON,
	}
	if err := s.auditRepo.Create(ctx, log); err != nil {
		s.logger.Error("audit log create failed",
			append(auditEventLogFields(event), zap.Error(err))...,
		)
		return err
	}
	s.logger.Debug("audit log created",
		append(auditEventLogFields(event), zap.Uint64("audit_log_id", log.ID))...,
	)
	return nil
}

func (s *AuditService) ListLogs(ctx context.Context, input ListAuditLogsInput) (PageResult[model.AuditLog], error) {
	if input.TenantID == 0 && input.ProjectID == 0 && input.UserID == 0 {
		return PageResult[model.AuditLog]{}, ErrInvalidInput
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.auditRepo.List(ctx, repository.AuditLogFilter{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		UserID:       input.UserID,
		EventType:    strings.TrimSpace(input.EventType),
		ResourceType: strings.TrimSpace(input.ResourceType),
		ResourceID:   input.ResourceID,
		StartTime:    input.StartTime,
		EndTime:      input.EndTime,
	}, p)
	if err != nil {
		s.logger.Error("audit log list failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("user_id", input.UserID),
			zap.String("event_type", strings.TrimSpace(input.EventType)),
			zap.String("resource_type", strings.TrimSpace(input.ResourceType)),
			zap.Uint64("resource_id", input.ResourceID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[model.AuditLog]{}, err
	}
	return PageResult[model.AuditLog]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func auditEventLogFields(event auditpkg.Event) []zap.Field {
	return []zap.Field{
		zap.String("request_id", strings.TrimSpace(event.RequestID)),
		zap.Uint64("tenant_id", event.TenantID),
		zap.Uint64("project_id", event.ProjectID),
		zap.Uint64("user_id", event.UserID),
		zap.String("event_type", strings.TrimSpace(event.EventType)),
		zap.String("resource_type", strings.TrimSpace(event.ResourceType)),
		zap.Uint64("resource_id", event.ResourceID),
	}
}

func marshalAuditPayload(payload map[string]any) (*string, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	out := string(content)
	return &out, nil
}
