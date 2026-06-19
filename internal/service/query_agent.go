package service

import (
	"context"
	"strings"
	"time"

	"ling-shu/internal/query"

	"go.uber.org/zap"
)

type QueryAgentService struct {
	agent               *query.ReactAgent
	agentContextBuilder AgentContextBuilder
	logger              *zap.Logger
}

type AskInput struct {
	TenantID              uint64
	ProjectID             uint64
	DatasourceID          uint64
	SelectedDatasourceIDs []uint64
	UserID                uint64
	Question              string
	MaxRows               int
	Attempt               int
	PreviousSQL           string
	PreviousError         string
	Datasources           []query.AgentDatasource
	BusinessTerms         []query.AgentKnowledge
	Metrics               []query.AgentKnowledge
	FewShots              []query.AgentFewShot
	Conversation          []query.AgentMessage
	Permission            query.AgentPermission
}

func NewQueryAgentService(agent *query.ReactAgent) *QueryAgentService {
	return &QueryAgentService{agent: agent, logger: zap.NewNop()}
}

func (s *QueryAgentService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *QueryAgentService) SetAgentContextBuilder(builder AgentContextBuilder) {
	s.agentContextBuilder = builder
}

func (s *QueryAgentService) Ask(ctx context.Context, input AskInput) (*query.AgentResult, error) {
	started := time.Now()
	s.logger.Info("query agent ask started", queryAgentLogFields(input)...)
	agentInput, err := s.buildAgentInput(ctx, input)
	if err != nil {
		s.logger.Error("query agent context build failed",
			append(queryAgentLogFields(input), zap.Error(err))...,
		)
		return nil, err
	}
	result, err := s.agent.Run(ctx, query.AgentRequest{
		TenantID:              agentInput.TenantID,
		ProjectID:             agentInput.ProjectID,
		DatasourceID:          agentInput.DatasourceID,
		SelectedDatasourceIDs: agentInput.SelectedDatasourceIDs,
		UserID:                agentInput.UserID,
		Question:              agentInput.Question,
		MaxRows:               agentInput.MaxRows,
		Attempt:               agentInput.Attempt,
		PreviousSQL:           agentInput.PreviousSQL,
		PreviousError:         agentInput.PreviousError,
		Datasources:           agentInput.Datasources,
		BusinessTerms:         agentInput.BusinessTerms,
		Metrics:               agentInput.Metrics,
		FewShots:              agentInput.FewShots,
		Conversation:          agentInput.Conversation,
		Permission:            agentInput.Permission,
	})
	if err != nil {
		s.logger.Error("query agent ask failed",
			append(queryAgentLogFields(agentInput),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	s.logger.Info("query agent ask finished",
		append(queryAgentLogFields(agentInput),
			zap.Duration("duration", time.Since(started)),
			zap.String("intent", result.Intent),
			zap.Bool("need_clarification", result.NeedClarification),
			zap.Bool("review_passed", result.Review.Passed),
			zap.Int("step_count", len(result.Steps)),
			zap.Int("sql_task_count", len(result.SQLTasks)),
			zap.Bool("has_sql", strings.TrimSpace(result.SQL) != ""),
		)...,
	)
	return result, nil
}

func (s *QueryAgentService) StreamAsk(ctx context.Context, input AskInput, emit func(query.AgentEvent) error) error {
	started := time.Now()
	s.logger.Info("query agent stream started", queryAgentLogFields(input)...)
	agentInput, err := s.buildAgentInput(ctx, input)
	if err != nil {
		s.logger.Error("query agent stream context build failed",
			append(queryAgentLogFields(input), zap.Error(err))...,
		)
		return err
	}
	err = s.agent.Stream(ctx, query.AgentRequest{
		TenantID:              agentInput.TenantID,
		ProjectID:             agentInput.ProjectID,
		DatasourceID:          agentInput.DatasourceID,
		SelectedDatasourceIDs: agentInput.SelectedDatasourceIDs,
		UserID:                agentInput.UserID,
		Question:              agentInput.Question,
		MaxRows:               agentInput.MaxRows,
		Attempt:               agentInput.Attempt,
		PreviousSQL:           agentInput.PreviousSQL,
		PreviousError:         agentInput.PreviousError,
		Datasources:           agentInput.Datasources,
		BusinessTerms:         agentInput.BusinessTerms,
		Metrics:               agentInput.Metrics,
		FewShots:              agentInput.FewShots,
		Conversation:          agentInput.Conversation,
		Permission:            agentInput.Permission,
	}, emit)
	if err != nil {
		s.logger.Error("query agent stream failed",
			append(queryAgentLogFields(agentInput),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return err
	}
	s.logger.Info("query agent stream finished",
		append(queryAgentLogFields(agentInput),
			zap.Duration("duration", time.Since(started)),
		)...,
	)
	return nil
}

func (s *QueryAgentService) SynthesizeResult(ctx context.Context, input ResultSynthesisInput) (string, error) {
	if s.agent == nil {
		return "", query.ErrInvalidAgentInput
	}
	started := time.Now()
	s.logger.Info("query agent result synthesis started",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64s("selected_datasource_ids", input.SelectedDatasourceIDs),
		zap.Int("sql_task_count", len(input.Tasks)),
		zap.Int("execution_count", len(input.Executions)),
		zap.Int("question_chars", len([]rune(strings.TrimSpace(input.Question)))),
		zap.String("question_hash", sqlHash(input.Question)),
	)
	answer, err := s.agent.SynthesizeResults(ctx, query.AgentResultSynthesisRequest{
		AgentRequest: query.AgentRequest{
			TenantID:              input.TenantID,
			ProjectID:             input.ProjectID,
			UserID:                input.UserID,
			Question:              input.Question,
			MaxRows:               input.MaxRows,
			SelectedDatasourceIDs: input.SelectedDatasourceIDs,
			Datasources:           input.Datasources,
			Conversation:          input.Conversation,
			Permission:            input.Permission,
		},
		SQLTasks:         append([]query.AgentSQLTask(nil), input.Tasks...),
		ExecutionResults: buildAgentExecutionSummaries(input.Tasks, input.Executions),
	})
	if err != nil {
		s.logger.Error("query agent result synthesis failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Int("sql_task_count", len(input.Tasks)),
			zap.Int("execution_count", len(input.Executions)),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return "", err
	}
	s.logger.Info("query agent result synthesis finished",
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Int("sql_task_count", len(input.Tasks)),
		zap.Int("execution_count", len(input.Executions)),
		zap.Int("answer_chars", len([]rune(answer))),
		zap.Duration("duration", time.Since(started)),
	)
	return strings.TrimSpace(answer), nil
}

func (s *QueryAgentService) SynthesizeMultiResult(ctx context.Context, input MultiResultSynthesisInput) (string, error) {
	return s.SynthesizeResult(ctx, input)
}

func (s *QueryAgentService) buildAgentInput(ctx context.Context, input AskInput) (AskInput, error) {
	if s.agentContextBuilder == nil {
		return input, nil
	}
	agentContext, err := s.agentContextBuilder.BuildAgentContext(ctx, AgentContextInput{
		TenantID:              input.TenantID,
		ProjectID:             input.ProjectID,
		DatasourceID:          input.DatasourceID,
		SelectedDatasourceIDs: input.SelectedDatasourceIDs,
		Datasources:           input.Datasources,
		Permission:            input.Permission,
	})
	if err != nil {
		return AskInput{}, err
	}
	input.Datasources = agentContext.Datasources
	input.Permission = agentContext.Permission
	s.logger.Debug("query agent context prepared",
		append(queryAgentLogFields(input),
			zap.Int("datasource_count", len(input.Datasources)),
			zap.Int("allowed_datasource_count", len(input.Permission.AllowedDatasourceIDs)),
			zap.Int("allowed_table_count", len(input.Permission.AllowedTables)),
		)...,
	)
	return input, nil
}

func buildAgentExecutionSummaries(tasks []query.AgentSQLTask, executions []*QueryExecutionResult) []query.AgentExecutionSummary {
	out := make([]query.AgentExecutionSummary, 0, len(executions))
	for index, execution := range executions {
		var task query.AgentSQLTask
		if index < len(tasks) {
			task = tasks[index]
		}
		summary := query.AgentExecutionSummary{
			DatasourceID:   task.DatasourceID,
			DatasourceName: task.DatasourceName,
			Purpose:        task.Purpose,
		}
		if execution == nil {
			out = append(out, summary)
			continue
		}
		if summary.DatasourceID == 0 && execution.Execution != nil {
			summary.DatasourceID = execution.Execution.DatasourceID
		}
		summary.Columns = append([]string(nil), execution.Columns...)
		summary.Rows = copyLimitedRows(execution.Rows, 8)
		summary.RowCount = len(execution.Rows)
		if execution.Execution != nil && execution.Execution.RowCount != nil {
			summary.RowCount = *execution.Execution.RowCount
		}
		summary.ChartType = firstNonEmptyService(execution.Chart.Type, chartTypeFromExecution(execution))
		summary.Answer = execution.Answer
		summary.Error = execution.Error
		out = append(out, summary)
	}
	return out
}

func copyLimitedRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) == 0 {
		return nil
	}
	if len(rows) < limit {
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

func chartTypeFromExecution(result *QueryExecutionResult) string {
	if result == nil || result.Execution == nil {
		return ""
	}
	return result.Execution.ChartType
}

func queryAgentLogFields(input AskInput) []zap.Field {
	question := strings.TrimSpace(input.Question)
	return []zap.Field{
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("user_id", input.UserID),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Uint64s("selected_datasource_ids", input.SelectedDatasourceIDs),
		zap.Int("selected_datasource_count", len(input.SelectedDatasourceIDs)),
		zap.Int("question_chars", len([]rune(question))),
		zap.String("question_hash", sqlHash(question)),
		zap.Int("max_rows", input.MaxRows),
		zap.Int("attempt", input.Attempt),
		zap.Int("business_term_count", len(input.BusinessTerms)),
		zap.Int("metric_count", len(input.Metrics)),
		zap.Int("few_shot_count", len(input.FewShots)),
		zap.Int("conversation_message_count", len(input.Conversation)),
	}
}
