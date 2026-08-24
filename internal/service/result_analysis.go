package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/pyexecclient"
	"ling-shu/internal/query"

	"go.uber.org/zap"
)

type ResultAnalysisClient interface {
	Analyze(ctx context.Context, input pyexecclient.AnalyzeRequest) (*pyexecclient.AnalyzeResponse, error)
	Health(ctx context.Context) (*pyexecclient.HealthStatus, error)
}

type ResultAnalysisConfig struct {
	Enabled        bool
	Timeout        time.Duration
	MaxInputRows   int
	MaxOutputRows  int
	MaxStdoutChars int
	FailOpen       bool
}

type AnalyzeQueryResultsInput struct {
	TenantID       uint64
	ProjectID      uint64
	SessionID      uint64
	UserID         uint64
	Question       string
	RequestID      string
	Mode           string
	AnalysisGoal   string
	TemplateName   string
	TemplateParams map[string]any
	Tasks          []query.AgentSQLTask
	Executions     []*QueryExecutionResult
}

type ResultAnalysisResult = pyexecclient.AnalyzeResponse
type ResultAnalysisTable = pyexecclient.Table
type ResultAnalysisChart = pyexecclient.Chart

type ResultAnalysisService struct {
	client         ResultAnalysisClient
	enabled        bool
	failOpen       bool
	timeout        time.Duration
	maxInputRows   int
	maxOutputRows  int
	maxStdoutChars int
	auditRecorder  auditpkg.Recorder
	logger         *zap.Logger
}

func NewResultAnalysisService(client ResultAnalysisClient, cfg ResultAnalysisConfig) *ResultAnalysisService {
	return &ResultAnalysisService{
		client:         client,
		enabled:        cfg.Enabled,
		failOpen:       cfg.FailOpen,
		timeout:        cfg.Timeout,
		maxInputRows:   defaultPositive(cfg.MaxInputRows, 5000),
		maxOutputRows:  defaultPositive(cfg.MaxOutputRows, 1000),
		maxStdoutChars: defaultPositive(cfg.MaxStdoutChars, 10000),
		logger:         zap.NewNop(),
	}
}

func (s *ResultAnalysisService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	s.logger = logger
}

func (s *ResultAnalysisService) SetAuditRecorder(recorder auditpkg.Recorder) {
	s.auditRecorder = recorder
}

func (s *ResultAnalysisService) AnalyzeQueryResults(ctx context.Context, input AnalyzeQueryResultsInput) (*ResultAnalysisResult, error) {
	if s == nil || !s.enabled || s.client == nil || len(input.Executions) == 0 {
		return nil, nil
	}
	started := time.Now()
	datasets := analysisDatasets(input.Tasks, input.Executions, s.maxInputRows)
	if len(datasets) == 0 {
		return nil, nil
	}
	inputRows := datasetRowCount(datasets)
	req := pyexecclient.AnalyzeRequest{
		RequestID:      input.RequestID,
		TenantID:       input.TenantID,
		ProjectID:      input.ProjectID,
		SessionID:      input.SessionID,
		UserID:         input.UserID,
		Question:       input.Question,
		Mode:           firstNonEmptyString(strings.TrimSpace(input.Mode), "auto"),
		AnalysisGoal:   strings.TrimSpace(input.AnalysisGoal),
		TemplateName:   strings.TrimSpace(input.TemplateName),
		TemplateParams: copyTemplateParams(input.TemplateParams),
		Datasets:       datasets,
		Limits: pyexecclient.Limits{
			TimeoutMS:      int(s.timeout.Milliseconds()),
			MaxInputRows:   s.maxInputRows,
			MaxOutputRows:  s.maxOutputRows,
			MaxStdoutChars: s.maxStdoutChars,
		},
	}
	s.logger.Info("python result analysis started",
		zap.String("request_id", input.RequestID),
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("session_id", input.SessionID),
		zap.Uint64("user_id", input.UserID),
		zap.Int("dataset_count", len(req.Datasets)),
		zap.Int("input_rows", inputRows),
		zap.String("mode", req.Mode),
		zap.String("template_name", req.TemplateName),
	)
	resp, err := s.client.Analyze(ctx, req)
	if err != nil {
		s.logger.Warn("python result analysis failed",
			zap.String("request_id", input.RequestID),
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("session_id", input.SessionID),
			zap.Uint64("user_id", input.UserID),
			zap.Int("dataset_count", len(req.Datasets)),
			zap.Int("input_rows", inputRows),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		s.recordAudit(ctx, input, nil, err)
		if s.failOpen {
			return nil, nil
		}
		return nil, err
	}
	s.logger.Info("python result analysis finished",
		zap.String("request_id", input.RequestID),
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("session_id", input.SessionID),
		zap.Uint64("user_id", input.UserID),
		zap.Int("dataset_count", len(req.Datasets)),
		zap.Bool("success", resp.Success),
		zap.String("analysis_kind", resp.AnalysisKind),
		zap.String("template_name", resp.TemplateName),
		zap.Int64("duration_ms", resp.DurationMS),
		zap.Int("input_rows", resp.InputRowCount),
		zap.Int("output_rows", resp.OutputRowCount),
		zap.Duration("total_duration", time.Since(started)),
	)
	s.recordAudit(ctx, input, resp, nil)
	return resp, nil
}

func (s *ResultAnalysisService) Health(ctx context.Context) (*pyexecclient.HealthStatus, error) {
	if s == nil || !s.enabled || s.client == nil {
		return nil, nil
	}
	return s.client.Health(ctx)
}

func (s *ResultAnalysisService) CheckHealth(ctx context.Context) error {
	status, err := s.Health(ctx)
	if err != nil {
		return err
	}
	if status != nil && !status.OK {
		return fmt.Errorf("exec health check failed version=%s capabilities=%v", status.Version, status.Capabilities)
	}
	return nil
}

func (s *ResultAnalysisService) recordAudit(ctx context.Context, input AnalyzeQueryResultsInput, result *ResultAnalysisResult, err error) {
	if s.auditRecorder == nil {
		return
	}
	payload := map[string]any{
		"session_id":      input.SessionID,
		"dataset_count":   len(input.Executions),
		"input_row_count": executionRowsForAnalysis(input.Executions, s.maxInputRows),
		"mode":            firstNonEmptyString(strings.TrimSpace(input.Mode), "auto"),
		"template_name":   strings.TrimSpace(input.TemplateName),
		"fail_open":       s.failOpen,
	}
	if goal := strings.TrimSpace(input.AnalysisGoal); goal != "" {
		payload["analysis_goal_chars"] = len([]rune(goal))
		payload["analysis_goal_hash"] = sqlHash(goal)
	}
	var resourceID uint64
	if len(input.Executions) == 1 && input.Executions[0] != nil && input.Executions[0].Execution != nil {
		resourceID = input.Executions[0].Execution.ID
	}
	if result != nil {
		payload["success"] = result.Success
		payload["analysis_kind"] = result.AnalysisKind
		payload["template_name"] = result.TemplateName
		payload["code_hash"] = result.CodeHash
		payload["duration_ms"] = result.DurationMS
		payload["input_row_count"] = result.InputRowCount
		payload["output_row_count"] = result.OutputRowCount
		if result.Error != "" {
			payload["error"] = result.Error
		}
	}
	if err != nil {
		payload["success"] = false
		payload["error"] = err.Error()
	}
	_ = s.auditRecorder.Record(ctx, auditpkg.Event{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		UserID:       input.UserID,
		EventType:    auditpkg.EventPythonAnalyze,
		ResourceType: auditpkg.ResourcePythonAnalysis,
		ResourceID:   resourceID,
		RequestID:    input.RequestID,
		Payload:      payload,
	})
}

func copyTemplateParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]any, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}

func executionRowsForAnalysis(executions []*QueryExecutionResult, maxRows int) int {
	total := 0
	for _, execution := range executions {
		if execution == nil {
			continue
		}
		rows := len(execution.Rows)
		if maxRows > 0 && rows > maxRows {
			rows = maxRows
		}
		total += rows
	}
	return total
}

func analysisDatasets(tasks []query.AgentSQLTask, executions []*QueryExecutionResult, maxRows int) []pyexecclient.Dataset {
	out := make([]pyexecclient.Dataset, 0, len(executions))
	for index, execution := range executions {
		if execution == nil || len(execution.Rows) == 0 {
			continue
		}
		var task query.AgentSQLTask
		if index < len(tasks) {
			task = tasks[index]
		}
		executionID := uint64(0)
		if execution.Execution != nil {
			executionID = execution.Execution.ID
			if task.DatasourceID == 0 {
				task.DatasourceID = execution.Execution.DatasourceID
			}
		}
		rows := copyRowsForAnalysis(execution.Rows, maxRows)
		name := firstNonEmptyString(task.DatasourceName, task.Purpose, "dataset")
		out = append(out, pyexecclient.Dataset{
			Name:           name,
			DatasourceID:   task.DatasourceID,
			DatasourceName: task.DatasourceName,
			Purpose:        task.Purpose,
			ExecutionID:    executionID,
			Columns:        append([]string(nil), execution.Columns...),
			Rows:           rows,
		})
	}
	return out
}

func copyRowsForAnalysis(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for _, row := range rows[:limit] {
		copied := make(map[string]any, len(row))
		for key, value := range row {
			copied[key] = value
		}
		out = append(out, copied)
	}
	return out
}

func datasetRowCount(datasets []pyexecclient.Dataset) int {
	total := 0
	for _, dataset := range datasets {
		total += len(dataset.Rows)
	}
	return total
}

func defaultPositive(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
