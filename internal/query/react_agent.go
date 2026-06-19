package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ling-shu/internal/llm"

	"go.uber.org/zap"
)

var (
	ErrInvalidAgentInput   = errors.New("invalid agent input")
	ErrLLMNotConfigured    = errors.New("llm provider is not configured")
	ErrPromptNotConfigured = errors.New("prompt renderer is not configured")
)

type ReactAgent struct {
	llmProvider llm.Provider
	llmResolver LLMProviderResolver
	reviewer    *SQLReviewer
	prompts     *PromptRenderer
	logger      *zap.Logger
}

type LLMProviderResolver func(ctx context.Context, req AgentRequest) (llm.Provider, error)

const (
	AgentIntentChat       = "chat"
	AgentIntentQuery      = "query"
	AgentIntentMultiQuery = "multi_query"
	AgentIntentClarify    = "clarify"
)

type agentPlan struct {
	Intent                  string   `json:"intent"`
	Thought                 string   `json:"thought"`
	Answer                  string   `json:"answer"`
	Explanation             string   `json:"explanation"`
	DatasourceID            uint64   `json:"datasource_id"`
	DatasourceIDs           []uint64 `json:"datasource_ids"`
	RequiresMultiDatasource bool     `json:"requires_multi_datasource"`
	NeedClarification       bool     `json:"need_clarification"`
}

type generatedSQL struct {
	Intent                  string             `json:"intent"`
	Thought                 string             `json:"thought"`
	DatasourceID            uint64             `json:"datasource_id"`
	DatasourceIDs           []uint64           `json:"datasource_ids"`
	Dialect                 string             `json:"dialect"`
	RequiresMultiDatasource bool               `json:"requires_multi_datasource"`
	NeedClarification       bool               `json:"need_clarification"`
	SQL                     string             `json:"sql"`
	SQLTasks                []generatedSQLTask `json:"sql_tasks"`
	Answer                  string             `json:"answer"`
	Explanation             string             `json:"explanation"`
}

type generatedSQLTask struct {
	DatasourceID   uint64 `json:"datasource_id"`
	DatasourceName string `json:"datasource_name"`
	Dialect        string `json:"dialect"`
	Purpose        string `json:"purpose"`
	SQL            string `json:"sql"`
	Explanation    string `json:"explanation"`
}

func NewReactAgent(llmProvider llm.Provider, reviewer *SQLReviewer, prompts *PromptRenderer, logger *zap.Logger) *ReactAgent {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ReactAgent{
		llmProvider: llmProvider,
		reviewer:    reviewer,
		prompts:     prompts,
		logger:      logger,
	}
}

func (a *ReactAgent) SetLLMProviderResolver(resolver LLMProviderResolver) {
	a.llmResolver = resolver
}

func (a *ReactAgent) Run(ctx context.Context, req AgentRequest) (*AgentResult, error) {
	var steps []AgentEvent
	var final *AgentResult
	err := a.Stream(ctx, req, func(event AgentEvent) error {
		if event.Final != nil {
			final = event.Final
		}
		steps = append(steps, eventWithoutFinal(event))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if final == nil {
		return nil, errors.New("agent finished without final result")
	}
	final.Steps = steps
	return final, nil
}

func eventWithoutFinal(event AgentEvent) AgentEvent {
	event.Final = nil
	return event
}

func (a *ReactAgent) Stream(ctx context.Context, req AgentRequest, emit func(AgentEvent) error) error {
	if emit == nil {
		return ErrInvalidAgentInput
	}
	if strings.TrimSpace(req.Question) == "" || req.ProjectID == 0 {
		return ErrInvalidAgentInput
	}
	step := 1
	if err := a.emit(emit, EventThought, step, "理解问题", "分析用户问题、项目上下文和可用工具。", "", nil, nil); err != nil {
		return err
	}
	step++

	promptContext, err := a.buildPromptContext(req)
	if err != nil {
		return err
	}

	if looksLikePlainConversation(req.Question) {
		if err := a.emit(emit, EventObservation, step, "intent.chat", "识别为普通对话，不调用数据源和 SQL 工具。", "", nil, nil); err != nil {
			return err
		}
		step++
		final := chatAgentResult(req, promptContext, smallTalkAnswer(req.Question))
		return a.emit(emit, EventFinal, step, "final", final.Explanation, "", &final.Review, final)
	}

	llmProvider, err := a.resolveLLMProvider(ctx, req)
	if err != nil {
		return err
	}
	if llmProvider == nil || !llmProvider.Configured() {
		return ErrLLMNotConfigured
	}

	if err := a.emit(emit, EventAction, step, "llm.plan", "判断用户任务类型，决定是否需要调用数据源工具。", "", nil, nil); err != nil {
		return err
	}
	planStep := step
	step++

	plannerPrompt, err := a.buildPlannerPrompt(promptContext)
	if err != nil {
		return err
	}
	rawPlan, err := a.streamLLM(ctx, llmProvider, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: plannerPrompt},
			{Role: "user", Content: req.Question},
		},
		Temperature: floatPtr(0.0),
		MaxTokens:   800,
	}, emit, planStep, "llm.plan")
	if err != nil {
		return err
	}
	plan := parseAgentPlan(rawPlan)
	plan.Intent = normalizeAgentIntent(plan.Intent)
	if plan.Thought != "" {
		if err := a.emit(emit, EventThought, step, "任务判断", plan.Thought, "", nil, nil); err != nil {
			return err
		}
		step++
	}
	switch plan.Intent {
	case AgentIntentChat:
		answer := firstNonEmpty(plan.Answer, plan.Explanation, "你好，我是 Ling-Shu。你可以直接告诉我想分析的问题，我会判断是否需要查询数据。")
		final := chatAgentResult(req, promptContext, answer)
		return a.emit(emit, EventFinal, step, "final", final.Explanation, "", &final.Review, final)
	case AgentIntentClarify:
		final := clarificationAgentResult(req, promptContext, firstNonEmpty(plan.Explanation, plan.Answer, "这个问题还需要补充目标数据源、指标口径或时间范围。"), plan)
		return a.emit(emit, EventFinal, step, "final", final.Explanation, "", &final.Review, final)
	}

	if err := a.emit(emit, EventAction, step, "datasource.route", datasourceRouteObservation(promptContext), "", nil, nil); err != nil {
		return err
	}
	step++
	if err := a.emit(emit, EventAction, step, "metadata.lookup", "读取项目数据源元数据、业务术语、指标、FewShot 和权限上下文。", "", nil, nil); err != nil {
		return err
	}
	step++
	if err := a.emit(emit, EventObservation, step, "metadata.lookup", metadataObservation(promptContext), "", nil, nil); err != nil {
		return err
	}
	step++

	if err := a.emit(emit, EventAction, step, "llm.text2sql", "调用模型生成候选 SQL。", "", nil, nil); err != nil {
		return err
	}
	text2SQLStep := step
	step++

	systemPrompt, err := a.buildSystemPrompt(promptContext)
	if err != nil {
		return err
	}
	raw, err := a.streamLLM(ctx, llmProvider, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: req.Question},
		},
		Temperature: floatPtr(0.1),
		MaxTokens:   1200,
	}, emit, text2SQLStep, "llm.text2sql")
	if err != nil {
		return err
	}

	generated := parseGeneratedSQL(raw)
	reviewer := a.reviewer
	if reviewer == nil {
		reviewer = NewSQLReviewer(promptContext.MaxRows, 1000)
	}
	generated.Intent = normalizeAgentIntent(firstNonEmpty(generated.Intent, plan.Intent))
	switch generated.Intent {
	case AgentIntentChat:
		final := chatAgentResult(req, promptContext, firstNonEmpty(generated.Answer, generated.Explanation, "你好，我是 Ling-Shu。"))
		return a.emit(emit, EventFinal, step, "final", final.Explanation, "", &final.Review, final)
	case AgentIntentClarify:
		final := noSQLAgentResult(req, promptContext, generated)
		final.Intent = AgentIntentClarify
		final.NeedClarification = true
		return a.emit(emit, EventFinal, step, "final", final.Explanation, "", &final.Review, final)
	}
	if generated.Thought != "" {
		if err := a.emit(emit, EventThought, step, "生成思路", generated.Thought, "", nil, nil); err != nil {
			return err
		}
		step++
	}

	sqlTasks := normalizeGeneratedSQLTasks(generated, promptContext)
	if len(sqlTasks) > 0 {
		for i := range sqlTasks {
			content := fmt.Sprintf("审核第 %d 个数据源查询：%s。", i+1, firstNonEmpty(sqlTasks[i].Purpose, sqlTasks[i].DatasourceName, fmt.Sprintf("datasource_%d", sqlTasks[i].DatasourceID)))
			if err := a.emit(emit, EventAction, step, "sql.review", content, sqlTasks[i].SQL, nil, nil); err != nil {
				return err
			}
			step++
			review := reviewer.ReviewWithDialect(sqlTasks[i].SQL, promptContext.MaxRows, sqlTaskDialect(sqlTasks[i], promptContext))
			sqlTasks[i].Review = review
			if review.Passed {
				sqlTasks[i].SQL = review.NormalizedSQL
			}
			if err := a.emit(emit, EventObservation, step, "sql.review", reviewObservation(review), sqlTasks[i].SQL, &review, nil); err != nil {
				return err
			}
			step++
		}
		review := aggregateTaskReview(sqlTasks, promptContext.MaxRows)
		final := &AgentResult{
			Question:                req.Question,
			Intent:                  AgentIntentMultiQuery,
			SQLTasks:                sqlTasks,
			Answer:                  generated.Answer,
			Explanation:             firstNonEmpty(generated.Explanation, generated.Answer, "已生成跨数据源拆分查询计划。"),
			DatasourceID:            generated.DatasourceID,
			DatasourceIDs:           datasourceIDsFromTasks(sqlTasks, normalizeGeneratedDatasourceIDs(generated, promptContext)),
			Dialect:                 generatedDialect(generated, promptContext),
			RequiresMultiDatasource: true,
			NeedClarification:       generated.NeedClarification,
			Review:                  review,
		}
		a.logger.Info("react agent finished with sql tasks",
			zap.Uint64("tenant_id", req.TenantID),
			zap.Uint64("project_id", req.ProjectID),
			zap.Uint64s("datasource_ids", final.DatasourceIDs),
			zap.Int("sql_tasks", len(final.SQLTasks)),
			zap.Bool("review_passed", review.Passed),
		)
		return a.emit(emit, EventFinal, step, "final", "Agent 完成跨数据源查询计划。", "", &review, final)
	}

	if strings.TrimSpace(generated.SQL) == "" {
		final := noSQLAgentResult(req, promptContext, generated)
		a.logger.Info("react agent finished without executable sql",
			zap.Uint64("tenant_id", req.TenantID),
			zap.Uint64("project_id", req.ProjectID),
			zap.Uint64s("datasource_ids", final.DatasourceIDs),
			zap.Bool("requires_multi_datasource", final.RequiresMultiDatasource),
			zap.Bool("need_clarification", final.NeedClarification),
		)
		return a.emit(emit, EventFinal, step, "final", final.Explanation, "", &final.Review, final)
	}

	if err := a.emit(emit, EventAction, step, "sql.review", "执行 SQL 安全审核。", generated.SQL, nil, nil); err != nil {
		return err
	}
	step++

	review := reviewer.ReviewWithDialect(generated.SQL, promptContext.MaxRows, generatedDialect(generated, promptContext))
	if err := a.emit(emit, EventObservation, step, "sql.review", reviewObservation(review), review.NormalizedSQL, &review, nil); err != nil {
		return err
	}
	step++

	final := &AgentResult{
		Question:                req.Question,
		Intent:                  AgentIntentQuery,
		SQL:                     review.NormalizedSQL,
		Answer:                  generated.Answer,
		Explanation:             generated.Explanation,
		DatasourceID:            generated.DatasourceID,
		DatasourceIDs:           normalizeGeneratedDatasourceIDs(generated, promptContext),
		Dialect:                 generatedDialect(generated, promptContext),
		RequiresMultiDatasource: generated.RequiresMultiDatasource,
		NeedClarification:       generated.NeedClarification,
		Review:                  review,
	}
	if !review.Passed {
		final.SQL = generated.SQL
	}

	a.logger.Info("react agent finished",
		zap.Uint64("tenant_id", req.TenantID),
		zap.Uint64("project_id", req.ProjectID),
		zap.Uint64s("datasource_ids", final.DatasourceIDs),
		zap.String("dialect", final.Dialect),
		zap.Bool("review_passed", review.Passed),
	)
	return a.emit(emit, EventFinal, step, "final", "Agent 完成。", final.SQL, &review, final)
}

func (a *ReactAgent) SynthesizeResults(ctx context.Context, req AgentResultSynthesisRequest) (string, error) {
	if strings.TrimSpace(req.Question) == "" || req.ProjectID == 0 {
		return "", ErrInvalidAgentInput
	}
	if a.prompts == nil {
		return "", ErrPromptNotConfigured
	}
	llmProvider, err := a.resolveLLMProvider(ctx, req.AgentRequest)
	if err != nil {
		return "", err
	}
	if llmProvider == nil || !llmProvider.Configured() {
		return "", ErrLLMNotConfigured
	}
	dialectRules, err := a.prompts.DialectRuleMap()
	if err != nil {
		return "", err
	}
	promptContext := NewPromptContext(req.AgentRequest, dialectRules)
	promptContext.SQLTasks = append([]AgentSQLTask(nil), req.SQLTasks...)
	promptContext.ExecutionResults = append([]AgentExecutionSummary(nil), req.ExecutionResults...)
	systemPrompt, err := a.prompts.ResultSynthesisSystem(promptContext)
	if err != nil {
		return "", err
	}
	started := time.Now()
	resp, err := llmProvider.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: req.Question},
		},
		Temperature: floatPtr(0.1),
		MaxTokens:   800,
	})
	if err != nil {
		a.logger.Error("llm result synthesis failed",
			zap.Uint64("tenant_id", req.TenantID),
			zap.Uint64("project_id", req.ProjectID),
			zap.Int("sql_task_count", len(req.SQLTasks)),
			zap.Int("execution_result_count", len(req.ExecutionResults)),
			zap.Duration("duration", time.Since(started)),
			zap.Error(err),
		)
		return "", err
	}
	answer := ""
	if resp != nil {
		answer = strings.TrimSpace(resp.Content)
	}
	a.logger.Info("react agent synthesized execution results",
		zap.Uint64("tenant_id", req.TenantID),
		zap.Uint64("project_id", req.ProjectID),
		zap.Int("sql_task_count", len(req.SQLTasks)),
		zap.Int("execution_result_count", len(req.ExecutionResults)),
		zap.Int("answer_chars", len([]rune(answer))),
		zap.Duration("duration", time.Since(started)),
	)
	return answer, nil
}

func (a *ReactAgent) resolveLLMProvider(ctx context.Context, req AgentRequest) (llm.Provider, error) {
	if a.llmResolver != nil {
		return a.llmResolver(ctx, req)
	}
	return a.llmProvider, nil
}

func (a *ReactAgent) emit(emit func(AgentEvent) error, eventType string, step int, name string, content string, sqlText string, review *ReviewResult, final *AgentResult) error {
	event := AgentEvent{
		Type:       eventType,
		Step:       step,
		Name:       name,
		Content:    content,
		SQL:        sqlText,
		Review:     review,
		Final:      final,
		OccurredAt: time.Now(),
	}
	return emit(event)
}

func (a *ReactAgent) buildPromptContext(req AgentRequest) (PromptContext, error) {
	if a.prompts == nil {
		return PromptContext{}, ErrPromptNotConfigured
	}
	dialectRules, err := a.prompts.DialectRuleMap()
	if err != nil {
		return PromptContext{}, err
	}
	return NewPromptContext(req, dialectRules), nil
}

func (a *ReactAgent) buildSystemPrompt(data PromptContext) (string, error) {
	if a.prompts == nil {
		return "", ErrPromptNotConfigured
	}
	return a.prompts.Text2SQLSystem(data)
}

func (a *ReactAgent) buildPlannerPrompt(data PromptContext) (string, error) {
	if a.prompts == nil {
		return "", ErrPromptNotConfigured
	}
	return a.prompts.PlannerSystem(data)
}

func (a *ReactAgent) streamLLM(ctx context.Context, provider llm.Provider, req llm.ChatRequest, emit func(AgentEvent) error, step int, name string) (string, error) {
	var raw strings.Builder
	started := time.Now()
	model := req.Model
	if model == "" && provider != nil {
		model = provider.DefaultChatModel()
	}
	err := provider.StreamChat(ctx, req, func(event llm.ChatStreamEvent) error {
		if event.Delta != "" {
			raw.WriteString(event.Delta)
			return a.emit(emit, EventLLMDelta, step, name, event.Delta, "", nil, nil)
		}
		return nil
	})
	if err != nil {
		a.logger.Error("llm stream chat failed",
			zap.String("step_name", name),
			zap.Int("step", step),
			zap.String("model", model),
			zap.Duration("duration", time.Since(started)),
			zap.Int("prompt_messages", len(req.Messages)),
			zap.Int("partial_chars", raw.Len()),
			zap.Error(err),
		)
		return "", err
	}
	a.logger.Debug("llm stream chat finished",
		zap.String("step_name", name),
		zap.Int("step", step),
		zap.String("model", model),
		zap.Duration("duration", time.Since(started)),
		zap.Int("prompt_messages", len(req.Messages)),
		zap.Int("response_chars", raw.Len()),
	)
	return raw.String(), nil
}

func parseAgentPlan(content string) agentPlan {
	content = strings.TrimSpace(content)
	var out agentPlan
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &out); err == nil {
		return out
	}
	out.Intent = AgentIntentQuery
	out.Thought = strings.TrimSpace(content)
	return out
}

func parseGeneratedSQL(content string) generatedSQL {
	content = strings.TrimSpace(content)
	var out generatedSQL
	if err := json.Unmarshal([]byte(extractJSONObject(content)), &out); err == nil {
		return out
	}
	out.SQL = extractSQL(content)
	out.Explanation = strings.TrimSpace(content)
	return out
}

func normalizeAgentIntent(intent string) string {
	switch strings.ToLower(strings.TrimSpace(intent)) {
	case AgentIntentChat, "smalltalk", "conversation", "general":
		return AgentIntentChat
	case AgentIntentMultiQuery, "multi", "multi_datasource", "cross_datasource", "cross_source":
		return AgentIntentMultiQuery
	case AgentIntentClarify, "clarification", "need_clarification":
		return AgentIntentClarify
	case AgentIntentQuery, "text2sql", "sql":
		return AgentIntentQuery
	default:
		return AgentIntentQuery
	}
}

func looksLikePlainConversation(question string) bool {
	text := strings.ToLower(strings.TrimSpace(question))
	text = strings.Trim(text, " \t\r\n，。！？!?~～,.")
	compact := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(text)
	switch compact {
	case "你好", "您好", "hi", "hello", "hey", "在吗", "谢谢", "感谢", "辛苦了", "你是谁", "你能做什么", "帮助", "help":
		return true
	}
	return false
}

func smallTalkAnswer(question string) string {
	compact := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "").Replace(strings.TrimSpace(question))
	switch strings.Trim(compact, "，。！？!?~～,.") {
	case "你是谁":
		return "我是 Ling-Shu，一个面向项目数据源的自然语言问数 Agent。你可以直接描述想完成的分析任务。"
	case "你能做什么", "帮助", "help":
		return "我可以帮你理解业务问题、选择项目内的数据源、生成安全 SQL、执行查询，并把结果整理成表格或图表。"
	default:
		return "你好，我是 Ling-Shu。你可以直接告诉我想分析的问题，我会判断是否需要查询数据。"
	}
}

func chatAgentResult(req AgentRequest, data PromptContext, answer string) *AgentResult {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = "你好，我是 Ling-Shu。"
	}
	return &AgentResult{
		Question:    req.Question,
		Intent:      AgentIntentChat,
		Answer:      answer,
		Explanation: answer,
		Review: ReviewResult{
			Passed:        true,
			RiskLevel:     "none",
			NormalizedSQL: "",
			Limit:         data.MaxRows,
		},
	}
}

func clarificationAgentResult(req AgentRequest, data PromptContext, explanation string, plan agentPlan) *AgentResult {
	explanation = strings.TrimSpace(explanation)
	if explanation == "" {
		explanation = "这个问题还需要补充目标数据源、指标口径或时间范围。"
	}
	return &AgentResult{
		Question:                req.Question,
		Intent:                  AgentIntentClarify,
		Answer:                  explanation,
		Explanation:             explanation,
		DatasourceID:            plan.DatasourceID,
		DatasourceIDs:           uniqueUint64(append([]uint64(nil), plan.DatasourceIDs...)),
		RequiresMultiDatasource: plan.RequiresMultiDatasource,
		NeedClarification:       true,
		Review: ReviewResult{
			Passed:        false,
			RiskLevel:     "none",
			NormalizedSQL: "",
			BlockedReason: "need clarification",
			Limit:         data.MaxRows,
		},
	}
}

func normalizeGeneratedSQLTasks(generated generatedSQL, data PromptContext) []AgentSQLTask {
	tasks := make([]AgentSQLTask, 0, len(generated.SQLTasks))
	for _, task := range generated.SQLTasks {
		sqlText := strings.TrimSpace(task.SQL)
		if sqlText == "" {
			continue
		}
		datasourceID := task.DatasourceID
		if datasourceID == 0 && len(generated.SQLTasks) == 1 {
			datasourceID = generated.DatasourceID
		}
		if datasourceID == 0 {
			continue
		}
		ds := datasourceByID(data.AvailableDatasources, datasourceID)
		name := firstNonEmpty(task.DatasourceName, ds.Name, fmt.Sprintf("datasource_%d", datasourceID))
		dialect := strings.ToLower(firstNonEmpty(task.Dialect, ds.Dialect, generated.Dialect, data.DefaultDialect))
		tasks = append(tasks, AgentSQLTask{
			DatasourceID:   datasourceID,
			DatasourceName: name,
			Dialect:        dialect,
			Purpose:        firstNonEmpty(task.Purpose, task.Explanation),
			SQL:            sqlText,
			Review: ReviewResult{
				Passed:        false,
				RiskLevel:     "none",
				NormalizedSQL: "",
				Limit:         data.MaxRows,
			},
		})
	}
	return tasks
}

func datasourceByID(datasources []AgentDatasource, id uint64) AgentDatasource {
	for _, ds := range datasources {
		if ds.ID == id {
			return ds
		}
	}
	return AgentDatasource{}
}

func sqlTaskDialect(task AgentSQLTask, data PromptContext) string {
	if strings.TrimSpace(task.Dialect) != "" {
		return task.Dialect
	}
	ds := datasourceByID(data.AvailableDatasources, task.DatasourceID)
	if strings.TrimSpace(ds.Dialect) != "" {
		return ds.Dialect
	}
	return generatedDialect(generatedSQL{Dialect: task.Dialect, DatasourceID: task.DatasourceID}, data)
}

func aggregateTaskReview(tasks []AgentSQLTask, limit int) ReviewResult {
	review := ReviewResult{
		Passed:        len(tasks) > 0,
		RiskLevel:     "low",
		NormalizedSQL: "",
		Limit:         limit,
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
		if len(blocked) == 0 {
			review.BlockedReason = "SQL task review failed"
		} else {
			review.BlockedReason = strings.Join(blocked, "；")
		}
	}
	return review
}

func datasourceIDsFromTasks(tasks []AgentSQLTask, fallback []uint64) []uint64 {
	ids := make([]uint64, 0, len(tasks)+len(fallback))
	for _, task := range tasks {
		ids = append(ids, task.DatasourceID)
	}
	if len(ids) == 0 {
		ids = append(ids, fallback...)
	}
	return uniqueUint64(ids)
}

func extractJSONObject(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
	}
	return content
}

func extractSQL(content string) string {
	lower := strings.ToLower(content)
	if idx := strings.Index(lower, "```sql"); idx >= 0 {
		rest := content[idx+6:]
		if end := strings.Index(rest, "```"); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
	}
	if idx := strings.Index(lower, "select "); idx >= 0 {
		return strings.TrimSpace(content[idx:])
	}
	if idx := strings.Index(lower, "with "); idx >= 0 {
		return strings.TrimSpace(content[idx:])
	}
	return ""
}

func reviewObservation(review ReviewResult) string {
	if review.Passed {
		if len(review.Warnings) > 0 {
			return "SQL 审核通过：" + strings.Join(review.Warnings, "；")
		}
		return "SQL 审核通过。"
	}
	return "SQL 审核未通过：" + review.BlockedReason
}

func datasourceRouteObservation(data PromptContext) string {
	switch {
	case len(data.SelectedDatasources) > 1:
		return "当前项目数据源范围：" + datasourceNames(data.SelectedDatasources) + "。Agent 会根据问题选择目标数据源；如需跨源计算，将返回拆分执行计划而不是生成跨库 SQL。"
	case len(data.SelectedDatasources) == 1:
		ds := data.SelectedDatasources[0]
		return "当前项目数据源范围：" + ds.Name + "（" + ds.Dialect + "）。"
	case len(data.AvailableDatasources) > 1:
		return "项目存在多个可用数据源，将由 Agent 根据问题、元数据和业务知识选择目标数据源。"
	case len(data.AvailableDatasources) == 1:
		ds := data.AvailableDatasources[0]
		return "项目仅有一个可用数据源：" + ds.Name + "（" + ds.Dialect + "）。"
	default:
		return "当前请求未提供项目数据源列表；如果无法判断目标数据源，Agent 将要求澄清。"
	}
}

func metadataObservation(data PromptContext) string {
	parts := []string{
		fmt.Sprintf("数据源=%d", len(data.AvailableDatasources)),
		fmt.Sprintf("选中数据源=%d", len(data.SelectedDatasources)),
		fmt.Sprintf("业务术语=%d", len(data.BusinessTerms)),
		fmt.Sprintf("指标=%d", len(data.Metrics)),
		fmt.Sprintf("FewShot=%d", len(data.FewShots)),
	}
	return "项目上下文已装配：" + strings.Join(parts, "，") + "。"
}

func noSQLAgentResult(req AgentRequest, data PromptContext, generated generatedSQL) *AgentResult {
	intent := normalizeAgentIntent(generated.Intent)
	if generated.RequiresMultiDatasource {
		intent = AgentIntentMultiQuery
	}
	if generated.NeedClarification {
		intent = AgentIntentClarify
	}
	explanation := strings.TrimSpace(firstNonEmpty(generated.Explanation, generated.Answer))
	if explanation == "" {
		switch {
		case generated.RequiresMultiDatasource:
			explanation = "这个问题需要跨多个独立数据源拆分查询，当前没有生成单条可执行 SQL。"
		case generated.NeedClarification:
			explanation = "当前项目上下文不足以确定目标数据源、指标口径或表字段，需要进一步澄清。"
		default:
			explanation = "未生成可执行 SQL。"
		}
	}
	return &AgentResult{
		Question:                req.Question,
		Intent:                  intent,
		Answer:                  explanation,
		Explanation:             explanation,
		DatasourceID:            generated.DatasourceID,
		DatasourceIDs:           normalizeGeneratedDatasourceIDs(generated, data),
		Dialect:                 generatedDialect(generated, data),
		RequiresMultiDatasource: generated.RequiresMultiDatasource,
		NeedClarification:       generated.NeedClarification || !generated.RequiresMultiDatasource,
		Review: ReviewResult{
			Passed:        false,
			RiskLevel:     "none",
			NormalizedSQL: "",
			BlockedReason: "no executable sql generated",
			Limit:         data.MaxRows,
		},
	}
}

func normalizeGeneratedDatasourceIDs(generated generatedSQL, data PromptContext) []uint64 {
	ids := append([]uint64(nil), generated.DatasourceIDs...)
	if generated.DatasourceID > 0 {
		ids = append(ids, generated.DatasourceID)
	}
	for _, task := range generated.SQLTasks {
		if task.DatasourceID > 0 {
			ids = append(ids, task.DatasourceID)
		}
	}
	if len(ids) == 0 {
		ids = append(ids, data.SelectedDatasourceIDs...)
	}
	if len(ids) == 0 && len(data.SelectedDatasources) == 1 {
		ids = append(ids, data.SelectedDatasources[0].ID)
	}
	return uniqueUint64(ids)
}

func generatedDialect(generated generatedSQL, data PromptContext) string {
	if generated.Dialect != "" {
		return strings.ToLower(generated.Dialect)
	}
	if len(generated.SQLTasks) > 1 {
		return "mixed"
	}
	if len(data.SelectedDatasources) > 0 {
		return data.SelectedDatasources[0].Dialect
	}
	return data.DefaultDialect
}

func datasourceNames(datasources []AgentDatasource) string {
	names := make([]string, 0, len(datasources))
	for _, ds := range datasources {
		if ds.Name != "" {
			names = append(names, ds.Name)
			continue
		}
		names = append(names, fmt.Sprintf("%d", ds.ID))
	}
	return strings.Join(names, "、")
}

func floatPtr(value float64) *float64 {
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
