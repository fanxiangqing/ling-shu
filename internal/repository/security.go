package repository

import (
	"context"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type SecurityRepository interface {
	ListSensitiveTables(ctx context.Context, filter SensitiveRuleFilter) ([]model.SensitiveTable, error)
	ListSensitiveColumns(ctx context.Context, filter SensitiveRuleFilter) ([]model.SensitiveColumn, error)
}

type SensitiveRuleFilter struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
}

type GormSecurityRepository struct {
	db *gorm.DB
}

func NewSecurityRepository(db *gorm.DB) SecurityRepository {
	return &GormSecurityRepository{db: db}
}

func (r *GormSecurityRepository) ListSensitiveTables(ctx context.Context, filter SensitiveRuleFilter) ([]model.SensitiveTable, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	query := applySensitiveRuleFilter(r.db.WithContext(ctx).Model(&model.SensitiveTable{}), filter)
	var items []model.SensitiveTable
	if err := query.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormSecurityRepository) ListSensitiveColumns(ctx context.Context, filter SensitiveRuleFilter) ([]model.SensitiveColumn, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	query := applySensitiveRuleFilter(r.db.WithContext(ctx).Model(&model.SensitiveColumn{}), filter)
	var items []model.SensitiveColumn
	if err := query.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func applySensitiveRuleFilter(query *gorm.DB, filter SensitiveRuleFilter) *gorm.DB {
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	query = query.Where("enabled = ?", true)
	if filter.DatasourceID > 0 {
		query = query.Where("datasource_id = ? OR datasource_id IS NULL OR datasource_id = 0", filter.DatasourceID)
	}
	return query
}
