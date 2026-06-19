package repository

import (
	"context"
	"time"

	"ling-shu/internal/model"

	"gorm.io/gorm"
)

type QueryRepository interface {
	CreateExecution(ctx context.Context, execution *model.QueryExecution) error
	FinishExecution(ctx context.Context, id uint64, updates QueryExecutionFinish) error
	CreateReviewResult(ctx context.Context, result *model.SQLReviewResult) error
	ListExecutions(ctx context.Context, filter QueryExecutionFilter, page Page) ([]model.QueryExecution, int64, error)
}

type QueryExecutionFinish struct {
	Status            string
	FinalSQL          string
	SQLHash           string
	RowCount          *int
	DurationMS        *int
	ChartType         string
	ResultPreviewJSON *string
	ErrorMessage      string
	FinishedAt        time.Time
}

type QueryExecutionFilter struct {
	TenantID     uint64
	ProjectID    uint64
	UserID       uint64
	DatasourceID uint64
	Status       string
	StartTime    time.Time
	EndTime      time.Time
}

type GormQueryRepository struct {
	db *gorm.DB
}

func NewQueryRepository(db *gorm.DB) QueryRepository {
	return &GormQueryRepository{db: db}
}

func (r *GormQueryRepository) CreateExecution(ctx context.Context, execution *model.QueryExecution) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(execution).Error
}

func (r *GormQueryRepository) FinishExecution(ctx context.Context, id uint64, updates QueryExecutionFinish) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	values := map[string]any{
		"status":      updates.Status,
		"final_sql":   updates.FinalSQL,
		"sql_hash":    updates.SQLHash,
		"finished_at": updates.FinishedAt,
	}
	if updates.RowCount != nil {
		values["row_count"] = *updates.RowCount
	}
	if updates.DurationMS != nil {
		values["duration_ms"] = *updates.DurationMS
	}
	if updates.ChartType != "" {
		values["chart_type"] = updates.ChartType
	}
	if updates.ResultPreviewJSON != nil {
		values["result_preview_json"] = *updates.ResultPreviewJSON
	}
	if updates.ErrorMessage != "" {
		values["error_message"] = updates.ErrorMessage
	}
	return r.db.WithContext(ctx).Model(&model.QueryExecution{}).Where("id = ?", id).Updates(values).Error
}

func (r *GormQueryRepository) CreateReviewResult(ctx context.Context, result *model.SQLReviewResult) error {
	if r.db == nil {
		return ErrDatabaseDisabled
	}
	return r.db.WithContext(ctx).Create(result).Error
}

func (r *GormQueryRepository) ListExecutions(ctx context.Context, filter QueryExecutionFilter, page Page) ([]model.QueryExecution, int64, error) {
	if r.db == nil {
		return nil, 0, ErrDatabaseDisabled
	}

	query := r.db.WithContext(ctx).Model(&model.QueryExecution{})
	if filter.TenantID > 0 {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.ProjectID > 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.DatasourceID > 0 {
		query = query.Where("datasource_id = ?", filter.DatasourceID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
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

	var executions []model.QueryExecution
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.Limit()).Find(&executions).Error; err != nil {
		return nil, 0, err
	}
	return executions, total, nil
}
