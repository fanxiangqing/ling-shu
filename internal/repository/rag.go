package repository

import (
	"context"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type RAGRepository interface {
	ReplaceChunks(ctx context.Context, tenantID uint64, projectID uint64, chunks []model.KBChunk) error
	ListChunks(ctx context.Context, filter RAGChunkFilter, page Page) ([]model.KBChunk, int64, error)
}

type RAGChunkFilter struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	KBType       string
}

type GormRAGRepository struct {
	db *gorm.DB
}

func NewRAGRepository(db *gorm.DB) RAGRepository {
	return &GormRAGRepository{db: db}
}

func (r *GormRAGRepository) ReplaceChunks(ctx context.Context, tenantID uint64, projectID uint64, chunks []model.KBChunk) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND project_id = ?", tenantID, projectID).Delete(&model.KBChunk{}).Error; err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}
		return tx.Create(&chunks).Error
	})
}

func (r *GormRAGRepository) ListChunks(ctx context.Context, filter RAGChunkFilter, page Page) ([]model.KBChunk, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.KBChunk{})
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.DatasourceID > 0 {
		query = query.Where("datasource_id = ? OR datasource_id IS NULL OR datasource_id = 0", filter.DatasourceID)
	}
	if filter.KBType != "" {
		query = query.Where("kb_type = ?", filter.KBType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var chunks []model.KBChunk
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&chunks).Error; err != nil {
		return nil, 0, err
	}
	return chunks, total, nil
}
