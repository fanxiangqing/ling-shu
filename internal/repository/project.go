package repository

import (
	"context"
	"time"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type ProjectRepository interface {
	Create(ctx context.Context, project *model.Project) error
	CreateWithBindings(ctx context.Context, project *model.Project, datasourceIDs []uint64, createdBy uint64) error
	List(ctx context.Context, tenantID uint64, page Page) ([]model.Project, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Project, error)
	Delete(ctx context.Context, tenantID uint64, projectID uint64) error
}

type GormProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &GormProjectRepository{db: db}
}

func (r *GormProjectRepository) Create(ctx context.Context, project *model.Project) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *GormProjectRepository) CreateWithBindings(ctx context.Context, project *model.Project, datasourceIDs []uint64, createdBy uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	ids := uniqueUint64s(datasourceIDs)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(project).Error; err != nil {
			return err
		}
		if createdBy > 0 {
			if err := tx.Create(&model.ProjectMember{
				TenantID:  project.TenantID,
				ProjectID: project.ID,
				UserID:    createdBy,
				Status:    "active",
			}).Error; err != nil {
				return err
			}
			if err := bindProjectRoleInTx(tx, createdBy, "project_admin", project.TenantID, project.ID, createdBy); err != nil {
				return err
			}
		}
		if len(ids) == 0 {
			return nil
		}
		var count int64
		if err := tx.Model(&model.Datasource{}).
			Where("tenant_id = ? AND id IN ?", project.TenantID, ids).
			Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return gorm.ErrRecordNotFound
		}
		now := time.Now()
		bindings := make([]model.ProjectDatasource, 0, len(ids))
		for _, id := range ids {
			bindings = append(bindings, model.ProjectDatasource{
				TenantID:     project.TenantID,
				ProjectID:    project.ID,
				DatasourceID: id,
				CreatedBy:    createdBy,
				CreatedAt:    now,
			})
		}
		if err := tx.Create(&bindings).Error; err != nil {
			return err
		}
		return nil
	})
}

func bindProjectRoleInTx(tx *gorm.DB, userID uint64, roleCode string, tenantID uint64, projectID uint64, createdBy uint64) error {
	var role model.Role
	if err := tx.First(&role, "code = ?", roleCode).Error; err != nil {
		return err
	}
	return tx.Create(&model.RoleBinding{
		UserID:    userID,
		RoleID:    role.ID,
		TenantID:  tenantID,
		ProjectID: projectID,
		CreatedBy: createdBy,
	}).Error
}

func (r *GormProjectRepository) List(ctx context.Context, tenantID uint64, page Page) ([]model.Project, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}

	query := r.db.WithContext(ctx).Model(&model.Project{})
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var projects []model.Project
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&projects).Error; err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}

func (r *GormProjectRepository) GetByID(ctx context.Context, id uint64) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}

	var project model.Project
	if err := r.db.WithContext(ctx).First(&project, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *GormProjectRepository) Delete(ctx context.Context, tenantID uint64, projectID uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	if projectID == 0 {
		return gorm.ErrRecordNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deleteByProject := func(value any) error {
			query := tx.Unscoped().Where("project_id = ?", projectID)
			if tenantID > 0 {
				query = query.Where("tenant_id = ?", tenantID)
			}
			return query.Delete(value).Error
		}

		for _, value := range []any{
			&model.SQLReviewResult{},
			&model.QueryExecution{},
			&model.ChatMessage{},
			&model.ChatSession{},
			&model.MetadataColumn{},
			&model.MetadataTable{},
			&model.MetadataSchema{},
			&model.MetadataSyncJob{},
			&model.KBTerm{},
			&model.KBMetric{},
			&model.KBFewShotSQL{},
			&model.KBChunk{},
			&model.SensitiveTable{},
			&model.SensitiveColumn{},
			&model.ProjectLLMConfig{},
			&model.ProjectASRConfig{},
			&model.ProjectTTSConfig{},
			&model.ProjectMember{},
			&model.RoleBinding{},
			&model.ProjectDatasource{},
		} {
			if err := deleteByProject(value); err != nil {
				return err
			}
		}

		query := tx.Unscoped().Where("id = ?", projectID)
		if tenantID > 0 {
			query = query.Where("tenant_id = ?", tenantID)
		}
		result := query.Delete(&model.Project{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
