package repository

import (
	"context"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type KnowledgeRepository interface {
	CreateTerm(ctx context.Context, term *model.KBTerm) error
	ListTerms(ctx context.Context, filter KnowledgeFilter, page Page) ([]model.KBTerm, int64, error)
	UpdateTermEnabled(ctx context.Context, scope KnowledgeItemScope, enabled bool) error
	DeleteTerm(ctx context.Context, scope KnowledgeItemScope) error
	CreateMetric(ctx context.Context, metric *model.KBMetric) error
	ListMetrics(ctx context.Context, filter KnowledgeFilter, page Page) ([]model.KBMetric, int64, error)
	UpdateMetricEnabled(ctx context.Context, scope KnowledgeItemScope, enabled bool) error
	DeleteMetric(ctx context.Context, scope KnowledgeItemScope) error
	CreateFewShot(ctx context.Context, fewShot *model.KBFewShotSQL) error
	ListFewShots(ctx context.Context, filter KnowledgeFilter, page Page) ([]model.KBFewShotSQL, int64, error)
	UpdateFewShotEnabled(ctx context.Context, scope KnowledgeItemScope, enabled bool) error
	DeleteFewShot(ctx context.Context, scope KnowledgeItemScope) error
}

type KnowledgeFilter struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	Enabled      *bool
}

type KnowledgeItemScope struct {
	TenantID  uint64
	ProjectID uint64
	ID        uint64
}

type GormKnowledgeRepository struct {
	db *gorm.DB
}

func NewKnowledgeRepository(db *gorm.DB) KnowledgeRepository {
	return &GormKnowledgeRepository{db: db}
}

func (r *GormKnowledgeRepository) CreateTerm(ctx context.Context, term *model.KBTerm) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(term).Error
}

func (r *GormKnowledgeRepository) ListTerms(ctx context.Context, filter KnowledgeFilter, page Page) ([]model.KBTerm, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := applyKnowledgeBaseFilter(r.db.WithContext(ctx).Model(&model.KBTerm{}), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.KBTerm
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormKnowledgeRepository) UpdateTermEnabled(ctx context.Context, scope KnowledgeItemScope, enabled bool) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return updateKnowledgeEnabled(ctx, r.db, &model.KBTerm{}, scope, enabled)
}

func (r *GormKnowledgeRepository) DeleteTerm(ctx context.Context, scope KnowledgeItemScope) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return deleteKnowledgeItem(ctx, r.db, &model.KBTerm{}, scope)
}

func (r *GormKnowledgeRepository) CreateMetric(ctx context.Context, metric *model.KBMetric) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(metric).Error
}

func (r *GormKnowledgeRepository) ListMetrics(ctx context.Context, filter KnowledgeFilter, page Page) ([]model.KBMetric, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := applyKnowledgeBaseFilter(r.db.WithContext(ctx).Model(&model.KBMetric{}), filter)
	if filter.DatasourceID > 0 {
		query = query.Where("datasource_id = ? OR datasource_id IS NULL OR datasource_id = 0", filter.DatasourceID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.KBMetric
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormKnowledgeRepository) UpdateMetricEnabled(ctx context.Context, scope KnowledgeItemScope, enabled bool) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return updateKnowledgeEnabled(ctx, r.db, &model.KBMetric{}, scope, enabled)
}

func (r *GormKnowledgeRepository) DeleteMetric(ctx context.Context, scope KnowledgeItemScope) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return deleteKnowledgeItem(ctx, r.db, &model.KBMetric{}, scope)
}

func (r *GormKnowledgeRepository) CreateFewShot(ctx context.Context, fewShot *model.KBFewShotSQL) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(fewShot).Error
}

func (r *GormKnowledgeRepository) ListFewShots(ctx context.Context, filter KnowledgeFilter, page Page) ([]model.KBFewShotSQL, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := applyKnowledgeBaseFilter(r.db.WithContext(ctx).Model(&model.KBFewShotSQL{}), filter)
	if filter.DatasourceID > 0 {
		query = query.Where("datasource_id = ? OR datasource_id IS NULL OR datasource_id = 0", filter.DatasourceID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.KBFewShotSQL
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormKnowledgeRepository) UpdateFewShotEnabled(ctx context.Context, scope KnowledgeItemScope, enabled bool) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return updateKnowledgeEnabled(ctx, r.db, &model.KBFewShotSQL{}, scope, enabled)
}

func (r *GormKnowledgeRepository) DeleteFewShot(ctx context.Context, scope KnowledgeItemScope) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return deleteKnowledgeItem(ctx, r.db, &model.KBFewShotSQL{}, scope)
}

func applyKnowledgeBaseFilter(query *gorm.DB, filter KnowledgeFilter) *gorm.DB {
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	return query
}

func applyKnowledgeItemScope(query *gorm.DB, scope KnowledgeItemScope) *gorm.DB {
	return query.Where("id = ? AND tenant_id = ? AND project_id = ?", scope.ID, scope.TenantID, scope.ProjectID)
}

func updateKnowledgeEnabled(ctx context.Context, db *gorm.DB, modelValue any, scope KnowledgeItemScope, enabled bool) error {
	result := applyKnowledgeItemScope(db.WithContext(ctx).Model(modelValue), scope).Update("enabled", enabled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func deleteKnowledgeItem(ctx context.Context, db *gorm.DB, modelValue any, scope KnowledgeItemScope) error {
	result := applyKnowledgeItemScope(db.WithContext(ctx).Unscoped(), scope).Delete(modelValue)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
