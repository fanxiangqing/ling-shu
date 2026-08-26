package service

import (
	"context"
	"strings"
	"time"

	"ling-shu/internal/memory"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
)

func (s *ChatService) resolveUserMemoryOperation(ctx context.Context, input SendChatMessageInput, userMessageID uint64, operation memory.UserMemoryOperation, now time.Time, emit func(query.AgentEvent) error) (*query.AgentResult, error) {
	steps := make([]query.AgentEvent, 0, 3)
	steps = appendServiceAgentEvent(steps, query.EventThought, "memory.user.resolve", "正在处理跨会话用户记忆。", "", nil)
	if err := emitAgentEvent(emit, steps[len(steps)-1]); err != nil {
		return nil, err
	}
	result, err := s.userMemoryManager.ExecuteOperation(ctx, memory.UserScope{
		TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID,
	}, operation, input.SessionID, userMessageID, now)
	answer := result.Answer
	if err != nil {
		answer = "长期记忆没有更新：" + err.Error()
	}
	steps = appendServiceAgentEvent(steps, query.EventObservation, "memory.user.resolve", answer, "", nil)
	if emitErr := emitAgentEvent(emit, steps[len(steps)-1]); emitErr != nil {
		return nil, emitErr
	}
	agentResult := &query.AgentResult{
		Question: input.Content, Intent: query.AgentIntentChat, Answer: answer, Explanation: answer,
		Review: query.ReviewResult{Passed: true, RiskLevel: "none", Limit: input.MaxRows}, Steps: steps,
	}
	return agentResult, nil
}

func buildAgentUserMemories(prepared memory.UserMemoryContext) []query.AgentMemory {
	out := make([]query.AgentMemory, 0, len(prepared.PromptItems)+len(prepared.Episodes))
	for _, item := range prepared.PromptItems {
		out = append(out, query.AgentMemory{ID: item.ID, Kind: item.Kind, ScopeLevel: item.ScopeLevel, Content: item.Content})
	}
	for _, episode := range prepared.Episodes {
		out = append(out, query.AgentMemory{ID: episode.ID, Kind: "episode", ScopeLevel: memory.UserMemoryScopeProject, Content: episode.Summary})
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func applyUserMemoryPresentation(question string, prepared memory.UserMemoryContext, execution *QueryExecutionResult, executions []*QueryExecutionResult) {
	chartType := strings.TrimSpace(prepared.DefaultChart)
	if chartType == "" || questionExplicitlyRequestsChart(question) {
		return
	}
	all := executions
	if len(all) == 0 && execution != nil {
		all = []*QueryExecutionResult{execution}
	}
	for _, item := range all {
		if item == nil {
			continue
		}
		applyDefaultChartType(&item.Chart, chartType)
		if item.Execution != nil {
			item.Execution.ChartType = item.Chart.Type
		}
	}
}

func questionExplicitlyRequestsChart(question string) bool {
	lower := strings.ToLower(question)
	for _, value := range []string{"柱状图", "条形图", "折线图", "饼图", "表格", "bar chart", "line chart", "pie chart"} {
		if strings.Contains(lower, value) {
			return true
		}
	}
	return false
}

func applyDefaultChartType(chart *query.ChartSuggestion, chartType string) {
	if chart == nil {
		return
	}
	switch chartType {
	case query.ChartBar, query.ChartLine:
		if chart.XField == "" && chart.NameField != "" {
			chart.XField = chart.NameField
		}
		if len(chart.YFields) == 0 && chart.ValueField != "" {
			chart.YFields = []string{chart.ValueField}
		}
		if chart.XField != "" && len(chart.YFields) > 0 {
			chart.Type = chartType
		}
	case query.ChartPie:
		if chart.NameField == "" {
			chart.NameField = chart.XField
		}
		if chart.ValueField == "" && len(chart.YFields) == 1 {
			chart.ValueField = chart.YFields[0]
		}
		if chart.NameField != "" && chart.ValueField != "" {
			chart.Type = query.ChartPie
		}
	case query.ChartTable:
		chart.Type = query.ChartTable
	}
}

func (s *ChatService) resolveMemoryFollowUp(input SendChatMessageInput, question string, resolution memory.Resolution, emit func(query.AgentEvent) error) (*query.AgentResult, *QueryExecutionResult, error) {
	steps := make([]query.AgentEvent, 0, 3)
	steps = appendServiceAgentEvent(steps, query.EventThought, "memory.resolve", "正在解析这次追问与最近查询结果的关系。", "", nil)
	if err := emitAgentEvent(emit, steps[len(steps)-1]); err != nil {
		return nil, nil, err
	}

	if resolution.Action == memory.ActionClarify {
		steps = appendServiceAgentEvent(steps, query.EventObservation, "memory.resolve", resolution.Reason, "", nil)
		if err := emitAgentEvent(emit, steps[len(steps)-1]); err != nil {
			return nil, nil, err
		}
		result := &query.AgentResult{
			Question:          question,
			Intent:            query.AgentIntentClarify,
			Answer:            resolution.Answer,
			Explanation:       resolution.Answer,
			NeedClarification: true,
			Review: query.ReviewResult{
				Passed:        false,
				RiskLevel:     "none",
				BlockedReason: "memory target clarification required",
				Limit:         input.MaxRows,
			},
			Steps: steps,
		}
		return result, nil, nil
	}

	if resolution.Artifact == nil {
		return nil, nil, ErrInvalidInput
	}
	artifact := resolution.Artifact
	steps = appendServiceAgentEvent(steps, query.EventAction, "artifact.transform", "复用最近查询结果并转换可视化表达。", "", nil)
	if err := emitAgentEvent(emit, steps[len(steps)-1]); err != nil {
		return nil, nil, err
	}
	steps = appendServiceAgentEvent(steps, query.EventObservation, "artifact.transform", resolution.Reason, "", nil)
	if err := emitAgentEvent(emit, steps[len(steps)-1]); err != nil {
		return nil, nil, err
	}

	chart := artifact.Payload.Chart
	if strings.TrimSpace(chart.Title) == "" {
		chart.Title = strings.TrimSpace(artifact.Purpose)
	}
	rowCount := len(artifact.Payload.Rows)
	datasourceID := firstArtifactDatasourceID(*artifact)
	review := query.ReviewResult{
		Passed:        true,
		RiskLevel:     "none",
		NormalizedSQL: "",
		Limit:         input.MaxRows,
	}
	execution := &QueryExecutionResult{
		Execution: &model.QueryExecution{
			TenantID:     input.TenantID,
			ProjectID:    input.ProjectID,
			DatasourceID: datasourceID,
			SessionID:    input.SessionID,
			UserID:       input.UserID,
			Question:     question,
			Status:       "success",
			RowCount:     &rowCount,
			ChartType:    chart.Type,
			CreatedAt:    time.Now(),
		},
		Review:        review,
		Chart:         chart,
		Answer:        resolution.Answer,
		SpeechSummary: resolution.Answer,
		Columns:       append([]string(nil), artifact.Payload.Columns...),
		Rows:          copyRowsForMemory(artifact.Payload.Rows),
	}
	result := &query.AgentResult{
		Question:     question,
		Intent:       query.AgentIntentTransform,
		Answer:       resolution.Answer,
		Explanation:  resolution.Answer,
		DatasourceID: datasourceID,
		Review:       review,
		Steps:        steps,
	}
	if datasourceID > 0 {
		result.DatasourceIDs = []uint64{datasourceID}
	}
	if err := emitExecutionResultEvent(emit, execution, nil, nextAgentStep(steps)); err != nil {
		return nil, nil, err
	}
	return result, execution, nil
}

func buildMemoryTurnInput(input SendChatMessageInput, assistantMessageID uint64, agentResult *query.AgentResult, execution *QueryExecutionResult, executions []*QueryExecutionResult) memory.TurnInput {
	turn := memory.TurnInput{
		Scope: memory.Scope{
			TenantID:  input.TenantID,
			ProjectID: input.ProjectID,
			SessionID: input.SessionID,
			UserID:    input.UserID,
		},
		SourceMessageID: assistantMessageID,
		Question:        strings.TrimSpace(input.Content),
	}
	if agentResult != nil {
		turn.Answer = firstNonEmptyService(agentResult.Answer, agentResult.Explanation)
		turn.Intent = agentResult.Intent
	}
	results := executions
	if len(results) == 0 && execution != nil {
		results = []*QueryExecutionResult{execution}
	}
	turn.Executions = make([]memory.ExecutionSnapshot, 0, len(results))
	for index, result := range results {
		if result == nil {
			continue
		}
		snapshot := memory.ExecutionSnapshot{
			Purpose: firstNonEmptyService(
				result.Chart.Title,
				agentTaskPurpose(agentResult, index),
				executionQuestion(result),
				turn.Question,
				turn.Answer,
			),
			Columns: append([]string(nil), result.Columns...),
			Rows:    copyRowsForMemory(result.Rows),
			Chart:   result.Chart,
			Answer:  result.Answer,
			Limit:   result.Review.Limit,
		}
		if result.Execution != nil {
			snapshot.QueryExecutionID = result.Execution.ID
			snapshot.DatasourceID = result.Execution.DatasourceID
			snapshot.SQL = firstNonEmptyService(result.Execution.FinalSQL, result.Execution.GeneratedSQL)
			snapshot.Status = result.Execution.Status
			if result.Execution.RowCount != nil {
				snapshot.RowCount = *result.Execution.RowCount
			}
		}
		turn.Executions = append(turn.Executions, snapshot)
	}
	return turn
}

func buildUserMemoryTurnInput(input SendChatMessageInput, userMessageID uint64, assistantMessageID uint64, agentResult *query.AgentResult, execution *QueryExecutionResult, executions []*QueryExecutionResult, occurredAt time.Time) memory.UserTurnInput {
	turn := memory.UserTurnInput{
		Scope:     memory.UserScope{TenantID: input.TenantID, ProjectID: input.ProjectID, UserID: input.UserID},
		SessionID: input.SessionID, UserMessageID: userMessageID, AssistantMessageID: assistantMessageID,
		Question: strings.TrimSpace(input.Content), Timezone: input.TimeContext.Timezone, OccurredAt: occurredAt,
	}
	if agentResult != nil {
		turn.Answer = firstNonEmptyService(agentResult.Answer, agentResult.Explanation)
		turn.Intent = agentResult.Intent
	}
	results := executions
	if len(results) == 0 && execution != nil {
		results = []*QueryExecutionResult{execution}
	}
	seen := map[uint64]bool{}
	for _, result := range results {
		if result == nil || result.Execution == nil || result.Execution.ID == 0 || seen[result.Execution.ID] {
			continue
		}
		seen[result.Execution.ID] = true
		turn.QueryExecutionIDs = append(turn.QueryExecutionIDs, result.Execution.ID)
	}
	return turn
}

func executionQuestion(result *QueryExecutionResult) string {
	if result == nil || result.Execution == nil {
		return ""
	}
	return strings.TrimSpace(result.Execution.Question)
}

func agentTaskPurpose(result *query.AgentResult, index int) string {
	if result == nil || index < 0 || index >= len(result.SQLTasks) {
		return ""
	}
	return strings.TrimSpace(result.SQLTasks[index].Purpose)
}

func firstArtifactDatasourceID(artifact memory.Artifact) uint64 {
	if len(artifact.Lineage.DatasourceIDs) == 0 {
		return 0
	}
	return artifact.Lineage.DatasourceIDs[0]
}

func copyRowsForMemory(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		copied := make(map[string]any, len(row))
		for key, value := range row {
			copied[key] = value
		}
		out = append(out, copied)
	}
	return out
}
