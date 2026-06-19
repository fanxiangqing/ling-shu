package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	auditpkg "ling-shu/internal/audit"
	dsdriver "ling-shu/internal/datasource"
	"ling-shu/internal/model"
	"ling-shu/internal/repository"
	"ling-shu/pkg/secret"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DatasourceService struct {
	datasourceRepo repository.DatasourceRepository
	registry       *dsdriver.Registry
	dsnCodec       secret.Codec
	auditRecorder  auditpkg.Recorder
	logger         *zap.Logger
}

type CreateDatasourceInput struct {
	TenantID   uint64
	ProjectID  uint64
	Name       string
	DBType     string
	DSN        string
	ConfigJSON string
	CreatedBy  uint64
}

type TestDatasourceConnectionInput struct {
	DBType     string
	DSN        string
	ConfigJSON string
}

type TestDatasourceConnectionResult struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

type DeleteDatasourceInput struct {
	TenantID     uint64
	DatasourceID uint64
}

type SyncMetadataInput struct {
	DatasourceID uint64
	TriggerType  string
	UserID       uint64
}

type SyncMetadataResult struct {
	JobID           uint64    `json:"job_id"`
	Status          string    `json:"status"`
	SchemaCount     int       `json:"schema_count"`
	TableCount      int       `json:"table_count"`
	ColumnCount     int       `json:"column_count"`
	IndexCount      int       `json:"index_count"`
	ForeignKeyCount int       `json:"foreign_key_count"`
	SyncedAt        time.Time `json:"synced_at"`
	ErrorMessage    string    `json:"error_message,omitempty"`
}

type UpdateMetadataCommentInput struct {
	DatasourceID uint64
	TableID      uint64
	ColumnID     uint64
	Comment      string
	UserID       uint64
	RequestID    string
	IP           string
	UserAgent    string
}

func NewDatasourceService(datasourceRepo repository.DatasourceRepository, registry *dsdriver.Registry) *DatasourceService {
	if registry == nil {
		registry = dsdriver.DefaultRegistry()
	}
	return &DatasourceService{
		datasourceRepo: datasourceRepo,
		registry:       registry,
		dsnCodec:       secret.PlainCodec{},
		logger:         zap.NewNop(),
	}
}

func (s *DatasourceService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	s.logger = logger
}

func (s *DatasourceService) SetDSNCodec(codec secret.Codec) {
	if codec == nil {
		codec = secret.PlainCodec{}
	}
	s.dsnCodec = codec
}

func (s *DatasourceService) SetAuditRecorder(recorder auditpkg.Recorder) {
	s.auditRecorder = recorder
}

func (s *DatasourceService) Create(ctx context.Context, input CreateDatasourceInput) (*model.Datasource, error) {
	name := strings.TrimSpace(input.Name)
	dbType := strings.ToLower(strings.TrimSpace(input.DBType))
	dsn := strings.TrimSpace(input.DSN)
	if input.TenantID == 0 || name == "" || dbType == "" || dsn == "" {
		return nil, ErrInvalidInput
	}
	if _, err := s.registry.Driver(dbType); err != nil {
		s.logger.Warn("datasource driver resolve failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("db_type", dbType),
			zap.Error(err),
		)
		return nil, err
	}

	dsnCiphertext, err := encryptSecret(s.dsnCodec, dsn)
	if err != nil {
		s.logger.Error("datasource dsn encrypt failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("db_type", dbType),
			zap.String("name_hash", sqlHash(name)),
			zap.Error(err),
		)
		return nil, err
	}
	configJSON := configStringPtr(input.ConfigJSON)
	datasource := &model.Datasource{
		TenantID:      input.TenantID,
		ProjectID:     input.ProjectID,
		Name:          name,
		DBType:        dbType,
		DSNCiphertext: dsnCiphertext,
		ConfigJSON:    configJSON,
		Status:        "active",
		CreatedBy:     input.CreatedBy,
	}
	if err := s.datasourceRepo.Create(ctx, datasource); err != nil {
		s.logger.Error("datasource create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.String("db_type", dbType),
			zap.String("name_hash", sqlHash(name)),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("datasource created",
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("project_id", datasource.ProjectID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.String("db_type", datasource.DBType),
	)
	return datasource, nil
}

func (s *DatasourceService) List(ctx context.Context, tenantID uint64, projectID uint64, page int, pageSize int) (PageResult[model.Datasource], error) {
	if tenantID == 0 {
		return PageResult[model.Datasource]{}, ErrInvalidInput
	}
	p := NewPage(page, pageSize)
	var (
		items []model.Datasource
		total int64
		err   error
	)
	if projectID > 0 {
		items, total, err = s.datasourceRepo.ListByProject(ctx, tenantID, projectID, p)
	} else {
		items, total, err = s.datasourceRepo.ListByTenant(ctx, tenantID, p)
	}
	if err != nil {
		s.logger.Error("datasource list failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[model.Datasource]{}, err
	}
	return PageResult[model.Datasource]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func (s *DatasourceService) ResolveDatasourceScope(ctx context.Context, datasourceID uint64) (uint64, uint64, error) {
	if datasourceID == 0 {
		return 0, 0, ErrInvalidInput
	}
	datasource, err := s.datasourceRepo.GetByID(ctx, datasourceID)
	if err != nil {
		s.logger.Warn("datasource scope resolve failed",
			zap.Uint64("datasource_id", datasourceID),
			zap.Error(err),
		)
		return 0, 0, err
	}
	return datasource.TenantID, datasource.ProjectID, nil
}

func (s *DatasourceService) IsDatasourceAvailableForProject(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64) (bool, error) {
	if tenantID == 0 || projectID == 0 || datasourceID == 0 {
		return false, ErrInvalidInput
	}
	datasource, err := s.datasourceRepo.GetByID(ctx, datasourceID)
	if err != nil {
		s.logger.Warn("datasource availability load failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Uint64("datasource_id", datasourceID),
			zap.Error(err),
		)
		return false, err
	}
	if datasource.TenantID != tenantID {
		return false, nil
	}
	if datasource.ProjectID == projectID {
		return true, nil
	}
	available, err := s.datasourceRepo.IsBoundToProject(ctx, tenantID, projectID, datasourceID)
	if err != nil {
		s.logger.Error("datasource project binding check failed",
			zap.Uint64("tenant_id", tenantID),
			zap.Uint64("project_id", projectID),
			zap.Uint64("datasource_id", datasourceID),
			zap.Error(err),
		)
		return false, err
	}
	return available, nil
}

func (s *DatasourceService) TestConnection(ctx context.Context, datasourceID uint64) (*TestDatasourceConnectionResult, error) {
	startedAt := time.Now()
	datasource, err := s.datasourceRepo.GetByID(ctx, datasourceID)
	if err != nil {
		s.logDatasourceOperationFailed("datasource connection test load failed", nil, startedAt, err)
		return nil, err
	}
	driver, err := s.registry.Driver(datasource.DBType)
	if err != nil {
		s.logDatasourceOperationFailed("datasource connection test driver resolve failed", datasource, startedAt, err)
		return nil, err
	}
	dsn, err := decryptSecret(s.dsnCodec, datasource.DSNCiphertext)
	if err != nil {
		s.logDatasourceOperationFailed("datasource connection test failed", datasource, startedAt, err)
		return nil, err
	}
	if err := driver.Ping(ctx, dsdriver.Config{DSN: dsn}); err != nil {
		s.logDatasourceOperationFailed("datasource connection test failed", datasource, startedAt, err)
		return nil, err
	}
	version := detectDatasourceVersion(ctx, driver, dsn)
	if nextConfig, ok := mergeDatasourceConfigVersion(datasource.ConfigJSON, version); ok {
		_ = s.datasourceRepo.UpdateConfigJSON(ctx, datasource.ID, nextConfig)
	}
	s.logger.Info("datasource connection test succeeded",
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("project_id", datasource.ProjectID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.String("db_type", datasource.DBType),
		zap.String("version", version),
		zap.Duration("duration", time.Since(startedAt)),
	)
	return &TestDatasourceConnectionResult{Status: "ok", Version: version}, nil
}

func (s *DatasourceService) TestConnectionWithConfig(ctx context.Context, input TestDatasourceConnectionInput) (*TestDatasourceConnectionResult, error) {
	startedAt := time.Now()
	dbType := strings.ToLower(strings.TrimSpace(input.DBType))
	dsn := strings.TrimSpace(input.DSN)
	if dbType == "" || dsn == "" {
		return nil, ErrInvalidInput
	}
	driver, err := s.registry.Driver(dbType)
	if err != nil {
		s.logger.Warn("datasource connection config driver resolve failed",
			zap.String("db_type", dbType),
			zap.Duration("duration", time.Since(startedAt)),
			zap.Error(err),
		)
		return nil, err
	}
	if err := driver.Ping(ctx, dsdriver.Config{DSN: dsn}); err != nil {
		s.logger.Warn("datasource connection config test failed",
			zap.String("db_type", dbType),
			zap.Duration("duration", time.Since(startedAt)),
			zap.Error(err),
		)
		return nil, err
	}
	version := detectDatasourceVersion(ctx, driver, dsn)
	s.logger.Info("datasource connection config test succeeded",
		zap.String("db_type", dbType),
		zap.String("version", version),
		zap.Duration("duration", time.Since(startedAt)),
	)
	return &TestDatasourceConnectionResult{Status: "ok", Version: version}, nil
}

func (s *DatasourceService) Delete(ctx context.Context, input DeleteDatasourceInput) error {
	if input.DatasourceID == 0 {
		return ErrInvalidInput
	}
	datasource, err := s.datasourceRepo.GetByID(ctx, input.DatasourceID)
	if err != nil {
		s.logger.Warn("datasource delete load failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Error(err),
		)
		return err
	}
	if input.TenantID > 0 {
		if datasource.TenantID != input.TenantID {
			s.logger.Warn("datasource delete tenant scope rejected",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("actual_tenant_id", datasource.TenantID),
				zap.Uint64("datasource_id", input.DatasourceID),
			)
			return gorm.ErrRecordNotFound
		}
	}
	if datasource.ProjectID > 0 {
		s.logger.Info("datasource delete rejected because datasource has project owner",
			zap.Uint64("tenant_id", datasource.TenantID),
			zap.Uint64("project_id", datasource.ProjectID),
			zap.Uint64("datasource_id", datasource.ID),
		)
		return ErrDatasourceInUse
	}
	references, err := s.datasourceRepo.CountProjectReferences(ctx, datasource.TenantID, input.DatasourceID)
	if err != nil {
		s.logger.Error("datasource delete reference count failed",
			zap.Uint64("tenant_id", datasource.TenantID),
			zap.Uint64("datasource_id", datasource.ID),
			zap.String("db_type", datasource.DBType),
			zap.Error(err),
		)
		return err
	}
	if references > 0 {
		s.logger.Info("datasource delete rejected because datasource is referenced",
			zap.Uint64("tenant_id", datasource.TenantID),
			zap.Uint64("datasource_id", datasource.ID),
			zap.String("db_type", datasource.DBType),
			zap.Int64("references", references),
		)
		return ErrDatasourceInUse
	}
	if err := s.datasourceRepo.Delete(ctx, input.TenantID, input.DatasourceID); err != nil {
		s.logger.Error("datasource delete failed",
			zap.Uint64("tenant_id", datasource.TenantID),
			zap.Uint64("datasource_id", datasource.ID),
			zap.String("db_type", datasource.DBType),
			zap.Error(err),
		)
		return err
	}
	s.logger.Info("datasource deleted",
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.String("db_type", datasource.DBType),
	)
	return nil
}

func (s *DatasourceService) SyncMetadata(ctx context.Context, input SyncMetadataInput) (*SyncMetadataResult, error) {
	startedAt := time.Now()
	if input.DatasourceID == 0 {
		return nil, ErrInvalidInput
	}
	triggerType := strings.TrimSpace(input.TriggerType)
	if triggerType == "" {
		triggerType = "manual"
	}

	datasource, err := s.datasourceRepo.GetByID(ctx, input.DatasourceID)
	if err != nil {
		s.logger.Warn("metadata sync datasource load failed",
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.String("trigger_type", triggerType),
			zap.Error(err),
		)
		return nil, err
	}
	driver, err := s.registry.Driver(datasource.DBType)
	if err != nil {
		s.logMetadataSyncFailed(datasource, 0, startedAt, err)
		return nil, err
	}

	now := time.Now()
	job := &model.MetadataSyncJob{
		TenantID:     datasource.TenantID,
		ProjectID:    datasource.ProjectID,
		DatasourceID: datasource.ID,
		TriggerType:  triggerType,
		Status:       "running",
		StartedAt:    &now,
		CreatedBy:    input.UserID,
	}
	if err := s.datasourceRepo.CreateSyncJob(ctx, job); err != nil {
		s.logMetadataSyncFailed(datasource, 0, startedAt, err)
		return nil, err
	}
	s.logger.Info("metadata sync started",
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("project_id", datasource.ProjectID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.Uint64("job_id", job.ID),
		zap.String("db_type", datasource.DBType),
		zap.String("trigger_type", triggerType),
	)

	dsn, err := decryptSecret(s.dsnCodec, datasource.DSNCiphertext)
	if err != nil {
		_ = s.datasourceRepo.FinishSyncJob(ctx, job.ID, "failed", err.Error())
		_ = s.datasourceRepo.UpdateSyncStatus(ctx, datasource.ID, "failed", nil)
		s.logMetadataSyncFailed(datasource, job.ID, startedAt, err)
		return &SyncMetadataResult{JobID: job.ID, Status: "failed", ErrorMessage: err.Error()}, err
	}

	metadata, err := driver.Introspect(ctx, dsdriver.Config{DSN: dsn})
	if err != nil {
		_ = s.datasourceRepo.FinishSyncJob(ctx, job.ID, "failed", err.Error())
		_ = s.datasourceRepo.UpdateSyncStatus(ctx, datasource.ID, "failed", nil)
		s.logMetadataSyncFailed(datasource, job.ID, startedAt, err)
		return &SyncMetadataResult{JobID: job.ID, Status: "failed", ErrorMessage: err.Error()}, err
	}

	schemas, tables, columnCount, indexCount, foreignKeyCount := metadataToModels(datasource, metadata, now)
	if err := s.datasourceRepo.ReplaceMetadata(ctx, datasource, schemas, tables); err != nil {
		_ = s.datasourceRepo.FinishSyncJob(ctx, job.ID, "failed", err.Error())
		_ = s.datasourceRepo.UpdateSyncStatus(ctx, datasource.ID, "failed", nil)
		s.logMetadataSyncFailed(datasource, job.ID, startedAt, err)
		return &SyncMetadataResult{JobID: job.ID, Status: "failed", ErrorMessage: err.Error()}, err
	}
	if nextConfig, ok := mergeDatasourceConfigVersion(datasource.ConfigJSON, metadata.Version); ok {
		if err := s.datasourceRepo.UpdateConfigJSON(ctx, datasource.ID, nextConfig); err != nil {
			_ = s.datasourceRepo.FinishSyncJob(ctx, job.ID, "failed", err.Error())
			_ = s.datasourceRepo.UpdateSyncStatus(ctx, datasource.ID, "failed", nil)
			s.logMetadataSyncFailed(datasource, job.ID, startedAt, err)
			return &SyncMetadataResult{JobID: job.ID, Status: "failed", ErrorMessage: err.Error()}, err
		}
	}
	if err := s.datasourceRepo.FinishSyncJob(ctx, job.ID, "success", ""); err != nil {
		s.logMetadataSyncFailed(datasource, job.ID, startedAt, err)
		return nil, err
	}
	if err := s.datasourceRepo.UpdateSyncStatus(ctx, datasource.ID, "success", &now); err != nil {
		s.logMetadataSyncFailed(datasource, job.ID, startedAt, err)
		return nil, err
	}

	s.logger.Info("metadata sync succeeded",
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("project_id", datasource.ProjectID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.Uint64("job_id", job.ID),
		zap.String("db_type", datasource.DBType),
		zap.Int("schema_count", len(schemas)),
		zap.Int("table_count", len(tables)),
		zap.Int("column_count", columnCount),
		zap.Int("index_count", indexCount),
		zap.Int("foreign_key_count", foreignKeyCount),
		zap.String("version", metadata.Version),
		zap.Duration("duration", time.Since(startedAt)),
	)
	return &SyncMetadataResult{
		JobID:           job.ID,
		Status:          "success",
		SchemaCount:     len(schemas),
		TableCount:      len(tables),
		ColumnCount:     columnCount,
		IndexCount:      indexCount,
		ForeignKeyCount: foreignKeyCount,
		SyncedAt:        now,
	}, nil
}

func (s *DatasourceService) logDatasourceOperationFailed(message string, datasource *model.Datasource, startedAt time.Time, err error) {
	if datasource == nil {
		s.logger.Warn(message, zap.Duration("duration", time.Since(startedAt)), zap.Error(err))
		return
	}
	s.logger.Warn(message,
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("project_id", datasource.ProjectID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.String("db_type", datasource.DBType),
		zap.Duration("duration", time.Since(startedAt)),
		zap.Error(err),
	)
}

func (s *DatasourceService) logMetadataSyncFailed(datasource *model.Datasource, jobID uint64, startedAt time.Time, err error) {
	s.logger.Warn("metadata sync failed",
		zap.Uint64("tenant_id", datasource.TenantID),
		zap.Uint64("project_id", datasource.ProjectID),
		zap.Uint64("datasource_id", datasource.ID),
		zap.Uint64("job_id", jobID),
		zap.String("db_type", datasource.DBType),
		zap.Duration("duration", time.Since(startedAt)),
		zap.Error(err),
	)
}

func (s *DatasourceService) ListMetadataTables(ctx context.Context, datasourceID uint64, page int, pageSize int, withColumns bool) (PageResult[model.MetadataTable], error) {
	if datasourceID == 0 {
		return PageResult[model.MetadataTable]{}, ErrInvalidInput
	}
	p := NewPage(page, pageSize)
	items, total, err := s.datasourceRepo.ListMetadataTables(ctx, datasourceID, p, withColumns)
	if err != nil {
		s.logger.Error("metadata table list failed",
			zap.Uint64("datasource_id", datasourceID),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Bool("with_columns", withColumns),
			zap.Error(err),
		)
		return PageResult[model.MetadataTable]{}, err
	}
	return PageResult[model.MetadataTable]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func (s *DatasourceService) GetMetadataTableDetail(ctx context.Context, datasourceID uint64, tableID uint64) (*model.MetadataTable, error) {
	if datasourceID == 0 || tableID == 0 {
		return nil, ErrInvalidInput
	}
	table, err := s.datasourceRepo.GetMetadataTableDetail(ctx, datasourceID, tableID)
	if err != nil {
		s.logger.Error("metadata table detail load failed",
			zap.Uint64("datasource_id", datasourceID),
			zap.Uint64("table_id", tableID),
			zap.Error(err),
		)
		return nil, err
	}
	return table, nil
}

func (s *DatasourceService) UpdateMetadataTableComment(ctx context.Context, input UpdateMetadataCommentInput) (*model.MetadataTable, error) {
	if input.DatasourceID == 0 || input.TableID == 0 {
		return nil, ErrInvalidInput
	}
	current, err := s.datasourceRepo.GetMetadataTableDetail(ctx, input.DatasourceID, input.TableID)
	if err != nil {
		s.logger.Error("metadata table comment load failed",
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("table_id", input.TableID),
			zap.Uint64("user_id", input.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	table, err := s.datasourceRepo.UpdateMetadataTableComment(ctx, input.DatasourceID, input.TableID, strings.TrimSpace(input.Comment))
	if err != nil {
		s.logger.Error("metadata table comment update failed",
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("table_id", input.TableID),
			zap.Uint64("user_id", input.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("metadata table comment updated",
		zap.Uint64("tenant_id", table.TenantID),
		zap.Uint64("project_id", table.ProjectID),
		zap.Uint64("datasource_id", table.DatasourceID),
		zap.Uint64("table_id", table.ID),
		zap.String("schema", table.SchemaName),
		zap.String("table", table.Name),
		zap.Uint64("user_id", input.UserID),
	)
	s.recordMetadataCommentAudit(ctx, input, current.TenantID, current.ProjectID, auditpkg.ResourceMetadataTable, table.ID, map[string]any{
		"schema_name": current.SchemaName,
		"table_name":  current.Name,
		"old_comment": current.CommentText,
		"new_comment": table.CommentText,
	})
	return table, nil
}

func (s *DatasourceService) UpdateMetadataColumnComment(ctx context.Context, input UpdateMetadataCommentInput) (*model.MetadataColumn, error) {
	if input.DatasourceID == 0 || input.ColumnID == 0 {
		return nil, ErrInvalidInput
	}
	current, err := s.datasourceRepo.GetMetadataColumn(ctx, input.DatasourceID, input.ColumnID)
	if err != nil {
		s.logger.Error("metadata column comment load failed",
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("column_id", input.ColumnID),
			zap.Uint64("user_id", input.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	column, err := s.datasourceRepo.UpdateMetadataColumnComment(ctx, input.DatasourceID, input.ColumnID, strings.TrimSpace(input.Comment))
	if err != nil {
		s.logger.Error("metadata column comment update failed",
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("column_id", input.ColumnID),
			zap.Uint64("user_id", input.UserID),
			zap.Error(err),
		)
		return nil, err
	}
	s.logger.Info("metadata column comment updated",
		zap.Uint64("tenant_id", column.TenantID),
		zap.Uint64("project_id", column.ProjectID),
		zap.Uint64("datasource_id", column.DatasourceID),
		zap.Uint64("table_id", column.TableID),
		zap.Uint64("column_id", column.ID),
		zap.String("column", column.ColumnName),
		zap.Uint64("user_id", input.UserID),
	)
	s.recordMetadataCommentAudit(ctx, input, current.TenantID, current.ProjectID, auditpkg.ResourceMetadataColumn, column.ID, map[string]any{
		"table_id":    current.TableID,
		"column_name": current.ColumnName,
		"old_comment": current.CommentText,
		"new_comment": column.CommentText,
	})
	return column, nil
}

func (s *DatasourceService) recordMetadataCommentAudit(ctx context.Context, input UpdateMetadataCommentInput, tenantID uint64, projectID uint64, resourceType string, resourceID uint64, payload map[string]any) {
	if s.auditRecorder == nil {
		return
	}
	payload["datasource_id"] = input.DatasourceID
	_ = s.auditRecorder.Record(ctx, auditpkg.Event{
		TenantID:     tenantID,
		ProjectID:    projectID,
		UserID:       input.UserID,
		EventType:    auditpkg.EventMetadataEdit,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestID:    strings.TrimSpace(input.RequestID),
		IP:           strings.TrimSpace(input.IP),
		UserAgent:    strings.TrimSpace(input.UserAgent),
		Payload:      payload,
	})
}

func metadataToModels(datasource *model.Datasource, metadata *dsdriver.Metadata, syncedAt time.Time) ([]model.MetadataSchema, []model.MetadataTable, int, int, int) {
	if metadata == nil {
		return nil, nil, 0, 0, 0
	}
	schemas := make([]model.MetadataSchema, 0, len(metadata.Schemas))
	for _, schema := range metadata.Schemas {
		schemas = append(schemas, model.MetadataSchema{
			TenantID:     datasource.TenantID,
			ProjectID:    datasource.ProjectID,
			DatasourceID: datasource.ID,
			SchemaName:   schema.Name,
			CommentText:  schema.Comment,
			SyncedAt:     syncedAt,
		})
	}

	columnCount := 0
	indexCount := 0
	foreignKeyCount := 0
	tables := make([]model.MetadataTable, 0, len(metadata.Tables))
	for _, table := range metadata.Tables {
		modelTable := model.MetadataTable{
			TenantID:            datasource.TenantID,
			ProjectID:           datasource.ProjectID,
			DatasourceID:        datasource.ID,
			SchemaName:          table.Schema,
			Name:                table.Name,
			TableType:           table.Type,
			CommentText:         table.Comment,
			OriginalCommentText: table.Comment,
			RowCount:            table.RowCount,
			SyncedAt:            syncedAt,
			Columns:             make([]model.MetadataColumn, 0, len(table.Columns)),
			Indexes:             make([]model.MetadataIndex, 0, len(table.Indexes)),
			ForeignKeys:         make([]model.MetadataForeignKey, 0, len(table.ForeignKeys)),
		}
		for _, column := range table.Columns {
			modelTable.Columns = append(modelTable.Columns, model.MetadataColumn{
				TenantID:            datasource.TenantID,
				ProjectID:           datasource.ProjectID,
				DatasourceID:        datasource.ID,
				ColumnName:          column.Name,
				OrdinalPosition:     column.OrdinalPosition,
				DataType:            column.DataType,
				ColumnType:          column.ColumnType,
				Nullable:            column.Nullable,
				DefaultValue:        column.DefaultValue,
				IsPrimaryKey:        column.IsPrimaryKey,
				IsForeignKey:        column.IsForeignKey,
				CommentText:         column.Comment,
				OriginalCommentText: column.Comment,
				SyncedAt:            syncedAt,
			})
		}
		for _, index := range table.Indexes {
			modelTable.Indexes = append(modelTable.Indexes, model.MetadataIndex{
				TenantID:     datasource.TenantID,
				ProjectID:    datasource.ProjectID,
				DatasourceID: datasource.ID,
				IndexName:    index.Name,
				IndexType:    index.Type,
				UniqueIndex:  index.Unique,
				ColumnsJSON:  metadataIndexColumnsJSON(index.Columns),
				SyncedAt:     syncedAt,
			})
		}
		for _, foreignKey := range table.ForeignKeys {
			modelTable.ForeignKeys = append(modelTable.ForeignKeys, model.MetadataForeignKey{
				TenantID:         datasource.TenantID,
				ProjectID:        datasource.ProjectID,
				DatasourceID:     datasource.ID,
				ConstraintName:   foreignKey.ConstraintName,
				ColumnName:       foreignKey.ColumnName,
				ReferencedSchema: foreignKey.ReferencedSchema,
				ReferencedTable:  foreignKey.ReferencedTable,
				ReferencedColumn: foreignKey.ReferencedColumn,
				SyncedAt:         syncedAt,
			})
		}
		columnCount += len(modelTable.Columns)
		indexCount += len(modelTable.Indexes)
		foreignKeyCount += len(modelTable.ForeignKeys)
		tables = append(tables, modelTable)
	}
	return schemas, tables, columnCount, indexCount, foreignKeyCount
}

func metadataIndexColumnsJSON(columns []string) string {
	data, err := json.Marshal(columns)
	if err != nil || len(data) == 0 {
		return "[]"
	}
	return string(data)
}

func detectDatasourceVersion(ctx context.Context, driver dsdriver.Driver, dsn string) string {
	versioner, ok := driver.(dsdriver.Versioner)
	if !ok {
		return ""
	}
	version, err := versioner.Version(ctx, dsdriver.Config{DSN: dsn})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(version)
}

func configStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func mergeDatasourceConfigVersion(configJSON *string, version string) (*string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return configJSON, false
	}
	config := map[string]any{}
	if configJSON != nil && strings.TrimSpace(*configJSON) != "" {
		_ = json.Unmarshal([]byte(*configJSON), &config)
		if config == nil {
			config = map[string]any{}
		}
	}
	if strings.TrimSpace(toString(config["version"])) == version {
		return configJSON, false
	}
	config["version"] = version
	data, err := json.Marshal(config)
	if err != nil {
		return configJSON, false
	}
	value := string(data)
	return &value, true
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		if typed == nil {
			return ""
		}
		data, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return strings.Trim(string(data), `"`)
	}
}
