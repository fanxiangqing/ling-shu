package repository

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ling-shu/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DatasourceRepository interface {
	Create(ctx context.Context, datasource *model.Datasource) error
	ListByTenant(ctx context.Context, tenantID uint64, page Page) ([]model.Datasource, int64, error)
	ListByProject(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]model.Datasource, int64, error)
	GetByID(ctx context.Context, id uint64) (*model.Datasource, error)
	BindToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceIDs []uint64, createdBy uint64) error
	IsBoundToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) (bool, error)
	CountProjectReferences(ctx context.Context, tenantID uint64, datasourceID uint64) (int64, error)
	Delete(ctx context.Context, tenantID uint64, datasourceID uint64) error
	UpdateConfigJSON(ctx context.Context, id uint64, configJSON *string) error
	UpdateSyncStatus(ctx context.Context, id uint64, status string, syncedAt *time.Time) error
	CreateSyncJob(ctx context.Context, job *model.MetadataSyncJob) error
	FinishSyncJob(ctx context.Context, id uint64, status string, errorMessage string) error
	ReplaceMetadata(ctx context.Context, datasource *model.Datasource, schemas []model.MetadataSchema, tables []model.MetadataTable) error
	ListMetadataTables(ctx context.Context, datasourceID uint64, page Page, withColumns bool) ([]model.MetadataTable, int64, error)
	GetMetadataTableDetail(ctx context.Context, datasourceID uint64, tableID uint64) (*model.MetadataTable, error)
	GetMetadataColumn(ctx context.Context, datasourceID uint64, columnID uint64) (*model.MetadataColumn, error)
	UpdateMetadataTableComment(ctx context.Context, datasourceID uint64, tableID uint64, comment string) (*model.MetadataTable, error)
	UpdateMetadataColumnComment(ctx context.Context, datasourceID uint64, columnID uint64, comment string) (*model.MetadataColumn, error)
}

type GormDatasourceRepository struct {
	db *gorm.DB
}

func NewDatasourceRepository(db *gorm.DB) DatasourceRepository {
	return &GormDatasourceRepository{db: db}
}

func (r *GormDatasourceRepository) Create(ctx context.Context, datasource *model.Datasource) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(datasource).Error
}

func (r *GormDatasourceRepository) ListByTenant(ctx context.Context, tenantID uint64, page Page) ([]model.Datasource, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.Datasource{}).Where("tenant_id = ?", tenantID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var datasources []model.Datasource
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&datasources).Error; err != nil {
		return nil, 0, err
	}
	return datasources, total, nil
}

func (r *GormDatasourceRepository) ListByProject(ctx context.Context, tenantID uint64, projectID uint64, page Page) ([]model.Datasource, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.Datasource{}).
		Where("(datasources.project_id = ? OR EXISTS (SELECT 1 FROM project_datasources pd WHERE pd.datasource_id = datasources.id AND pd.project_id = ? AND pd.tenant_id = datasources.tenant_id))", projectID, projectID)
	if tenantID > 0 {
		query = query.Where("datasources.tenant_id = ?", tenantID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var datasources []model.Datasource
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&datasources).Error; err != nil {
		return nil, 0, err
	}
	return datasources, total, nil
}

func (r *GormDatasourceRepository) GetByID(ctx context.Context, id uint64) (*model.Datasource, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var datasource model.Datasource
	if err := r.db.WithContext(ctx).First(&datasource, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &datasource, nil
}

func (r *GormDatasourceRepository) BindToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceIDs []uint64, createdBy uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	ids := uniqueUint64s(datasourceIDs)
	if tenantID == 0 || projectID == 0 || len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Datasource{}).
			Where("tenant_id = ? AND id IN ?", tenantID, ids).
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
				TenantID:     tenantID,
				ProjectID:    projectID,
				DatasourceID: id,
				CreatedBy:    createdBy,
				CreatedAt:    now,
			})
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&bindings).Error
	})
}

func (r *GormDatasourceRepository) IsBoundToProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) (bool, error) {
	if r.db == nil {
		return false, ErrDatabaseDisabled
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.ProjectDatasource{}).
		Where("tenant_id = ? AND project_id = ? AND datasource_id = ?", tenantID, projectID, datasourceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := r.db.WithContext(ctx).Model(&model.Datasource{}).
		Where("tenant_id = ? AND project_id = ? AND id = ?", tenantID, projectID, datasourceID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormDatasourceRepository) CountProjectReferences(ctx context.Context, tenantID uint64, datasourceID uint64) (int64, error) {
	if r.db == nil {
		return 0, ErrDatabaseDisabled
	}
	query := r.db.WithContext(ctx).Model(&model.ProjectDatasource{}).Where("datasource_id = ?", datasourceID)
	if tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *GormDatasourceRepository) Delete(ctx context.Context, tenantID uint64, datasourceID uint64) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	if datasourceID == 0 {
		return gorm.ErrRecordNotFound
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		deleteByDatasource := func(value any) error {
			query := tx.Unscoped().Where("datasource_id = ?", datasourceID)
			if tenantID > 0 {
				query = query.Where("tenant_id = ?", tenantID)
			}
			return query.Delete(value).Error
		}

		for _, value := range []any{
			&model.MetadataForeignKey{},
			&model.MetadataIndex{},
			&model.MetadataColumn{},
			&model.MetadataTable{},
			&model.MetadataSchema{},
			&model.MetadataSyncJob{},
			&model.ProjectDatasource{},
			&model.KBMetric{},
			&model.KBFewShotSQL{},
			&model.KBChunk{},
			&model.SensitiveTable{},
			&model.SensitiveColumn{},
		} {
			if err := deleteByDatasource(value); err != nil {
				return err
			}
		}

		query := tx.Unscoped().Where("id = ?", datasourceID)
		if tenantID > 0 {
			query = query.Where("tenant_id = ?", tenantID)
		}
		result := query.Delete(&model.Datasource{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *GormDatasourceRepository) UpdateConfigJSON(ctx context.Context, id uint64, configJSON *string) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Model(&model.Datasource{}).Where("id = ?", id).Update("config_json", configJSON).Error
}

func (r *GormDatasourceRepository) UpdateSyncStatus(ctx context.Context, id uint64, status string, syncedAt *time.Time) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	updates := map[string]any{"last_sync_status": status}
	if syncedAt != nil {
		updates["last_sync_at"] = *syncedAt
	}
	return r.db.WithContext(ctx).Model(&model.Datasource{}).Where("id = ?", id).Updates(updates).Error
}

func (r *GormDatasourceRepository) CreateSyncJob(ctx context.Context, job *model.MetadataSyncJob) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *GormDatasourceRepository) FinishSyncJob(ctx context.Context, id uint64, status string, errorMessage string) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.MetadataSyncJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"error_message": errorMessage,
		"finished_at":   now,
	}).Error
}

func (r *GormDatasourceRepository) ReplaceMetadata(ctx context.Context, datasource *model.Datasource, schemas []model.MetadataSchema, tables []model.MetadataTable) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tableComments, columnComments, err := loadExistingMetadataComments(tx, datasource.ID)
		if err != nil {
			return err
		}
		if err := tx.Where("datasource_id = ?", datasource.ID).Delete(&model.MetadataForeignKey{}).Error; err != nil {
			return err
		}
		if err := tx.Where("datasource_id = ?", datasource.ID).Delete(&model.MetadataIndex{}).Error; err != nil {
			return err
		}
		if err := tx.Where("datasource_id = ?", datasource.ID).Delete(&model.MetadataColumn{}).Error; err != nil {
			return err
		}
		if err := tx.Where("datasource_id = ?", datasource.ID).Delete(&model.MetadataTable{}).Error; err != nil {
			return err
		}
		if err := tx.Where("datasource_id = ?", datasource.ID).Delete(&model.MetadataSchema{}).Error; err != nil {
			return err
		}

		if len(schemas) > 0 {
			if err := tx.Create(&schemas).Error; err != nil {
				return err
			}
		}
		for _, table := range tables {
			columns := table.Columns
			indexes := table.Indexes
			foreignKeys := table.ForeignKeys
			if comment := tableComments[metadataTableCommentKey(table.SchemaName, table.Name)]; strings.TrimSpace(comment) != "" {
				table.BusinessCommentText = comment
				table.CommentText = comment
			} else if strings.TrimSpace(table.CommentText) == "" {
				table.CommentText = table.OriginalCommentText
			}
			table.Columns = nil
			table.Indexes = nil
			table.ForeignKeys = nil
			if err := tx.Create(&table).Error; err != nil {
				return err
			}
			for i := range columns {
				columns[i].TableID = table.ID
				if comment := columnComments[metadataColumnCommentKey(table.SchemaName, table.Name, columns[i].ColumnName)]; strings.TrimSpace(comment) != "" {
					columns[i].BusinessCommentText = comment
					columns[i].CommentText = comment
				} else if strings.TrimSpace(columns[i].CommentText) == "" {
					columns[i].CommentText = columns[i].OriginalCommentText
				}
			}
			if len(columns) > 0 {
				if err := tx.Create(&columns).Error; err != nil {
					return err
				}
			}
			for i := range indexes {
				indexes[i].TableID = table.ID
			}
			if len(indexes) > 0 {
				if err := tx.Create(&indexes).Error; err != nil {
					return err
				}
			}
			for i := range foreignKeys {
				foreignKeys[i].TableID = table.ID
			}
			if len(foreignKeys) > 0 {
				if err := tx.Create(&foreignKeys).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *GormDatasourceRepository) ListMetadataTables(ctx context.Context, datasourceID uint64, page Page, withColumns bool) ([]model.MetadataTable, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}

	query := r.db.WithContext(ctx).Model(&model.MetadataTable{}).Where("datasource_id = ?", datasourceID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if withColumns {
		query = query.Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("ordinal_position ASC")
		}).Preload("Indexes", func(db *gorm.DB) *gorm.DB {
			return db.Order("index_name ASC")
		}).Preload("ForeignKeys", func(db *gorm.DB) *gorm.DB {
			return db.Order("constraint_name ASC, column_name ASC")
		})
	}

	var tables []model.MetadataTable
	if err := query.Order("schema_name ASC, table_name ASC").Offset(page.Offset()).Limit(page.Limit()).Find(&tables).Error; err != nil {
		return nil, 0, err
	}
	return tables, total, nil
}

func (r *GormDatasourceRepository) GetMetadataTableDetail(ctx context.Context, datasourceID uint64, tableID uint64) (*model.MetadataTable, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var table model.MetadataTable
	if err := r.db.WithContext(ctx).
		Preload("Columns", func(db *gorm.DB) *gorm.DB {
			return db.Order("ordinal_position ASC")
		}).
		Preload("Indexes", func(db *gorm.DB) *gorm.DB {
			return db.Order("index_name ASC")
		}).
		Preload("ForeignKeys", func(db *gorm.DB) *gorm.DB {
			return db.Order("constraint_name ASC, column_name ASC")
		}).
		First(&table, "datasource_id = ? AND id = ?", datasourceID, tableID).Error; err != nil {
		return nil, err
	}
	return &table, nil
}

func (r *GormDatasourceRepository) GetMetadataColumn(ctx context.Context, datasourceID uint64, columnID uint64) (*model.MetadataColumn, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var column model.MetadataColumn
	if err := r.db.WithContext(ctx).First(&column, "datasource_id = ? AND id = ?", datasourceID, columnID).Error; err != nil {
		return nil, err
	}
	return &column, nil
}

func (r *GormDatasourceRepository) UpdateMetadataTableComment(ctx context.Context, datasourceID uint64, tableID uint64, comment string) (*model.MetadataTable, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var table model.MetadataTable
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&table, "datasource_id = ? AND id = ?", datasourceID, tableID).Error; err != nil {
			return err
		}
		return tx.Model(&model.MetadataTable{}).
			Where("datasource_id = ? AND id = ?", datasourceID, tableID).
			Updates(map[string]any{
				"comment_text":          comment,
				"business_comment_text": comment,
			}).Error
	}); err != nil {
		return nil, err
	}
	table.CommentText = comment
	table.BusinessCommentText = comment
	return &table, nil
}

func (r *GormDatasourceRepository) UpdateMetadataColumnComment(ctx context.Context, datasourceID uint64, columnID uint64, comment string) (*model.MetadataColumn, error) {
	if r.db == nil {
		return nil, ErrDatabaseDisabled
	}
	var column model.MetadataColumn
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&column, "datasource_id = ? AND id = ?", datasourceID, columnID).Error; err != nil {
			return err
		}
		return tx.Model(&model.MetadataColumn{}).
			Where("datasource_id = ? AND id = ?", datasourceID, columnID).
			Updates(map[string]any{
				"comment_text":          comment,
				"business_comment_text": comment,
			}).Error
	}); err != nil {
		return nil, err
	}
	column.CommentText = comment
	column.BusinessCommentText = comment
	return &column, nil
}

func loadExistingMetadataComments(tx *gorm.DB, datasourceID uint64) (map[string]string, map[string]string, error) {
	tableComments := map[string]string{}
	columnComments := map[string]string{}
	var tables []model.MetadataTable
	if err := tx.Preload("Columns").Where("datasource_id = ?", datasourceID).Find(&tables).Error; err != nil {
		return nil, nil, err
	}
	for _, table := range tables {
		comment := strings.TrimSpace(table.BusinessCommentText)
		if comment == "" {
			comment = strings.TrimSpace(table.CommentText)
		}
		if comment != "" {
			tableComments[metadataTableCommentKey(table.SchemaName, table.Name)] = comment
		}
		for _, column := range table.Columns {
			comment := strings.TrimSpace(column.BusinessCommentText)
			if comment == "" {
				comment = strings.TrimSpace(column.CommentText)
			}
			if comment != "" {
				columnComments[metadataColumnCommentKey(table.SchemaName, table.Name, column.ColumnName)] = comment
			}
		}
	}
	return tableComments, columnComments, nil
}

func metadataTableCommentKey(schemaName string, tableName string) string {
	return strings.ToLower(strings.TrimSpace(schemaName)) + "." + strings.ToLower(strings.TrimSpace(tableName))
}

func metadataColumnCommentKey(schemaName string, tableName string, columnName string) string {
	return metadataTableCommentKey(schemaName, tableName) + "." + strings.ToLower(strings.TrimSpace(columnName))
}

func MetadataIndexColumnsJSON(columns []string) string {
	data, err := json.Marshal(columns)
	if err != nil || len(data) == 0 {
		return "[]"
	}
	return string(data)
}

func uniqueUint64s(values []uint64) []uint64 {
	out := make([]uint64, 0, len(values))
	seen := map[uint64]bool{}
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
