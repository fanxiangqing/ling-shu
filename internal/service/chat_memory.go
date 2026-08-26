package service

import (
	"strings"
	"time"

	"ling-shu/internal/memory"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
)

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
