package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/cache"
	dsdriver "ling-shu/internal/datasource"
	"ling-shu/internal/model"
	querypkg "ling-shu/internal/query"
	"ling-shu/internal/repository"
	"ling-shu/pkg/secret"

	"go.uber.org/zap"
)

const defaultQueryTimeout = 30 * time.Second

type QueryService struct {
	datasourceRepo  repository.DatasourceRepository
	queryRepo       repository.QueryRepository
	securityRepo    repository.SecurityRepository
	registry        *dsdriver.Registry
	reviewer        *querypkg.SQLReviewer
	auditRecorder   auditpkg.Recorder
	timeout         time.Duration
	dsnCodec        secret.Codec
	queryLockStore  cache.Store
	queryLockPrefix string
	queryLockTTL    time.Duration
	logger          *zap.Logger
}

type ReviewSQLInput struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	UserID       uint64
	SQL          string
	MaxRows      int
	RequestID    string
	IP           string
	UserAgent    string
}

type ExecuteSQLInput struct {
	TenantID     uint64
	ProjectID    uint64
	DatasourceID uint64
	SessionID    uint64
	UserID       uint64
	Question     string
	SQL          string
	MaxRows      int
	RequestID    string
	IP           string
	UserAgent    string
}

type QueryHistoryInput struct {
	TenantID     uint64
	ProjectID    uint64
	UserID       uint64
	DatasourceID uint64
	Status       string
	StartTime    time.Time
	EndTime      time.Time
	Page         int
	PageSize     int
}

type QueryExecutionResult struct {
	Execution     *model.QueryExecution    `json:"execution"`
	Review        querypkg.ReviewResult    `json:"review"`
	Chart         querypkg.ChartSuggestion `json:"chart"`
	Answer        string                   `json:"answer,omitempty"`
	SpeechSummary string                   `json:"speech_summary,omitempty"`
	Error         string                   `json:"error,omitempty"`
	Columns       []string                 `json:"columns,omitempty"`
	Rows          []map[string]any         `json:"rows,omitempty"`
}

func NewQueryService(datasourceRepo repository.DatasourceRepository, queryRepo repository.QueryRepository, registry *dsdriver.Registry, reviewer *querypkg.SQLReviewer, auditRecorders ...auditpkg.Recorder) *QueryService {
	if registry == nil {
		registry = dsdriver.DefaultRegistry()
	}
	if reviewer == nil {
		reviewer = querypkg.NewSQLReviewer(200, 1000)
	}
	var auditRecorder auditpkg.Recorder
	if len(auditRecorders) > 0 {
		auditRecorder = auditRecorders[0]
	}
	return &QueryService{
		datasourceRepo:  datasourceRepo,
		queryRepo:       queryRepo,
		registry:        registry,
		reviewer:        reviewer,
		auditRecorder:   auditRecorder,
		timeout:         defaultQueryTimeout,
		dsnCodec:        secret.PlainCodec{},
		queryLockPrefix: "ling-shu:query:lock",
		queryLockTTL:    30 * time.Second,
		logger:          zap.NewNop(),
	}
}

func (s *QueryService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	s.logger = logger
}

func (s *QueryService) SetDSNCodec(codec secret.Codec) {
	if codec == nil {
		codec = secret.PlainCodec{}
	}
	s.dsnCodec = codec
}

func (s *QueryService) SetSecurityRepository(securityRepo repository.SecurityRepository) {
	s.securityRepo = securityRepo
}

func (s *QueryService) SetQueryLock(store cache.Store, prefix string, ttl time.Duration) {
	s.queryLockStore = store
	if strings.TrimSpace(prefix) != "" {
		s.queryLockPrefix = prefix
	}
	if ttl > 0 {
		s.queryLockTTL = ttl
	}
}

func (s *QueryService) ReviewSQL(ctx context.Context, input ReviewSQLInput) (*querypkg.ReviewResult, error) {
	sqlText := strings.TrimSpace(input.SQL)
	if input.TenantID == 0 || input.ProjectID == 0 || sqlText == "" {
		return nil, ErrInvalidInput
	}
	review, err := s.reviewSQL(ctx, input.TenantID, input.ProjectID, input.DatasourceID, sqlText, input.MaxRows)
	if err != nil {
		s.logger.Warn("sql review failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.String("sql_hash", sqlHash(sqlText)),
			zap.Int("max_rows", input.MaxRows),
			zap.Error(err),
		)
		return nil, err
	}
	if err := s.saveReview(ctx, input.TenantID, input.ProjectID, input.DatasourceID, 0, sqlText, review); err != nil {
		s.logger.Error("sql review save failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.String("sql_hash", sqlHash(sqlText)),
			zap.Bool("review_passed", review.Passed),
			zap.Error(err),
		)
		return nil, err
	}
	s.recordAudit(ctx, auditpkg.Event{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		UserID:       input.UserID,
		EventType:    auditpkg.EventSQLReview,
		ResourceType: auditpkg.ResourceSQLReview,
		RequestID:    input.RequestID,
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Payload: map[string]any{
			"datasource_id":  input.DatasourceID,
			"passed":         review.Passed,
			"risk_level":     review.RiskLevel,
			"blocked_reason": review.BlockedReason,
			"limit":          review.Limit,
		},
	})
	return &review, nil
}

func (s *QueryService) ExecuteSQL(ctx context.Context, input ExecuteSQLInput) (*QueryExecutionResult, error) {
	operationStartedAt := time.Now()
	sqlText := strings.TrimSpace(input.SQL)
	question := strings.TrimSpace(input.Question)
	if input.TenantID == 0 || input.ProjectID == 0 || input.DatasourceID == 0 || input.UserID == 0 || sqlText == "" {
		return nil, ErrInvalidInput
	}
	if question == "" {
		question = "SQL query"
	}

	execution := &model.QueryExecution{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		DatasourceID: input.DatasourceID,
		SessionID:    input.SessionID,
		UserID:       input.UserID,
		Question:     question,
		GeneratedSQL: sqlText,
		Status:       "running",
	}
	if err := s.queryRepo.CreateExecution(ctx, execution); err != nil {
		s.logger.Error("query execution create failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("session_id", input.SessionID),
			zap.Uint64("user_id", input.UserID),
			zap.String("sql_hash", sqlHash(sqlText)),
			zap.String("question_hash", sqlHash(question)),
			zap.Error(err),
		)
		return nil, err
	}

	review, err := s.reviewSQL(ctx, input.TenantID, input.ProjectID, input.DatasourceID, sqlText, input.MaxRows)
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, sqlText, err, input)
		return failedQueryExecutionResult(execution, querypkg.ReviewResult{NormalizedSQL: sqlText}, err), err
	}
	if err := s.saveReview(ctx, input.TenantID, input.ProjectID, input.DatasourceID, execution.ID, sqlText, review); err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Error("query execution review save failed",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("sql_hash", sqlHash(review.NormalizedSQL)),
			zap.Bool("review_passed", review.Passed),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}
	if !review.Passed {
		now := time.Now()
		if err := s.queryRepo.FinishExecution(ctx, execution.ID, repository.QueryExecutionFinish{
			Status:       "blocked",
			FinalSQL:     review.NormalizedSQL,
			SQLHash:      sqlHash(review.NormalizedSQL),
			ErrorMessage: review.BlockedReason,
			FinishedAt:   now,
		}); err != nil {
			s.logger.Error("query execution finish blocked state failed",
				zap.Uint64("tenant_id", execution.TenantID),
				zap.Uint64("project_id", execution.ProjectID),
				zap.Uint64("datasource_id", execution.DatasourceID),
				zap.Uint64("session_id", execution.SessionID),
				zap.Uint64("execution_id", execution.ID),
				zap.String("sql_hash", sqlHash(review.NormalizedSQL)),
				zap.Error(err),
			)
			return nil, err
		}
		execution.Status = "blocked"
		execution.FinalSQL = review.NormalizedSQL
		execution.SQLHash = sqlHash(review.NormalizedSQL)
		execution.ErrorMessage = review.BlockedReason
		execution.FinishedAt = &now
		s.recordQueryExecutionAudit(ctx, execution, review, input)
		s.logger.Info("query execution blocked",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("sql_hash", execution.SQLHash),
			zap.String("risk_level", review.RiskLevel),
			zap.String("blocked_reason", review.BlockedReason),
			zap.Duration("duration", time.Since(operationStartedAt)),
		)
		return &QueryExecutionResult{Execution: execution, Review: review}, nil
	}

	queryLock, err := s.acquireQueryLock(ctx, input, review.NormalizedSQL)
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Warn("query execution rejected by duplicate lock",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("sql_hash", sqlHash(review.NormalizedSQL)),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}
	defer s.releaseQueryLock(queryLock, execution, review.NormalizedSQL)

	datasource, err := s.datasourceRepo.GetByID(ctx, input.DatasourceID)
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Warn("query execution datasource load failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("execution_id", execution.ID),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}
	if datasource.TenantID != input.TenantID {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, ErrInvalidInput, input)
		s.logger.Warn("query execution datasource tenant scope rejected",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("actual_tenant_id", datasource.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("execution_id", execution.ID),
		)
		return failedQueryExecutionResult(execution, review, ErrInvalidInput), ErrInvalidInput
	}
	if datasource.ProjectID != input.ProjectID {
		bound, bindErr := s.datasourceRepo.IsBoundToProject(ctx, input.TenantID, input.ProjectID, input.DatasourceID)
		if bindErr != nil {
			_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, bindErr, input)
			s.logger.Error("query execution datasource binding check failed",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Uint64("datasource_id", input.DatasourceID),
				zap.Uint64("execution_id", execution.ID),
				zap.Error(bindErr),
			)
			return failedQueryExecutionResult(execution, review, bindErr), bindErr
		}
		if !bound {
			_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, ErrInvalidInput, input)
			s.logger.Warn("query execution datasource project scope rejected",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Uint64("datasource_id", input.DatasourceID),
				zap.Uint64("datasource_project_id", datasource.ProjectID),
				zap.Uint64("execution_id", execution.ID),
			)
			return failedQueryExecutionResult(execution, review, ErrInvalidInput), ErrInvalidInput
		}
	}
	driver, err := s.registry.Driver(datasource.DBType)
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Error("query execution driver resolve failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("db_type", datasource.DBType),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}
	dsn, err := decryptSecret(s.dsnCodec, datasource.DSNCiphertext)
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Error("query execution dsn decrypt failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("db_type", datasource.DBType),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	startedAt := time.Now()
	result, err := driver.Query(queryCtx, dsdriver.Config{DSN: dsn}, review.NormalizedSQL, review.Limit)
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Warn("query execution driver query failed",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("db_type", datasource.DBType),
			zap.String("sql_hash", sqlHash(review.NormalizedSQL)),
			zap.Int("duration_ms", durationMS),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}
	if result == nil {
		result = &dsdriver.QueryResult{}
	}

	rowCount := len(result.Rows)
	chart := querypkg.SuggestChart(result.Columns, result.Rows)
	answer := summarizeQueryAnswer(question, result.Columns, result.Rows, chart)
	speechSummary := summarizeQuerySpeech(result.Columns, result.Rows)
	previewJSON, err := marshalResultPreview(result)
	if err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Error("query execution result preview marshal failed",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.Int("row_count", rowCount),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}
	now := time.Now()
	if err := s.queryRepo.FinishExecution(ctx, execution.ID, repository.QueryExecutionFinish{
		Status:            "success",
		FinalSQL:          review.NormalizedSQL,
		SQLHash:           sqlHash(review.NormalizedSQL),
		RowCount:          &rowCount,
		DurationMS:        &durationMS,
		ChartType:         chart.Type,
		ResultPreviewJSON: &previewJSON,
		FinishedAt:        now,
	}); err != nil {
		_ = s.finishExecutionError(ctx, execution, review.NormalizedSQL, err, input)
		s.logger.Error("query execution finish success state failed",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("sql_hash", sqlHash(review.NormalizedSQL)),
			zap.Int("row_count", rowCount),
			zap.Error(err),
		)
		return failedQueryExecutionResult(execution, review, err), err
	}

	execution.Status = "success"
	execution.FinalSQL = review.NormalizedSQL
	execution.SQLHash = sqlHash(review.NormalizedSQL)
	execution.RowCount = &rowCount
	execution.DurationMS = &durationMS
	execution.ChartType = chart.Type
	execution.ResultPreviewJSON = &previewJSON
	execution.FinishedAt = &now
	s.recordQueryExecutionAudit(ctx, execution, review, input)
	s.logger.Info("query execution succeeded",
		zap.Uint64("tenant_id", execution.TenantID),
		zap.Uint64("project_id", execution.ProjectID),
		zap.Uint64("datasource_id", execution.DatasourceID),
		zap.Uint64("session_id", execution.SessionID),
		zap.Uint64("execution_id", execution.ID),
		zap.String("sql_hash", execution.SQLHash),
		zap.Int("row_count", rowCount),
		zap.Int("duration_ms", durationMS),
		zap.String("chart_type", chart.Type),
		zap.Duration("total_duration", time.Since(operationStartedAt)),
	)
	return &QueryExecutionResult{
		Execution:     execution,
		Review:        review,
		Chart:         chart,
		Answer:        answer,
		SpeechSummary: speechSummary,
		Columns:       result.Columns,
		Rows:          result.Rows,
	}, nil
}

func (s *QueryService) acquireQueryLock(ctx context.Context, input ExecuteSQLInput, normalizedSQL string) (*cache.Lock, error) {
	if s.queryLockStore == nil {
		return nil, nil
	}
	rawKey := fmt.Sprintf("tenant=%d:project=%d:datasource=%d:max_rows=%d:sql=%s",
		input.TenantID,
		input.ProjectID,
		input.DatasourceID,
		input.MaxRows,
		normalizedSQL,
	)
	lock, ok, err := cache.TryLock(ctx, s.queryLockStore, s.queryLockPrefix, rawKey, s.queryLockTTL)
	if err != nil {
		s.logger.Warn("query execution lock check failed; continuing without lock",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.String("sql_hash", sqlHash(normalizedSQL)),
			zap.Error(err),
		)
		return nil, nil
	}
	if !ok {
		return nil, ErrQueryAlreadyRunning
	}
	return lock, nil
}

func (s *QueryService) releaseQueryLock(lock *cache.Lock, execution *model.QueryExecution, normalizedSQL string) {
	if lock == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lock.Release(ctx); err != nil {
		s.logger.Warn("query execution lock release failed",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("sql_hash", sqlHash(normalizedSQL)),
			zap.Error(err),
		)
	}
}

func failedQueryExecutionResult(execution *model.QueryExecution, review querypkg.ReviewResult, err error) *QueryExecutionResult {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	return &QueryExecutionResult{
		Execution:     execution,
		Review:        review,
		Answer:        "SQL 已生成，但自动执行失败：" + errorMessage,
		SpeechSummary: "自动执行失败：" + errorMessage,
		Error:         errorMessage,
	}
}

func (s *QueryService) reviewSQL(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64, sqlText string, maxRows int) (querypkg.ReviewResult, error) {
	review := s.reviewer.ReviewWithDialect(sqlText, maxRows, "")
	if !review.Passed {
		return review, nil
	}

	dialect := ""
	if datasourceID > 0 {
		datasource, err := s.datasourceRepo.GetByID(ctx, datasourceID)
		if err != nil {
			return review, err
		}
		dialect = datasource.DBType
	}
	if dialect != "" {
		review = s.reviewer.ReviewWithDialect(sqlText, maxRows, dialect)
	}
	if !review.Passed || s.securityRepo == nil {
		return review, nil
	}
	tables, err := s.securityRepo.ListSensitiveTables(ctx, repository.SensitiveRuleFilter{
		TenantID:     tenantID,
		ProjectID:    projectID,
		DatasourceID: datasourceID,
	})
	if err != nil {
		return review, err
	}
	columns, err := s.securityRepo.ListSensitiveColumns(ctx, repository.SensitiveRuleFilter{
		TenantID:     tenantID,
		ProjectID:    projectID,
		DatasourceID: datasourceID,
	})
	if err != nil {
		return review, err
	}
	return applySensitiveRules(review, tables, columns), nil
}

func (s *QueryService) History(ctx context.Context, input QueryHistoryInput) (PageResult[model.QueryExecution], error) {
	if input.TenantID == 0 || input.ProjectID == 0 {
		return PageResult[model.QueryExecution]{}, ErrInvalidInput
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.queryRepo.ListExecutions(ctx, repository.QueryExecutionFilter{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		UserID:       input.UserID,
		DatasourceID: input.DatasourceID,
		Status:       strings.TrimSpace(input.Status),
		StartTime:    input.StartTime,
		EndTime:      input.EndTime,
	}, p)
	if err != nil {
		s.logger.Error("query history list failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", input.DatasourceID),
			zap.Uint64("user_id", input.UserID),
			zap.String("status", strings.TrimSpace(input.Status)),
			zap.Int("page", p.Page),
			zap.Int("page_size", p.Limit()),
			zap.Error(err),
		)
		return PageResult[model.QueryExecution]{}, err
	}
	return PageResult[model.QueryExecution]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func (s *QueryService) saveReview(ctx context.Context, tenantID uint64, projectID uint64, datasourceID uint64, executionID uint64, sqlText string, review querypkg.ReviewResult) error {
	rules := map[string]any{
		"warnings": review.Warnings,
		"limit":    review.Limit,
	}
	rulesJSONBytes, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	rulesJSON := string(rulesJSONBytes)
	return s.queryRepo.CreateReviewResult(ctx, &model.SQLReviewResult{
		TenantID:         tenantID,
		ProjectID:        projectID,
		QueryExecutionID: executionID,
		DatasourceID:     datasourceID,
		SQLText:          sqlText,
		Passed:           review.Passed,
		RiskLevel:        review.RiskLevel,
		BlockedReason:    review.BlockedReason,
		RulesJSON:        &rulesJSON,
	})
}

func (s *QueryService) finishExecutionError(ctx context.Context, execution *model.QueryExecution, finalSQL string, err error, input ExecuteSQLInput) error {
	now := time.Now()
	finishErr := s.queryRepo.FinishExecution(ctx, execution.ID, repository.QueryExecutionFinish{
		Status:       "failed",
		FinalSQL:     finalSQL,
		SQLHash:      sqlHash(finalSQL),
		ErrorMessage: err.Error(),
		FinishedAt:   now,
	})
	if finishErr != nil {
		s.logger.Error("query execution finish failed state failed",
			zap.Uint64("tenant_id", execution.TenantID),
			zap.Uint64("project_id", execution.ProjectID),
			zap.Uint64("datasource_id", execution.DatasourceID),
			zap.Uint64("session_id", execution.SessionID),
			zap.Uint64("execution_id", execution.ID),
			zap.String("sql_hash", sqlHash(finalSQL)),
			zap.Error(finishErr),
		)
		return finishErr
	}
	execution.Status = "failed"
	execution.FinalSQL = finalSQL
	execution.SQLHash = sqlHash(finalSQL)
	execution.ErrorMessage = err.Error()
	execution.FinishedAt = &now
	s.recordQueryExecutionAudit(ctx, execution, querypkg.ReviewResult{NormalizedSQL: finalSQL}, input)
	s.logger.Warn("query execution failed",
		zap.Uint64("tenant_id", execution.TenantID),
		zap.Uint64("project_id", execution.ProjectID),
		zap.Uint64("datasource_id", execution.DatasourceID),
		zap.Uint64("session_id", execution.SessionID),
		zap.Uint64("execution_id", execution.ID),
		zap.String("sql_hash", execution.SQLHash),
		zap.Error(err),
	)
	return nil
}

func (s *QueryService) recordQueryExecutionAudit(ctx context.Context, execution *model.QueryExecution, review querypkg.ReviewResult, input ExecuteSQLInput) {
	payload := map[string]any{
		"datasource_id": execution.DatasourceID,
		"session_id":    execution.SessionID,
		"status":        execution.Status,
		"sql_hash":      execution.SQLHash,
		"review_passed": review.Passed,
	}
	if execution.RowCount != nil {
		payload["row_count"] = *execution.RowCount
	}
	if execution.DurationMS != nil {
		payload["duration_ms"] = *execution.DurationMS
	}
	if execution.ChartType != "" {
		payload["chart_type"] = execution.ChartType
	}
	if execution.ErrorMessage != "" {
		payload["error_message"] = execution.ErrorMessage
	}
	s.recordAudit(ctx, auditpkg.Event{
		TenantID:     execution.TenantID,
		ProjectID:    execution.ProjectID,
		UserID:       execution.UserID,
		EventType:    auditpkg.EventQueryExecute,
		ResourceType: auditpkg.ResourceQueryExecution,
		ResourceID:   execution.ID,
		RequestID:    input.RequestID,
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Payload:      payload,
	})
}

func (s *QueryService) recordAudit(ctx context.Context, event auditpkg.Event) {
	if s.auditRecorder == nil {
		return
	}
	_ = s.auditRecorder.Record(ctx, event)
}

func marshalResultPreview(result *dsdriver.QueryResult) (string, error) {
	if result == nil {
		return "{}", nil
	}
	rows := result.Rows
	if len(rows) > 100 {
		rows = rows[:100]
	}
	payload := map[string]any{
		"columns": result.Columns,
		"rows":    rows,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func sqlHash(sqlText string) string {
	sum := sha256.Sum256([]byte(sqlText))
	return hex.EncodeToString(sum[:])
}

func applySensitiveRules(review querypkg.ReviewResult, tables []model.SensitiveTable, columns []model.SensitiveColumn) querypkg.ReviewResult {
	sqlText := strings.ToLower(review.NormalizedSQL)
	referencedTables := referencedTableSet(sqlText)
	for _, rule := range tables {
		if sensitiveTableMatched(rule, referencedTables, sqlText) {
			review.Passed = false
			review.RiskLevel = firstNonEmptyString(rule.RiskLevel, "high")
			review.BlockedReason = fmt.Sprintf("SQL 命中敏感表规则: %s", qualifiedName(rule.SchemaName, rule.Table))
			return review
		}
	}
	for _, rule := range columns {
		if sensitiveColumnMatched(rule, referencedTables, sqlText) {
			review.Passed = false
			review.RiskLevel = firstNonEmptyString(rule.RiskLevel, "medium")
			review.BlockedReason = fmt.Sprintf("SQL 命中敏感字段规则: %s.%s", qualifiedName(rule.SchemaName, rule.Table), rule.ColumnName)
			return review
		}
	}
	return review
}

var tableReferencePattern = regexp.MustCompile(`(?i)\b(?:from|join)\s+([` + "`" + `"'\w.]+)`)

func referencedTableSet(sqlText string) map[string]bool {
	out := map[string]bool{}
	matches := tableReferencePattern.FindAllStringSubmatch(sqlText, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := normalizeSQLIdentifier(match[1])
		if name == "" {
			continue
		}
		out[name] = true
		parts := strings.Split(name, ".")
		if len(parts) > 0 {
			out[parts[len(parts)-1]] = true
		}
	}
	return out
}

func sensitiveTableMatched(rule model.SensitiveTable, referencedTables map[string]bool, sqlText string) bool {
	table := normalizeSQLIdentifier(rule.Table)
	schema := normalizeSQLIdentifier(rule.SchemaName)
	if table == "" {
		return false
	}
	if schema != "" && referencedTables[schema+"."+table] {
		return true
	}
	if referencedTables[table] {
		return true
	}
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(table) + `\b`).MatchString(sqlText)
}

func sensitiveColumnMatched(rule model.SensitiveColumn, referencedTables map[string]bool, sqlText string) bool {
	column := normalizeSQLIdentifier(rule.ColumnName)
	table := normalizeSQLIdentifier(rule.Table)
	schema := normalizeSQLIdentifier(rule.SchemaName)
	if column == "" || table == "" {
		return false
	}
	tableMatched := referencedTables[table]
	if schema != "" {
		tableMatched = tableMatched || referencedTables[schema+"."+table]
	}
	if !tableMatched {
		return false
	}
	return regexp.MustCompile(`(?i)(?:\b|[.` + "`" + `"])` + regexp.QuoteMeta(column) + `(?:\b|[` + "`" + `"])`).MatchString(sqlText)
}

func normalizeSQLIdentifier(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, "`\"'")
	return value
}

func qualifiedName(schema string, table string) string {
	schema = strings.TrimSpace(schema)
	table = strings.TrimSpace(table)
	if schema == "" {
		return table
	}
	return schema + "." + table
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func summarizeQueryAnswer(question string, columns []string, rows []map[string]any, chart querypkg.ChartSuggestion) string {
	if len(rows) == 0 {
		return "查询已完成，未返回数据。"
	}
	if len(rows) == 1 && len(columns) == 1 {
		return fmt.Sprintf("%s是 %s。", metricDisplayName(columns[0]), formatAny(rows[0][columns[0]]))
	}
	if len(rows) == 1 && len(columns) <= 4 {
		return fmt.Sprintf("查询已完成，返回 1 行结果：%s。", compactRowSummary(columns, rows[0]))
	}
	if chart.Type != "" && chart.Type != querypkg.ChartTable {
		return fmt.Sprintf("查询已完成，返回 %d 行数据，已按%s展示。", len(rows), chartTypeName(chart.Type))
	}
	return fmt.Sprintf("查询已完成，返回 %d 行数据。", len(rows))
}

func summarizeQuerySpeech(columns []string, rows []map[string]any) string {
	if len(rows) == 0 {
		return "没有查询到符合条件的数据。"
	}
	if len(rows) == 1 && len(columns) == 1 {
		return fmt.Sprintf("%s是 %s。", metricDisplayName(columns[0]), formatAny(rows[0][columns[0]]))
	}
	if len(rows) == 1 && len(columns) <= 4 {
		return "查询结果：" + compactRowSummary(columns, rows[0]) + "。"
	}
	if len(rows) <= 4 && len(columns) <= 4 {
		return fmt.Sprintf("共返回 %d 行数据，第一行是：%s。", len(rows), compactRowSummary(columns, rows[0]))
	}
	return fmt.Sprintf("共返回 %d 行数据。", len(rows))
}

func compactRowSummary(columns []string, row map[string]any) string {
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, fmt.Sprintf("%s=%s", metricDisplayName(column), formatAny(row[column])))
	}
	return strings.Join(parts, "，")
}

func chartTypeName(chartType string) string {
	switch chartType {
	case querypkg.ChartLine:
		return "折线图"
	case querypkg.ChartBar:
		return "柱状图"
	case querypkg.ChartPie:
		return "饼图"
	case querypkg.ChartFunnel:
		return "漏斗图"
	case querypkg.ChartRadar:
		return "雷达图"
	default:
		return "表格"
	}
}
