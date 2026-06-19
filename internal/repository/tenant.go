package repository

import (
	"context"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type TenantRepository interface {
	Create(ctx context.Context, tenant *model.Tenant) error
	List(ctx context.Context, page Page) ([]model.Tenant, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Tenant, error)
}

type GormTenantRepository struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) TenantRepository {
	return &GormTenantRepository{db: db}
}

func (r *GormTenantRepository) Create(ctx context.Context, tenant *model.Tenant) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(tenant).Error
}

func (r *GormTenantRepository) List(ctx context.Context, page Page) ([]model.Tenant, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}

	var total int64
	query := r.db.WithContext(ctx).Model(&model.Tenant{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tenants []model.Tenant
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&tenants).Error; err != nil {
		return nil, 0, err
	}

	return tenants, total, nil
}

func (r *GormTenantRepository) GetByID(ctx context.Context, id uint64) (*model.Tenant, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}

	var tenant model.Tenant
	if err := r.db.WithContext(ctx).First(&tenant, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}
