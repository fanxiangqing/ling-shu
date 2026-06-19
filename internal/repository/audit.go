package repository

import (
	"context"
	"time"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type AuditRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, filter AuditLogFilter, page Page) ([]model.AuditLog, int64, error)
}

type AuditLogFilter struct {
	TenantID     uint64
	ProjectID    uint64
	UserID       uint64
	EventType    string
	ResourceType string
	ResourceID   uint64
	StartTime    time.Time
	EndTime      time.Time
}

type GormAuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &GormAuditRepository{db: db}
}

func (r *GormAuditRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *GormAuditRepository) List(ctx context.Context, filter AuditLogFilter, page Page) ([]model.AuditLog, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.AuditLog{})
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.EventType != "" {
		query = query.Where("event_type = ?", filter.EventType)
	}
	if filter.ResourceType != "" {
		query = query.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID > 0 {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
