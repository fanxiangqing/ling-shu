package memory

import (
	"encoding/json"
	"fmt"
	"strings"

	"ling-shu/internal/model"
	"ling-shu/internal/query"
)

type savedAssistantPayload struct {
	Agent      *query.AgentResult      `json:"agent"`
	Execution  *savedExecutionResult   `json:"execution"`
	Executions []*savedExecutionResult `json:"executions"`
}

type savedExecutionResult struct {
	Execution *model.QueryExecution `json:"execution"`
	Review    query.ReviewResult    `json:"review"`
	Chart     query.ChartSuggestion `json:"chart"`
	Answer    string                `json:"answer"`
	Columns   []string              `json:"columns"`
	Rows      []map[string]any      `json:"rows"`
}

func BuildConversation(messages []model.ChatMessage) []query.AgentMessage {
	conversation := make([]query.AgentMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if role == "assistant" && message.ContentType == "agent_result" {
			content = compactAssistantContent(content)
		}
		if content == "" {
			continue
		}
		conversation = append(conversation, query.AgentMessage{Role: role, Content: content})
	}
	return conversation
}

func ExtractArtifactsFromMessages(scope Scope, messages []model.ChatMessage) []Artifact {
	if !scope.Valid() {
		return nil
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role != "assistant" || message.ContentType != "agent_result" {
			continue
		}
		payload, ok := decodeAssistantPayload(message.Content)
		if !ok {
			continue
		}
		executions := payload.Executions
		if len(executions) == 0 && payload.Execution != nil {
			executions = []*savedExecutionResult{payload.Execution}
		}
		snapshots := make([]ExecutionSnapshot, 0, len(executions))
		for executionIndex, execution := range executions {
			if execution == nil {
				continue
			}
			purpose := "最近查询结果"
			if payload.Agent != nil {
				purpose = firstNonEmpty(taskPurpose(payload.Agent.SQLTasks, executionIndex), payload.Agent.Question, payload.Agent.Explanation, payload.Agent.Answer, purpose)
			}
			snapshot := ExecutionSnapshot{
				Purpose: purpose,
				Limit:   execution.Review.Limit,
				Chart:   execution.Chart,
				Answer:  execution.Answer,
				Columns: append([]string(nil), execution.Columns...),
				Rows:    copyRows(execution.Rows),
			}
			if execution.Execution != nil {
				snapshot.QueryExecutionID = execution.Execution.ID
				snapshot.DatasourceID = execution.Execution.DatasourceID
				snapshot.SQL = firstNonEmpty(execution.Execution.FinalSQL, execution.Execution.GeneratedSQL)
				snapshot.Status = execution.Execution.Status
				if execution.Execution.RowCount != nil {
					snapshot.RowCount = *execution.Execution.RowCount
				}
			}
			snapshots = append(snapshots, snapshot)
		}
		answer := ""
		intent := ""
		question := ""
		if payload.Agent != nil {
			answer = firstNonEmpty(payload.Agent.Answer, payload.Agent.Explanation)
			intent = payload.Agent.Intent
			question = payload.Agent.Question
		}
		artifacts := ExtractArtifacts(TurnInput{
			Scope:           scope,
			SourceMessageID: message.ID,
			Question:        question,
			Answer:          answer,
			Intent:          intent,
			Executions:      snapshots,
		})
		for artifactIndex := range artifacts {
			artifacts[artifactIndex].CreatedAt = message.CreatedAt
		}
		if len(artifacts) > 0 {
			return artifacts
		}
	}
	return nil
}

func compactAssistantContent(content string) string {
	payload, ok := decodeAssistantPayload(content)
	if !ok {
		return content
	}
	parts := make([]string, 0, 3)
	if payload.Agent != nil {
		if answer := firstNonEmpty(payload.Agent.Answer, payload.Agent.Explanation); answer != "" {
			parts = append(parts, "回答："+answer)
		}
	}
	executions := payload.Executions
	if len(executions) == 0 && payload.Execution != nil {
		executions = []*savedExecutionResult{payload.Execution}
	}
	for index, execution := range executions {
		if execution == nil || len(execution.Columns) == 0 {
			continue
		}
		purpose := fmt.Sprintf("结果%d", index+1)
		if payload.Agent != nil {
			purpose = firstNonEmpty(taskPurpose(payload.Agent.SQLTasks, index), purpose)
		}
		parts = append(parts, fmt.Sprintf("%s：%d 行，字段为 %s", purpose, len(execution.Rows), strings.Join(execution.Columns, "、")))
	}
	return strings.Join(parts, "；")
}

func decodeAssistantPayload(content string) (savedAssistantPayload, bool) {
	var payload savedAssistantPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return savedAssistantPayload{}, false
	}
	if payload.Agent == nil && payload.Execution == nil && len(payload.Executions) == 0 {
		return savedAssistantPayload{}, false
	}
	return payload, true
}

func taskPurpose(tasks []query.AgentSQLTask, index int) string {
	if index < 0 || index >= len(tasks) {
		return ""
	}
	return strings.TrimSpace(tasks[index].Purpose)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
