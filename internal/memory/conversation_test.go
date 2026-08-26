package memory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"ling-shu/internal/model"
	"ling-shu/internal/query"
)

func TestBuildConversationCompactsAgentResult(t *testing.T) {
	payload, err := json.Marshal(savedAssistantPayload{
		Agent: &query.AgentResult{
			Question: "项目有多少风险点",
			Answer:   "当前共有 17 个风险点。",
			SQLTasks: []query.AgentSQLTask{{Purpose: "统计风险点总数"}},
		},
		Execution: &savedExecutionResult{
			Columns: []string{"risk_point_count"},
			Rows:    []map[string]any{{"risk_point_count": 17}},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	messages := []model.ChatMessage{
		{Role: "user", Content: "项目有多少风险点", ContentType: "text"},
		{Role: "assistant", Content: string(payload), ContentType: "agent_result"},
	}

	conversation := BuildConversation(messages)
	if len(conversation) != 2 {
		t.Fatalf("expected two messages, got %+v", conversation)
	}
	content := conversation[1].Content
	if !strings.Contains(content, "当前共有 17 个风险点") || !strings.Contains(content, "risk_point_count") {
		t.Fatalf("expected compact answer and result schema, got %q", content)
	}
	if strings.Contains(content, `\"execution\"`) || strings.Contains(content, `\"review\"`) {
		t.Fatalf("expected agent payload to be compacted, got %q", content)
	}
}

func TestExtractArtifactsFromMessagesPrefersRawExecutions(t *testing.T) {
	rowCount := 2
	payload, err := json.Marshal(savedAssistantPayload{
		Agent: &query.AgentResult{
			Question: "项目情况",
			SQLTasks: []query.AgentSQLTask{
				{Purpose: "查询当前环号"},
				{Purpose: "查询风险列表"},
			},
		},
		Execution: &savedExecutionResult{
			Columns: []string{"数据源", "数值"},
			Rows: []map[string]any{
				{"数据源": "项目库", "数值": 1254},
				{"数据源": "项目库", "数值": 17},
			},
		},
		Executions: []*savedExecutionResult{
			{
				Execution: &model.QueryExecution{ID: 10, Status: "success", DatasourceID: 8, RowCount: &rowCount, FinalSQL: "select current_ring from project limit 200"},
				Columns:   []string{"current_ring"},
				Rows:      []map[string]any{{"current_ring": 1254}},
			},
			{
				Execution: &model.QueryExecution{ID: 11, Status: "success", DatasourceID: 8, RowCount: &rowCount, FinalSQL: "select risk_name from risks limit 200"},
				Columns:   []string{"risk_name"},
				Rows:      []map[string]any{{"risk_name": "联络通道"}, {"risk_name": "厂房"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	scope := Scope{TenantID: 1, ProjectID: 2, SessionID: 3, UserID: 4}
	artifacts := ExtractArtifactsFromMessages(scope, []model.ChatMessage{{
		ID:          9,
		Role:        "assistant",
		Content:     string(payload),
		ContentType: "agent_result",
		CreatedAt:   time.Now(),
	}})
	if len(artifacts) != 2 {
		t.Fatalf("expected two raw artifacts and no combined artifact, got %+v", artifacts)
	}
	if artifacts[0].Payload.Columns[0] != "current_ring" || artifacts[1].Payload.Columns[0] != "risk_name" {
		t.Fatalf("unexpected artifacts: %+v", artifacts)
	}
}
