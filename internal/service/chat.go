package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
	"ling-shu/internal/rag"
	"ling-shu/internal/repository"

	"go.uber.org/zap"
)

const (
	recentConversationLimit = 20
	maxChatAgentLoops       = 3
)

type AgentRunner interface {
	Ask(ctx context.Context, input AskInput) (*query.AgentResult, error)
}

type StreamAgentRunner interface {
	StreamAsk(ctx context.Context, input AskInput, emit func(query.AgentEvent) error) error
}

type ResultSynthesizer interface {
	SynthesizeResult(ctx context.Context, input ResultSynthesisInput) (string, error)
}

type MultiResultSynthesizer interface {
	SynthesizeMultiResult(ctx context.Context, input MultiResultSynthesisInput) (string, error)
}

type QueryExecutor interface {
	ExecuteSQL(ctx context.Context, input ExecuteSQLInput) (*QueryExecutionResult, error)
}

type RAGRetriever interface {
	Retrieve(ctx context.Context, input rag.Request) (*rag.Context, error)
}

type ChatService struct {
	chatRepo            repository.ChatRepository
	agentRunner         AgentRunner
	queryExecutor       QueryExecutor
	ragRetriever        RAGRetriever
	agentContextBuilder AgentContextBuilder
	auditRecorder       auditpkg.Recorder
	logger              *zap.Logger
}

type CreateChatSessionInput struct {
	TenantID  uint64
	ProjectID uint64
	UserID    uint64
	Title     string
}

type ListChatSessionsInput struct {
	TenantID  uint64
	ProjectID uint64
	UserID    uint64
	Status    string
	Page      int
	PageSize  int
}

type ListChatMessagesInput struct {
	TenantID  uint64
	ProjectID uint64
	SessionID uint64
	Page      int
	PageSize  int
}

type SendChatMessageInput struct {
	TenantID              uint64
	ProjectID             uint64
	SessionID             uint64
	UserID                uint64
	Content               string
	DatasourceID          uint64
	SelectedDatasourceIDs []uint64
	MaxRows               int
	AutoExecute           bool
	Datasources           []query.AgentDatasource
	BusinessTerms         []query.AgentKnowledge
	Metrics               []query.AgentKnowledge
	FewShots              []query.AgentFewShot
	Permission            query.AgentPermission
	RequestID             string
	IP                    string
	UserAgent             string
}

type ResultSynthesisInput struct {
	TenantID              uint64
	ProjectID             uint64
	UserID                uint64
	Question              string
	MaxRows               int
	SelectedDatasourceIDs []uint64
	Datasources           []query.AgentDatasource
	Permission            query.AgentPermission
	Conversation          []query.AgentMessage
	Tasks                 []query.AgentSQLTask
	Executions            []*QueryExecutionResult
}

type MultiResultSynthesisInput = ResultSynthesisInput

type SendChatMessageResult struct {
	UserMessage      *model.ChatMessage      `json:"user_message"`
	AssistantMessage *model.ChatMessage      `json:"assistant_message"`
	Agent            *query.AgentResult      `json:"agent"`
	Execution        *QueryExecutionResult   `json:"execution,omitempty"`
	Executions       []*QueryExecutionResult `json:"executions,omitempty"`
	Loops            int                     `json:"loops,omitempty"`
	MaxLoops         int                     `json:"max_loops,omitempty"`
}

func NewChatService(chatRepo repository.ChatRepository, agentRunner AgentRunner, queryExecutor QueryExecutor, ragRetrievers ...RAGRetriever) *ChatService {
	var ragRetriever RAGRetriever
	if len(ragRetrievers) > 0 {
		ragRetriever = ragRetrievers[0]
	}
	return &ChatService{
		chatRepo:      chatRepo,
		agentRunner:   agentRunner,
		queryExecutor: queryExecutor,
		ragRetriever:  ragRetriever,
		logger:        zap.NewNop(),
	}
}

func (s *ChatService) SetAuditRecorder(recorder auditpkg.Recorder) {
	s.auditRecorder = recorder
}

func (s *ChatService) SetLogger(logger *zap.Logger) {
	if logger == nil {
		s.logger = zap.NewNop()
		return
	}
	s.logger = logger
}

func (s *ChatService) SetAgentContextBuilder(builder AgentContextBuilder) {
	s.agentContextBuilder = builder
}

func (s *ChatService) CreateSession(ctx context.Context, input CreateChatSessionInput) (*model.ChatSession, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "新会话"
	}
	if input.TenantID == 0 || input.ProjectID == 0 || input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	session := &model.ChatSession{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		UserID:    input.UserID,
		Title:     title,
		Status:    "active",
	}
	if err := s.chatRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ChatService) ListSessions(ctx context.Context, input ListChatSessionsInput) (PageResult[model.ChatSession], error) {
	if input.TenantID == 0 {
		return PageResult[model.ChatSession]{}, ErrInvalidInput
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.chatRepo.ListSessions(ctx, repository.ChatSessionFilter{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		UserID:    input.UserID,
		Status:    strings.TrimSpace(input.Status),
	}, p)
	if err != nil {
		return PageResult[model.ChatSession]{}, err
	}
	return PageResult[model.ChatSession]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func (s *ChatService) ListMessages(ctx context.Context, input ListChatMessagesInput) (PageResult[model.ChatMessage], error) {
	if input.TenantID == 0 || input.ProjectID == 0 || input.SessionID == 0 {
		return PageResult[model.ChatMessage]{}, ErrInvalidInput
	}
	if err := s.ensureSessionScope(ctx, input.SessionID, input.TenantID, input.ProjectID, 0); err != nil {
		return PageResult[model.ChatMessage]{}, err
	}
	p := NewPage(input.Page, input.PageSize)
	items, total, err := s.chatRepo.ListMessages(ctx, repository.ChatMessageFilter{
		TenantID:  input.TenantID,
		ProjectID: input.ProjectID,
		SessionID: input.SessionID,
	}, p)
	if err != nil {
		return PageResult[model.ChatMessage]{}, err
	}
	return PageResult[model.ChatMessage]{
		Items:    items,
		Total:    total,
		Page:     p.Page,
		PageSize: p.Limit(),
	}, nil
}

func (s *ChatService) SendMessage(ctx context.Context, input SendChatMessageInput) (*SendChatMessageResult, error) {
	return s.sendMessage(ctx, input, nil)
}

func (s *ChatService) StreamMessage(ctx context.Context, input SendChatMessageInput, emit func(query.AgentEvent) error) (*SendChatMessageResult, error) {
	return s.sendMessage(ctx, input, emit)
}

func chatLogFields(input SendChatMessageInput, content string, streaming bool) []zap.Field {
	return []zap.Field{
		zap.String("request_id", input.RequestID),
		zap.Uint64("tenant_id", input.TenantID),
		zap.Uint64("project_id", input.ProjectID),
		zap.Uint64("session_id", input.SessionID),
		zap.Uint64("user_id", input.UserID),
		zap.Bool("streaming", streaming),
		zap.Bool("auto_execute", input.AutoExecute),
		zap.Int("max_rows", input.MaxRows),
		zap.Int("question_chars", len([]rune(content))),
		zap.String("question_hash", sqlHash(content)),
		zap.Uint64("datasource_id", input.DatasourceID),
		zap.Uint64s("selected_datasource_ids", input.SelectedDatasourceIDs),
		zap.Int("selected_datasource_count", len(input.SelectedDatasourceIDs)),
	}
}

func (s *ChatService) sendMessage(ctx context.Context, input SendChatMessageInput, emit func(query.AgentEvent) error) (*SendChatMessageResult, error) {
	started := time.Now()
	content := strings.TrimSpace(input.Content)
	if input.TenantID == 0 || input.ProjectID == 0 || input.SessionID == 0 || input.UserID == 0 || content == "" {
		return nil, ErrInvalidInput
	}
	if s.agentRunner == nil {
		return nil, ErrInvalidInput
	}
	logFields := chatLogFields(input, content, emit != nil)
	s.logger.Info("chat message started", logFields...)
	if err := s.ensureSessionScope(ctx, input.SessionID, input.TenantID, input.ProjectID, input.UserID); err != nil {
		s.logger.Warn("chat message rejected by session scope",
			append(chatLogFields(input, content, emit != nil), zap.Error(err))...,
		)
		return nil, err
	}

	recentMessages, err := s.chatRepo.GetRecentMessages(ctx, input.SessionID, recentConversationLimit)
	if err != nil {
		s.logger.Error("chat recent messages load failed",
			append(chatLogFields(input, content, emit != nil), zap.Error(err))...,
		)
		return nil, err
	}
	conversation := buildConversation(recentMessages)
	conversation = append(conversation, query.AgentMessage{Role: "user", Content: content})
	businessTerms, metrics, fewShots, err := s.agentKnowledge(ctx, input)
	if err != nil {
		s.logger.Error("chat knowledge context load failed",
			append(chatLogFields(input, content, emit != nil), zap.Error(err))...,
		)
		return nil, err
	}
	agentContext, err := s.buildAgentContext(ctx, input)
	if err != nil {
		s.logger.Error("chat agent context build failed",
			append(chatLogFields(input, content, emit != nil), zap.Error(err))...,
		)
		return nil, err
	}
	s.logger.Info("chat context prepared",
		append(chatLogFields(input, content, emit != nil),
			zap.Int("recent_message_count", len(recentMessages)),
			zap.Int("conversation_message_count", len(conversation)),
			zap.Int("datasource_count", len(agentContext.Datasources)),
			zap.Int("business_term_count", len(businessTerms)),
			zap.Int("metric_count", len(metrics)),
			zap.Int("few_shot_count", len(fewShots)),
			zap.Int("allowed_datasource_count", len(agentContext.Permission.AllowedDatasourceIDs)),
			zap.Int("allowed_table_count", len(agentContext.Permission.AllowedTables)),
		)...,
	)

	userMessage := &model.ChatMessage{
		TenantID:    input.TenantID,
		ProjectID:   input.ProjectID,
		SessionID:   input.SessionID,
		UserID:      input.UserID,
		Role:        "user",
		Content:     content,
		ContentType: "text",
	}
	if err := s.chatRepo.CreateMessage(ctx, userMessage); err != nil {
		s.logger.Error("chat user message persist failed",
			append(chatLogFields(input, content, emit != nil), zap.Error(err))...,
		)
		return nil, err
	}

	agentResult, execution, executions, loops, err := s.runAgentLoop(ctx, input, content, agentContext, businessTerms, metrics, fewShots, conversation, emit)
	if err != nil {
		s.logger.Error("chat agent loop failed",
			append(chatLogFields(input, content, emit != nil),
				zap.Int("loops", loops),
				zap.Duration("duration", time.Since(started)),
				zap.Error(err),
			)...,
		)
		return nil, err
	}

	assistantContent, err := marshalAssistantMessage(agentResult, execution, executions, loops, maxChatAgentLoops)
	if err != nil {
		s.logger.Error("chat assistant message marshal failed",
			append(chatLogFields(input, content, emit != nil),
				zap.Int("loops", loops),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	assistantMessage := &model.ChatMessage{
		TenantID:    input.TenantID,
		ProjectID:   input.ProjectID,
		SessionID:   input.SessionID,
		Role:        "assistant",
		Content:     assistantContent,
		ContentType: "agent_result",
	}
	if execution != nil && execution.Execution != nil {
		assistantMessage.QueryExecutionID = execution.Execution.ID
	}
	if err := s.chatRepo.CreateMessage(ctx, assistantMessage); err != nil {
		s.logger.Error("chat assistant message persist failed",
			append(chatLogFields(input, content, emit != nil),
				zap.Int("loops", loops),
				zap.Error(err),
			)...,
		)
		return nil, err
	}
	s.recordChatAudit(ctx, input, userMessage, assistantMessage, agentResult, execution)
	s.logger.Info("chat message finished",
		append(chatLogFields(input, content, emit != nil),
			zap.Uint64("user_message_id", userMessage.ID),
			zap.Uint64("assistant_message_id", assistantMessage.ID),
			zap.Int("loops", loops),
			zap.Int("execution_count", len(executions)),
			zap.Bool("has_execution", execution != nil),
			zap.Duration("duration", time.Since(started)),
		)...,
	)

	return &SendChatMessageResult{
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Agent:            agentResult,
		Execution:        execution,
		Executions:       executions,
		Loops:            loops,
		MaxLoops:         maxChatAgentLoops,
	}, nil
}

func (s *ChatService) runAgentLoop(ctx context.Context, input SendChatMessageInput, content string, agentContext AgentContext, businessTerms []query.AgentKnowledge, metrics []query.AgentKnowledge, fewShots []query.AgentFewShot, conversation []query.AgentMessage, emit func(query.AgentEvent) error) (*query.AgentResult, *QueryExecutionResult, []*QueryExecutionResult, int, error) {
	var (
		agentResult   *query.AgentResult
		execution     *QueryExecutionResult
		executions    []*QueryExecutionResult
		mergedSteps   []query.AgentEvent
		previousSQL   string
		previousError string
		loops         int
	)

	mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventObservation, "rag.lookup", knowledgeContextObservation(businessTerms, metrics, fewShots), "", nil)
	if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
		return nil, nil, nil, loops, err
	}

	for attempt := 1; attempt <= maxChatAgentLoops; attempt++ {
		loops = attempt
		attemptStarted := time.Now()
		s.logger.Info("chat agent loop attempt started",
			append(chatLogFields(input, content, emit != nil),
				zap.Int("attempt", attempt),
				zap.Int("previous_sql_chars", len(previousSQL)),
				zap.Bool("has_previous_error", strings.TrimSpace(previousError) != ""),
			)...,
		)
		mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventThought, fmt.Sprintf("第 %d 轮尝试", attempt), "开始理解用户任务并选择可用工具。", "", nil)
		if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
			return nil, nil, nil, loops, err
		}

		result, steps, err := s.askAgent(ctx, AskInput{
			TenantID:              input.TenantID,
			ProjectID:             input.ProjectID,
			DatasourceID:          input.DatasourceID,
			SelectedDatasourceIDs: input.SelectedDatasourceIDs,
			UserID:                input.UserID,
			Question:              content,
			MaxRows:               input.MaxRows,
			Attempt:               attempt,
			PreviousSQL:           previousSQL,
			PreviousError:         previousError,
			Datasources:           agentContext.Datasources,
			BusinessTerms:         businessTerms,
			Metrics:               metrics,
			FewShots:              fewShots,
			Conversation:          conversation,
			Permission:            agentContext.Permission,
		}, emit)
		if err != nil {
			s.logger.Error("chat agent ask failed",
				append(chatLogFields(input, content, emit != nil),
					zap.Int("attempt", attempt),
					zap.Duration("duration", time.Since(attemptStarted)),
					zap.Error(err),
				)...,
			)
			return nil, nil, nil, loops, err
		}
		if result == nil {
			result = &query.AgentResult{}
		}
		agentResult = result
		mergedSteps = appendRenumberedAgentSteps(mergedSteps, steps)
		s.logger.Info("chat agent ask finished",
			append(chatLogFields(input, content, emit != nil),
				zap.Int("attempt", attempt),
				zap.String("intent", result.Intent),
				zap.Bool("need_clarification", result.NeedClarification),
				zap.Bool("review_passed", result.Review.Passed),
				zap.Int("step_count", len(steps)),
				zap.Int("sql_task_count", len(result.SQLTasks)),
				zap.Bool("has_sql", strings.TrimSpace(result.SQL) != ""),
				zap.Duration("duration", time.Since(attemptStarted)),
			)...,
		)
		if len(result.SQLTasks) > 0 {
			if result.NeedClarification {
				s.logger.Info("chat agent loop stopped for clarification",
					append(chatLogFields(input, content, emit != nil),
						zap.Int("attempt", attempt),
						zap.Int("sql_task_count", len(result.SQLTasks)),
					)...,
				)
				break
			}
			if !allSQLTasksPassed(result.SQLTasks) {
				previousSQL = sqlTaskDebugList(result.SQLTasks)
				previousError = "SQL 审核失败：" + result.Review.BlockedReason
				s.logger.Warn("chat sql tasks review failed",
					append(chatLogFields(input, content, emit != nil),
						zap.Int("attempt", attempt),
						zap.Int("sql_task_count", len(result.SQLTasks)),
						zap.String("risk_level", result.Review.RiskLevel),
						zap.String("blocked_reason", result.Review.BlockedReason),
					)...,
				)
				mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventObservation, "sql.review.failed", previousError, previousSQL, &result.Review)
				if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
					return nil, nil, nil, loops, err
				}
				if attempt < maxChatAgentLoops {
					continue
				}
				break
			}
			if !input.AutoExecute || s.queryExecutor == nil {
				s.logger.Info("chat sql tasks ready without auto execute",
					append(chatLogFields(input, content, emit != nil),
						zap.Int("attempt", attempt),
						zap.Int("sql_task_count", len(result.SQLTasks)),
						zap.Bool("auto_execute", input.AutoExecute),
						zap.Bool("query_executor_configured", s.queryExecutor != nil),
					)...,
				)
				break
			}
			runResults, err := s.executeSQLTasks(ctx, input, content, result.SQLTasks, &mergedSteps, emit)
			executions = runResults
			if err == nil {
				s.attachMultiExecutionAnswer(ctx, input, content, agentContext, conversation, result, runResults, &mergedSteps, emit)
				execution = buildMultiDatasourceChartResult(input, content, result.SQLTasks, runResults, result.Answer)
				if execution == nil && len(runResults) > 0 {
					execution = runResults[0]
				}
				s.logger.Info("chat multi datasource execution finished",
					append(chatLogFields(input, content, emit != nil),
						zap.Int("attempt", attempt),
						zap.Int("sql_task_count", len(result.SQLTasks)),
						zap.Int("execution_count", len(runResults)),
						zap.Duration("duration", time.Since(attemptStarted)),
					)...,
				)
				break
			}
			if len(runResults) > 0 {
				execution = runResults[0]
			}
			previousSQL = sqlTaskDebugList(result.SQLTasks)
			previousError = "SQL 执行失败：" + err.Error()
			s.logger.Warn("chat multi datasource auto execute failed",
				zap.Uint64("tenant_id", input.TenantID),
				zap.Uint64("project_id", input.ProjectID),
				zap.Uint64("session_id", input.SessionID),
				zap.Int("attempt", attempt),
				zap.Error(err),
			)
			if attempt < maxChatAgentLoops {
				continue
			}
			break
		}
		if result.NeedClarification || strings.TrimSpace(result.SQL) == "" {
			s.logger.Info("chat agent loop stopped without executable sql",
				append(chatLogFields(input, content, emit != nil),
					zap.Int("attempt", attempt),
					zap.Bool("need_clarification", result.NeedClarification),
					zap.Bool("has_sql", strings.TrimSpace(result.SQL) != ""),
				)...,
			)
			break
		}

		if !result.Review.Passed {
			previousSQL = result.SQL
			previousError = "SQL 审核失败：" + result.Review.BlockedReason
			s.logger.Warn("chat sql review failed",
				append(chatLogFields(input, content, emit != nil),
					zap.Int("attempt", attempt),
					zap.String("risk_level", result.Review.RiskLevel),
					zap.String("blocked_reason", result.Review.BlockedReason),
					zap.String("sql_hash", sqlHash(result.SQL)),
				)...,
			)
			mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventObservation, "sql.review.failed", previousError, result.SQL, &result.Review)
			if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
				return nil, nil, nil, loops, err
			}
			if attempt < maxChatAgentLoops {
				continue
			}
			break
		}

		if !input.AutoExecute || s.queryExecutor == nil {
			s.logger.Info("chat sql ready without auto execute",
				append(chatLogFields(input, content, emit != nil),
					zap.Int("attempt", attempt),
					zap.Bool("auto_execute", input.AutoExecute),
					zap.Bool("query_executor_configured", s.queryExecutor != nil),
					zap.String("sql_hash", sqlHash(result.SQL)),
				)...,
			)
			break
		}
		datasourceID := executableDatasourceID(result)
		if datasourceID == 0 {
			s.logger.Warn("chat sql execution skipped without datasource",
				append(chatLogFields(input, content, emit != nil),
					zap.Int("attempt", attempt),
					zap.String("sql_hash", sqlHash(result.SQL)),
				)...,
			)
			break
		}
		mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventAction, "sql.execute", "执行审核通过的 SQL。", result.SQL, &result.Review)
		if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
			return nil, nil, nil, loops, err
		}
		runResult, err := s.queryExecutor.ExecuteSQL(ctx, ExecuteSQLInput{
			TenantID:     input.TenantID,
			ProjectID:    input.ProjectID,
			DatasourceID: datasourceID,
			SessionID:    input.SessionID,
			UserID:       input.UserID,
			Question:     content,
			SQL:          result.SQL,
			MaxRows:      input.MaxRows,
		})
		execution = runResult
		if err == nil {
			rowCount := 0
			if runResult != nil {
				rowCount = len(runResult.Rows)
			}
			mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventObservation, "sql.execute", fmt.Sprintf("SQL 执行成功，返回 %d 行数据。", rowCount), result.SQL, &result.Review)
			if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
				return nil, nil, nil, loops, err
			}
			s.logger.Info("chat sql execution finished",
				append(chatLogFields(input, content, emit != nil),
					zap.Int("attempt", attempt),
					zap.Uint64("datasource_id", datasourceID),
					zap.Int("row_count", rowCount),
					zap.String("sql_hash", sqlHash(result.SQL)),
					zap.Duration("duration", time.Since(attemptStarted)),
				)...,
			)
			s.attachSingleExecutionAnswer(ctx, input, content, agentContext, conversation, result, runResult, &mergedSteps, emit)
			break
		}

		execution = normalizeFailedExecution(runResult, err)
		previousSQL = result.SQL
		previousError = "SQL 执行失败：" + err.Error()
		mergedSteps = appendServiceAgentEvent(mergedSteps, query.EventError, "sql.execute.failed", previousError, result.SQL, &result.Review)
		if err := emitAgentEvent(emit, mergedSteps[len(mergedSteps)-1]); err != nil {
			return nil, nil, nil, loops, err
		}
		s.logger.Warn("chat auto execute failed",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("session_id", input.SessionID),
			zap.Uint64("datasource_id", datasourceID),
			zap.Int("attempt", attempt),
			zap.Error(err),
		)
		if attempt < maxChatAgentLoops {
			continue
		}
		break
	}

	if agentResult != nil {
		agentResult.Steps = mergedSteps
	}
	s.logger.Info("chat agent loop finished",
		append(chatLogFields(input, content, emit != nil),
			zap.Int("loops", loops),
			zap.Bool("has_agent_result", agentResult != nil),
			zap.Bool("has_execution", execution != nil),
			zap.Int("execution_count", len(executions)),
			zap.Int("merged_step_count", len(mergedSteps)),
		)...,
	)
	return agentResult, execution, executions, loops, nil
}

func (s *ChatService) askAgent(ctx context.Context, input AskInput, emit func(query.AgentEvent) error) (*query.AgentResult, []query.AgentEvent, error) {
	if emit != nil {
		if streamer, ok := s.agentRunner.(StreamAgentRunner); ok {
			var (
				final *query.AgentResult
				steps []query.AgentEvent
			)
			err := streamer.StreamAsk(ctx, input, func(event query.AgentEvent) error {
				if event.Final != nil {
					final = event.Final
				}
				event.Final = nil
				steps = append(steps, event)
				return emit(event)
			})
			if err != nil {
				return nil, steps, err
			}
			if final == nil {
				return nil, steps, ErrInvalidInput
			}
			final.Steps = steps
			return final, steps, nil
		}
	}
	result, err := s.agentRunner.Ask(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	if result == nil {
		result = &query.AgentResult{}
	}
	steps := append([]query.AgentEvent(nil), result.Steps...)
	if emit != nil {
		for _, event := range steps {
			event.Final = nil
			if err := emit(event); err != nil {
				return nil, steps, err
			}
		}
	}
	return result, steps, nil
}

func (s *ChatService) executeSQLTasks(ctx context.Context, input SendChatMessageInput, question string, tasks []query.AgentSQLTask, mergedSteps *[]query.AgentEvent, emit func(query.AgentEvent) error) ([]*QueryExecutionResult, error) {
	results := make([]*QueryExecutionResult, 0, len(tasks))
	var taskErrors []error
	successCount := 0
	for index, task := range tasks {
		review := task.Review
		content := fmt.Sprintf("执行第 %d 个数据源查询：%s。", index+1, firstNonEmptyService(task.Purpose, task.DatasourceName, fmt.Sprintf("datasource_%d", task.DatasourceID)))
		*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventAction, "sql.execute", content, task.SQL, &review)
		if err := emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1]); err != nil {
			return results, err
		}
		runResult, err := s.queryExecutor.ExecuteSQL(ctx, ExecuteSQLInput{
			TenantID:     input.TenantID,
			ProjectID:    input.ProjectID,
			DatasourceID: task.DatasourceID,
			SessionID:    input.SessionID,
			UserID:       input.UserID,
			Question:     question,
			SQL:          task.SQL,
			MaxRows:      input.MaxRows,
		})
		if err != nil {
			failed := normalizeFailedExecution(runResult, err)
			results = append(results, failed)
			taskErrors = append(taskErrors, fmt.Errorf("datasource %d: %w", task.DatasourceID, err))
			errorContent := fmt.Sprintf("第 %d 个数据源查询失败：%s", index+1, err.Error())
			*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventError, "sql.execute.failed", errorContent, task.SQL, &review)
			if emitErr := emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1]); emitErr != nil {
				return results, emitErr
			}
			s.logger.Warn("chat multi datasource task execute failed",
				append(chatLogFields(input, question, emit != nil),
					zap.Int("task_index", index+1),
					zap.Int("task_count", len(tasks)),
					zap.Uint64("datasource_id", task.DatasourceID),
					zap.String("sql_hash", sqlHash(task.SQL)),
					zap.Error(err),
				)...,
			)
			continue
		}
		results = append(results, runResult)
		successCount++
		rowCount := 0
		if runResult != nil {
			rowCount = len(runResult.Rows)
		}
		*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventObservation, "sql.execute", fmt.Sprintf("第 %d 个数据源查询成功，返回 %d 行数据。", index+1, rowCount), task.SQL, &review)
		if err := emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1]); err != nil {
			return results, err
		}
	}
	if len(taskErrors) > 0 {
		if successCount == 0 {
			return results, fmt.Errorf("所有数据源查询均失败：%w", errors.Join(taskErrors...))
		}
		partialContent := fmt.Sprintf("共有 %d 个数据源查询成功，%d 个数据源查询失败；已保留成功结果继续综合。", successCount, len(taskErrors))
		*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventObservation, "sql.execute.partial", partialContent, "", nil)
		if err := emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1]); err != nil {
			return results, err
		}
		s.logger.Warn("chat multi datasource execution partially failed",
			append(chatLogFields(input, question, emit != nil),
				zap.Int("task_count", len(tasks)),
				zap.Int("success_count", successCount),
				zap.Int("failure_count", len(taskErrors)),
				zap.Error(errors.Join(taskErrors...)),
			)...,
		)
	}
	return results, nil
}

func (s *ChatService) attachMultiExecutionAnswer(ctx context.Context, input SendChatMessageInput, question string, agentContext AgentContext, conversation []query.AgentMessage, result *query.AgentResult, executions []*QueryExecutionResult, mergedSteps *[]query.AgentEvent, emit func(query.AgentEvent) error) {
	if result == nil || len(result.SQLTasks) == 0 {
		return
	}
	summary, _ := s.synthesizeExecutionAnswer(ctx, input, question, agentContext, conversation, result.SQLTasks, executions, mergedSteps, emit)
	if summary == "" {
		summary = summarizeMultiDatasourceExecutions(result.SQLTasks, executions)
	}
	if summary == "" {
		return
	}
	result.Answer = summary
	result.Explanation = summary
}

func (s *ChatService) attachSingleExecutionAnswer(ctx context.Context, input SendChatMessageInput, question string, agentContext AgentContext, conversation []query.AgentMessage, result *query.AgentResult, execution *QueryExecutionResult, mergedSteps *[]query.AgentEvent, emit func(query.AgentEvent) error) {
	if result == nil || execution == nil {
		return
	}
	task := singleResultSynthesisTask(result, execution, agentContext.Datasources)
	answer, synthesized := s.synthesizeExecutionAnswer(ctx, input, question, agentContext, conversation, []query.AgentSQLTask{task}, []*QueryExecutionResult{execution}, mergedSteps, emit)
	if answer == "" {
		answer = strings.TrimSpace(execution.Answer)
	}
	if answer == "" {
		answer, _, _ = summarizeExecutionResult(execution)
	}
	if answer != "" {
		result.Answer = answer
		result.Explanation = answer
		execution.Answer = answer
		if synthesized || strings.TrimSpace(execution.SpeechSummary) == "" {
			execution.SpeechSummary = answer
		}
	}
}

func (s *ChatService) synthesizeExecutionAnswer(ctx context.Context, input SendChatMessageInput, question string, agentContext AgentContext, conversation []query.AgentMessage, tasks []query.AgentSQLTask, executions []*QueryExecutionResult, mergedSteps *[]query.AgentEvent, emit func(query.AgentEvent) error) (string, bool) {
	if len(tasks) == 0 || len(executions) == 0 || !s.hasResultSynthesizer() {
		return "", false
	}
	*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventThought, "观察执行结果", "观察 SQL 工具返回的数据，判断是否已经足够回答用户。", "", nil)
	if err := emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1]); err != nil {
		s.logger.Warn("chat result observation thought emit failed",
			append(chatLogFields(input, question, emit != nil), zap.Error(err))...,
		)
	}
	*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventAction, "result.synthesize", "基于工具观察结果生成最终业务答案。", "", nil)
	if err := emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1]); err != nil {
		s.logger.Warn("chat result synthesis action emit failed",
			append(chatLogFields(input, question, emit != nil), zap.Error(err))...,
		)
	}
	answer, err := s.callResultSynthesizer(ctx, ResultSynthesisInput{
		TenantID:              input.TenantID,
		ProjectID:             input.ProjectID,
		UserID:                input.UserID,
		Question:              question,
		MaxRows:               input.MaxRows,
		SelectedDatasourceIDs: input.SelectedDatasourceIDs,
		Datasources:           agentContext.Datasources,
		Permission:            agentContext.Permission,
		Conversation:          conversation,
		Tasks:                 tasks,
		Executions:            executions,
	})
	if err != nil {
		s.logger.Warn("chat result synthesis failed; using local summary",
			append(chatLogFields(input, question, emit != nil),
				zap.Int("sql_task_count", len(tasks)),
				zap.Int("execution_count", len(executions)),
				zap.Error(err),
			)...,
		)
		*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventObservation, "result.synthesize", "结果综合暂时不可用，已使用本地摘要兜底。", "", nil)
		_ = emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1])
		return "", false
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventObservation, "result.synthesize", "结果综合未返回内容，已使用本地摘要兜底。", "", nil)
		_ = emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1])
		return "", false
	}
	if hasUnresolvedTemplatePlaceholder(answer) {
		s.logger.Warn("chat result synthesis returned unresolved placeholder; using local summary",
			append(chatLogFields(input, question, emit != nil),
				zap.Int("sql_task_count", len(tasks)),
				zap.Int("execution_count", len(executions)),
			)...,
		)
		*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventObservation, "result.synthesize", "结果综合包含未替换占位符，已使用本地摘要兜底。", "", nil)
		_ = emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1])
		return "", false
	}
	*mergedSteps = appendServiceAgentEvent(*mergedSteps, query.EventObservation, "result.synthesize", "已根据工具观察结果生成最终答案。", "", nil)
	_ = emitAgentEvent(emit, (*mergedSteps)[len(*mergedSteps)-1])
	return answer, true
}

func (s *ChatService) hasResultSynthesizer() bool {
	if _, ok := s.agentRunner.(ResultSynthesizer); ok {
		return true
	}
	_, ok := s.agentRunner.(MultiResultSynthesizer)
	return ok
}

func (s *ChatService) callResultSynthesizer(ctx context.Context, input ResultSynthesisInput) (string, error) {
	if synthesizer, ok := s.agentRunner.(ResultSynthesizer); ok {
		return synthesizer.SynthesizeResult(ctx, input)
	}
	if synthesizer, ok := s.agentRunner.(MultiResultSynthesizer); ok {
		return synthesizer.SynthesizeMultiResult(ctx, input)
	}
	return "", ErrInvalidInput
}

func singleResultSynthesisTask(result *query.AgentResult, execution *QueryExecutionResult, datasources []query.AgentDatasource) query.AgentSQLTask {
	datasourceID := result.DatasourceID
	if datasourceID == 0 {
		datasourceID = executableDatasourceID(result)
	}
	if datasourceID == 0 && execution != nil && execution.Execution != nil {
		datasourceID = execution.Execution.DatasourceID
	}
	datasourceName, dialect := datasourceLabel(datasources, datasourceID)
	return query.AgentSQLTask{
		DatasourceID:   datasourceID,
		DatasourceName: datasourceName,
		Dialect:        firstNonEmptyService(result.Dialect, dialect),
		Purpose:        firstNonEmptyService(result.Explanation, result.Answer, "回答用户问题"),
		SQL:            result.SQL,
		Review:         result.Review,
	}
}

func datasourceLabel(datasources []query.AgentDatasource, datasourceID uint64) (string, string) {
	for _, datasource := range datasources {
		if datasource.ID == datasourceID {
			return datasource.Name, datasource.Dialect
		}
	}
	return "", ""
}

func buildMultiDatasourceChartResult(input SendChatMessageInput, question string, tasks []query.AgentSQLTask, executions []*QueryExecutionResult, answer string) *QueryExecutionResult {
	rows := make([]map[string]any, 0, len(executions))
	metricName := ""
	for index, execution := range executions {
		column, value, _, ok := primaryExecutionNumber(execution)
		if !ok {
			continue
		}
		if metricName == "" {
			metricName = metricDisplayName(column)
		}
		label := fmt.Sprintf("数据源 %d", index+1)
		if index < len(tasks) {
			label = firstNonEmptyService(tasks[index].DatasourceName, tasks[index].Purpose, fmt.Sprintf("datasource_%d", tasks[index].DatasourceID))
		}
		rows = append(rows, map[string]any{
			"数据源":      label,
			metricName: value,
		})
	}
	if len(rows) < 2 {
		return nil
	}
	columns := []string{"数据源", metricName}
	chart := query.SuggestChart(columns, rows)
	rowCount := len(rows)
	return &QueryExecutionResult{
		Execution: &model.QueryExecution{
			TenantID:     input.TenantID,
			ProjectID:    input.ProjectID,
			SessionID:    input.SessionID,
			UserID:       input.UserID,
			Question:     question,
			Status:       "success",
			RowCount:     &rowCount,
			ChartType:    chart.Type,
			CreatedAt:    time.Now(),
			GeneratedSQL: sqlTaskDebugList(tasks),
		},
		Review:        aggregateSQLTaskReview(tasks, input.MaxRows),
		Chart:         chart,
		Answer:        answer,
		SpeechSummary: strings.TrimSpace(answer),
		Columns:       columns,
		Rows:          rows,
	}
}

func primaryExecutionNumber(result *QueryExecutionResult) (string, float64, string, bool) {
	if result == nil || len(result.Rows) == 0 {
		return "", 0, "", false
	}
	row := result.Rows[0]
	columns := result.Columns
	if len(columns) == 0 {
		columns = make([]string, 0, len(row))
		for column := range row {
			columns = append(columns, column)
		}
		sort.Strings(columns)
	}
	for _, column := range columns {
		value, ok := row[column]
		if !ok {
			continue
		}
		if parsed, ok := parseFloatValue(value); ok {
			return column, parsed, formatAny(value), true
		}
	}
	return "", 0, "", false
}

func metricDisplayName(column string) string {
	name := strings.TrimSpace(column)
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(name, "用户") || (strings.Contains(lower, "user") && strings.Contains(lower, "count")):
		return "用户数"
	case strings.Contains(name, "数量") || strings.Contains(lower, "count") || strings.Contains(lower, "total"):
		return "数量"
	case name != "":
		return name
	default:
		return "指标值"
	}
}

func aggregateSQLTaskReview(tasks []query.AgentSQLTask, limit int) query.ReviewResult {
	review := query.ReviewResult{
		Passed:    len(tasks) > 0,
		RiskLevel: "low",
		Limit:     limit,
	}
	var blocked []string
	for _, task := range tasks {
		if task.Review.RiskLevel == "high" {
			review.RiskLevel = "high"
		}
		if !task.Review.Passed {
			review.Passed = false
			blocked = append(blocked, fmt.Sprintf("datasource_%d: %s", task.DatasourceID, task.Review.BlockedReason))
		}
		review.Warnings = append(review.Warnings, task.Review.Warnings...)
	}
	if !review.Passed {
		review.BlockedReason = strings.Join(blocked, "；")
		if review.BlockedReason == "" {
			review.BlockedReason = "SQL task review failed"
		}
	}
	return review
}

func allSQLTasksPassed(tasks []query.AgentSQLTask) bool {
	if len(tasks) == 0 {
		return false
	}
	for _, task := range tasks {
		if !task.Review.Passed {
			return false
		}
	}
	return true
}

func sqlTaskDebugList(tasks []query.AgentSQLTask) string {
	parts := make([]string, 0, len(tasks))
	for _, task := range tasks {
		label := firstNonEmptyService(task.Purpose, task.DatasourceName, fmt.Sprintf("datasource_%d", task.DatasourceID))
		parts = append(parts, fmt.Sprintf("[%s] %s", label, task.SQL))
	}
	return strings.Join(parts, "\n")
}

type executionNumber struct {
	label string
	value float64
	raw   string
}

func summarizeMultiDatasourceExecutions(tasks []query.AgentSQLTask, executions []*QueryExecutionResult) string {
	if len(executions) == 0 {
		return ""
	}
	lines := make([]string, 0, len(executions)+2)
	numbers := make([]executionNumber, 0, len(executions))
	for index, execution := range executions {
		var task query.AgentSQLTask
		if index < len(tasks) {
			task = tasks[index]
		}
		label := firstNonEmptyService(task.DatasourceName, fmt.Sprintf("datasource_%d", task.DatasourceID))
		purpose := strings.TrimSpace(task.Purpose)
		if purpose != "" {
			label = fmt.Sprintf("%s（%s）", label, purpose)
		}
		summary, number, ok := summarizeExecutionResult(execution)
		if !ok {
			summary = "无返回数据"
		}
		lines = append(lines, fmt.Sprintf("%s：%s。", label, summary))
		if number != nil {
			number.label = label
			numbers = append(numbers, *number)
		}
	}
	answer := "已完成跨数据源查询：" + strings.Join(lines, " ")
	if len(numbers) == 2 {
		diff := numbers[0].value - numbers[1].value
		answer += fmt.Sprintf(" 两个数据源首个数值差异为 %s（%s - %s）。", formatFloat(diff), numbers[0].raw, numbers[1].raw)
	}
	return answer
}

func summarizeExecutionResult(result *QueryExecutionResult) (string, *executionNumber, bool) {
	if result == nil {
		return "", nil, false
	}
	if result.Error != "" {
		return "执行失败：" + result.Error, nil, true
	}
	if result.Answer != "" {
		return result.Answer, nil, true
	}
	if len(result.Rows) == 0 {
		return "", nil, false
	}
	row := result.Rows[0]
	columns := result.Columns
	if len(columns) == 0 {
		columns = make([]string, 0, len(row))
		for column := range row {
			columns = append(columns, column)
		}
		sort.Strings(columns)
	}
	parts := make([]string, 0, len(columns))
	var number *executionNumber
	for _, column := range columns {
		value, ok := row[column]
		if !ok {
			continue
		}
		display := formatAny(value)
		parts = append(parts, fmt.Sprintf("%s=%s", column, display))
		if number == nil {
			if parsed, ok := parseFloatValue(value); ok {
				number = &executionNumber{value: parsed, raw: display}
			}
		}
		if len(parts) >= 4 {
			break
		}
	}
	if len(parts) == 0 {
		return "", number, false
	}
	if len(result.Rows) > 1 {
		parts = append(parts, fmt.Sprintf("共 %d 行", len(result.Rows)))
	}
	return strings.Join(parts, "，"), number, true
}

func parseFloatValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", ""), 64)
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		return parsed, err == nil
	}
}

func formatAny(value any) string {
	switch v := value.(type) {
	case nil:
		return "-"
	case float32:
		return formatFloat(float64(v))
	case float64:
		return formatFloat(v)
	default:
		return fmt.Sprint(v)
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func firstNonEmptyService(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func hasUnresolvedTemplatePlaceholder(value string) bool {
	value = strings.TrimSpace(value)
	return strings.Contains(value, "{") && strings.Contains(value, "}")
}

func emitAgentEvent(emit func(query.AgentEvent) error, event query.AgentEvent) error {
	if emit == nil {
		return nil
	}
	event.Final = nil
	return emit(event)
}

func normalizeFailedExecution(result *QueryExecutionResult, err error) *QueryExecutionResult {
	if err == nil {
		return result
	}
	message := err.Error()
	if result == nil {
		return &QueryExecutionResult{
			Answer: "SQL 已生成，但自动执行失败：" + message,
			Error:  message,
		}
	}
	if result.Answer == "" {
		result.Answer = "SQL 已生成，但自动执行失败：" + message
	}
	if result.Error == "" {
		result.Error = message
	}
	return result
}

func appendServiceAgentEvent(events []query.AgentEvent, eventType string, name string, content string, sqlText string, review *query.ReviewResult) []query.AgentEvent {
	return append(events, query.AgentEvent{
		Type:       eventType,
		Step:       nextAgentStep(events),
		Name:       name,
		Content:    content,
		SQL:        sqlText,
		Review:     review,
		OccurredAt: time.Now(),
	})
}

func knowledgeContextObservation(businessTerms []query.AgentKnowledge, metrics []query.AgentKnowledge, fewShots []query.AgentFewShot) string {
	parts := []string{
		fmt.Sprintf("业务术语 %d 条", len(businessTerms)),
		fmt.Sprintf("指标 %d 条", len(metrics)),
		fmt.Sprintf("FewShot %d 条", len(fewShots)),
	}
	samples := knowledgeSamples(businessTerms, metrics, fewShots, 6)
	if len(samples) > 0 {
		parts = append(parts, "命中："+strings.Join(samples, "、"))
	}
	return strings.Join(parts, "，") + "。"
}

func knowledgeSamples(businessTerms []query.AgentKnowledge, metrics []query.AgentKnowledge, fewShots []query.AgentFewShot, limit int) []string {
	if limit <= 0 {
		return nil
	}
	samples := make([]string, 0, limit)
	for _, item := range businessTerms {
		if len(samples) >= limit {
			return samples
		}
		if name := strings.TrimSpace(item.Name); name != "" {
			samples = append(samples, name)
		}
	}
	for _, item := range metrics {
		if len(samples) >= limit {
			return samples
		}
		if name := strings.TrimSpace(item.Name); name != "" {
			samples = append(samples, name)
		}
	}
	for _, item := range fewShots {
		if len(samples) >= limit {
			return samples
		}
		if question := strings.TrimSpace(item.Question); question != "" {
			samples = append(samples, question)
		}
	}
	return samples
}

func appendRenumberedAgentSteps(dst []query.AgentEvent, src []query.AgentEvent) []query.AgentEvent {
	stepMap := map[int]int{}
	for _, event := range src {
		event.Final = nil
		if event.Step <= 0 {
			event.Step = nextAgentStep(dst)
		} else if mapped, ok := stepMap[event.Step]; ok {
			event.Step = mapped
		} else {
			mapped := nextAgentStep(dst)
			stepMap[event.Step] = mapped
			event.Step = mapped
		}
		dst = append(dst, event)
	}
	return dst
}

func nextAgentStep(events []query.AgentEvent) int {
	maxStep := 0
	for _, event := range events {
		if event.Step > maxStep {
			maxStep = event.Step
		}
	}
	return maxStep + 1
}

func (s *ChatService) buildAgentContext(ctx context.Context, input SendChatMessageInput) (AgentContext, error) {
	fallback := AgentContext{
		Datasources: append([]query.AgentDatasource(nil), input.Datasources...),
		Permission:  input.Permission,
	}
	if s.agentContextBuilder == nil {
		return fallback, nil
	}
	context, err := s.agentContextBuilder.BuildAgentContext(ctx, AgentContextInput{
		TenantID:              input.TenantID,
		ProjectID:             input.ProjectID,
		DatasourceID:          input.DatasourceID,
		SelectedDatasourceIDs: input.SelectedDatasourceIDs,
		Datasources:           input.Datasources,
		Permission:            input.Permission,
	})
	if err != nil {
		return AgentContext{}, err
	}
	return context, nil
}

func (s *ChatService) recordChatAudit(ctx context.Context, input SendChatMessageInput, userMessage *model.ChatMessage, assistantMessage *model.ChatMessage, agentResult *query.AgentResult, execution *QueryExecutionResult) {
	if s.auditRecorder == nil {
		return
	}
	payload := map[string]any{
		"user_message_id":      userMessage.ID,
		"assistant_message_id": assistantMessage.ID,
		"content_length":       len([]rune(userMessage.Content)),
		"datasource_id":        input.DatasourceID,
		"auto_execute":         input.AutoExecute,
	}
	if agentResult != nil {
		payload["agent_sql_generated"] = agentResult.SQL != ""
		payload["agent_review_passed"] = agentResult.Review.Passed
		payload["agent_datasource_id"] = agentResult.DatasourceID
		payload["requires_multi_datasource"] = agentResult.RequiresMultiDatasource
		payload["need_clarification"] = agentResult.NeedClarification
	}
	if execution != nil && execution.Execution != nil {
		payload["query_execution_id"] = execution.Execution.ID
		payload["query_status"] = execution.Execution.Status
	}
	if agentResult != nil && len(agentResult.SQLTasks) > 0 {
		payload["agent_sql_task_count"] = len(agentResult.SQLTasks)
	}
	_ = s.auditRecorder.Record(ctx, auditpkg.Event{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		UserID:       input.UserID,
		EventType:    auditpkg.EventChatMessage,
		ResourceType: auditpkg.ResourceChatSession,
		ResourceID:   input.SessionID,
		RequestID:    input.RequestID,
		IP:           input.IP,
		UserAgent:    input.UserAgent,
		Payload:      payload,
	})
}

func (s *ChatService) ensureSessionScope(ctx context.Context, sessionID uint64, tenantID uint64, projectID uint64, userID uint64) error {
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.TenantID != tenantID || session.ProjectID != projectID {
		return ErrInvalidInput
	}
	if userID > 0 && session.UserID != userID {
		return ErrInvalidInput
	}
	return nil
}

func (s *ChatService) agentKnowledge(ctx context.Context, input SendChatMessageInput) ([]query.AgentKnowledge, []query.AgentKnowledge, []query.AgentFewShot, error) {
	businessTerms := append([]query.AgentKnowledge(nil), input.BusinessTerms...)
	metrics := append([]query.AgentKnowledge(nil), input.Metrics...)
	fewShots := append([]query.AgentFewShot(nil), input.FewShots...)
	if s.ragRetriever == nil {
		return businessTerms, metrics, fewShots, nil
	}
	ragContext, err := s.ragRetriever.Retrieve(ctx, rag.Request{
		TenantID:     input.TenantID,
		ProjectID:    input.ProjectID,
		DatasourceID: ragDatasourceID(input),
		Question:     input.Content,
		Limit:        50,
	})
	if err != nil {
		s.logger.Warn("chat rag retrieval skipped",
			zap.Uint64("tenant_id", input.TenantID),
			zap.Uint64("project_id", input.ProjectID),
			zap.Uint64("datasource_id", ragDatasourceID(input)),
			zap.Error(err),
		)
		return businessTerms, metrics, fewShots, nil
	}
	if ragContext == nil {
		return businessTerms, metrics, fewShots, nil
	}
	businessTerms = append(businessTerms, ragContext.BusinessTerms...)
	metrics = append(metrics, ragContext.Metrics...)
	fewShots = append(fewShots, ragContext.FewShots...)
	return businessTerms, metrics, fewShots, nil
}

func ragDatasourceID(input SendChatMessageInput) uint64 {
	if input.DatasourceID > 0 {
		return input.DatasourceID
	}
	if len(input.SelectedDatasourceIDs) == 1 {
		return input.SelectedDatasourceIDs[0]
	}
	return 0
}

func buildConversation(messages []model.ChatMessage) []query.AgentMessage {
	conversation := make([]query.AgentMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		conversation = append(conversation, query.AgentMessage{Role: role, Content: content})
	}
	return conversation
}

func executableDatasourceID(result *query.AgentResult) uint64 {
	if result == nil {
		return 0
	}
	if result.DatasourceID > 0 {
		return result.DatasourceID
	}
	if len(result.DatasourceIDs) == 1 {
		return result.DatasourceIDs[0]
	}
	return 0
}

func marshalAssistantMessage(agentResult *query.AgentResult, execution *QueryExecutionResult, executions []*QueryExecutionResult, loops int, maxLoops int) (string, error) {
	payload := map[string]any{
		"agent":      agentResult,
		"execution":  execution,
		"executions": executions,
		"loops":      loops,
		"max_loops":  maxLoops,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
