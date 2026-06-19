package query

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ling-shu/internal/llm"

	"go.uber.org/zap"
)

func TestReactAgentRun(t *testing.T) {
	agent := NewReactAgent(fakeLLMProvider{
		streamContent: `{"thought":"需要查询订单表","datasource_id":10,"datasource_ids":[10],"dialect":"mysql","sql":"select count(*) as total from orders","explanation":"统计订单总数"}`,
	}, NewSQLReviewer(200, 1000), mustDefaultPromptRenderer(t), zap.NewNop())

	result, err := agent.Run(context.Background(), AgentRequest{
		TenantID:     1,
		ProjectID:    1,
		DatasourceID: 10,
		Question:     "订单数是多少",
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if !result.Review.Passed {
		t.Fatalf("expected review passed: %s", result.Review.BlockedReason)
	}
	if result.SQL != "select count(*) as total from orders LIMIT 200" {
		t.Fatalf("unexpected sql: %s", result.SQL)
	}
	if result.DatasourceID != 10 {
		t.Fatalf("expected datasource id 10, got %d", result.DatasourceID)
	}
	if len(result.Steps) == 0 {
		t.Fatal("expected agent steps")
	}
	if _, err := json.Marshal(result); err != nil {
		t.Fatalf("agent result should be json serializable: %v", err)
	}
	for _, step := range result.Steps {
		if step.Final != nil {
			t.Fatal("aggregated agent steps must not keep final result back references")
		}
	}
}

func TestReactAgentAnswersPlainConversationWithoutSQLTools(t *testing.T) {
	agent := NewReactAgent(fakeLLMProvider{
		streamContent: `{"intent":"query","sql":"select 1"}`,
	}, NewSQLReviewer(200, 1000), mustDefaultPromptRenderer(t), zap.NewNop())

	result, err := agent.Run(context.Background(), AgentRequest{
		TenantID:  1,
		ProjectID: 1,
		Question:  "你好",
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result.Intent != AgentIntentChat {
		t.Fatalf("expected chat intent, got %s", result.Intent)
	}
	if result.SQL != "" || len(result.SQLTasks) != 0 {
		t.Fatalf("plain conversation should not generate sql: sql=%s tasks=%+v", result.SQL, result.SQLTasks)
	}
	if !result.Review.Passed {
		t.Fatalf("expected chat review passed")
	}
	for _, step := range result.Steps {
		if step.Name == "datasource.route" || step.Name == "llm.text2sql" {
			t.Fatalf("plain conversation should not use sql tools, got step %+v", step)
		}
	}
}

func TestReactAgentReturnsNoSQLForMultiDatasource(t *testing.T) {
	agent := NewReactAgent(fakeLLMProvider{
		streamContent: `{"thought":"需要订单库和CRM库拆分查询","datasource_ids":[1,2],"dialect":"mixed","requires_multi_datasource":true,"sql":"","explanation":"这个问题需要跨订单和CRM两个数据源拆分执行后再合并分析"}`,
	}, NewSQLReviewer(200, 1000), mustDefaultPromptRenderer(t), zap.NewNop())

	result, err := agent.Run(context.Background(), AgentRequest{
		TenantID:    1,
		ProjectID:   1,
		Question:    "线索到订单转化率是多少",
		Datasources: []AgentDatasource{{ID: 1, Name: "orders", Type: "mysql"}, {ID: 2, Name: "crm", Type: "postgresql"}},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result.SQL != "" {
		t.Fatalf("expected empty sql, got %s", result.SQL)
	}
	if !result.RequiresMultiDatasource {
		t.Fatal("expected multi datasource result")
	}
	if result.NeedClarification {
		t.Fatal("multi datasource plan should not be marked as clarification")
	}
	if len(result.DatasourceIDs) != 2 {
		t.Fatalf("expected two datasource ids, got %v", result.DatasourceIDs)
	}
}

func TestReactAgentBuildsSQLTasksForMultiDatasource(t *testing.T) {
	index := 0
	agent := NewReactAgent(fakeLLMProvider{
		streamContents: []string{
			`{"intent":"multi_query","thought":"需要比较两个数据源","datasource_ids":[1,2],"requires_multi_datasource":true}`,
			`{"intent":"multi_query","thought":"分别统计两个数据源用户数","datasource_ids":[1,2],"dialect":"mixed","requires_multi_datasource":true,"sql_tasks":[{"datasource_id":1,"datasource_name":"ling_shu","dialect":"mysql","purpose":"统计灵数用户数","sql":"select count(*) as user_count from users"},{"datasource_id":2,"datasource_name":"survey","dialect":"mysql","purpose":"统计问卷用户数","sql":"select count(*) as user_count from users"}],"explanation":"分别查询两个数据源后对比"}`,
		},
		streamIndex: &index,
	}, NewSQLReviewer(200, 1000), mustDefaultPromptRenderer(t), zap.NewNop())

	result, err := agent.Run(context.Background(), AgentRequest{
		TenantID:    1,
		ProjectID:   1,
		Question:    "对比灵数和问卷服务用户数",
		Datasources: []AgentDatasource{{ID: 1, Name: "ling_shu", Type: "mysql"}, {ID: 2, Name: "survey", Type: "mysql"}},
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result.Intent != AgentIntentMultiQuery || !result.RequiresMultiDatasource {
		t.Fatalf("expected multi query result, got intent=%s multi=%v", result.Intent, result.RequiresMultiDatasource)
	}
	if len(result.SQLTasks) != 2 {
		t.Fatalf("expected two sql tasks, got %+v", result.SQLTasks)
	}
	for _, task := range result.SQLTasks {
		if !task.Review.Passed {
			t.Fatalf("expected task review passed: %+v", task)
		}
		if !strings.Contains(task.SQL, "LIMIT 200") {
			t.Fatalf("expected normalized task sql with limit, got %s", task.SQL)
		}
	}
}

func TestReactAgentUsesResolvedProjectLLMProvider(t *testing.T) {
	agent := NewReactAgent(fakeLLMProvider{
		streamContent: `{"sql":"select 1","explanation":"wrong provider"}`,
	}, NewSQLReviewer(200, 1000), mustDefaultPromptRenderer(t), zap.NewNop())
	agent.SetLLMProviderResolver(func(ctx context.Context, req AgentRequest) (llm.Provider, error) {
		if req.TenantID != 1 || req.ProjectID != 2 {
			t.Fatalf("unexpected scope: tenant=%d project=%d", req.TenantID, req.ProjectID)
		}
		return fakeLLMProvider{
			streamContent: `{"thought":"使用项目级模型","datasource_id":9,"dialect":"mysql","sql":"select count(*) as total from users","explanation":"统计用户数"}`,
		}, nil
	})

	result, err := agent.Run(context.Background(), AgentRequest{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 9,
		Question:     "用户数是多少",
	})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result.SQL != "select count(*) as total from users LIMIT 200" {
		t.Fatalf("unexpected sql: %s", result.SQL)
	}
}

func TestReactAgentSynthesizesMultiDatasourceResults(t *testing.T) {
	agent := NewReactAgent(fakeLLMProvider{
		streamContent: `灵数用户数为 2080，问卷用户数为 1200，建议使用柱状图对比。`,
	}, NewSQLReviewer(200, 1000), mustDefaultPromptRenderer(t), zap.NewNop())

	answer, err := agent.SynthesizeResults(context.Background(), AgentResultSynthesisRequest{
		AgentRequest: AgentRequest{
			TenantID:    1,
			ProjectID:   2,
			Question:    "对比两个系统用户数",
			Datasources: []AgentDatasource{{ID: 7, Name: "灵数数据库", Type: "mysql"}, {ID: 8, Name: "问卷数据库", Type: "mysql"}},
		},
		SQLTasks: []AgentSQLTask{
			{DatasourceID: 7, DatasourceName: "灵数数据库", Purpose: "统计灵数用户数", SQL: "select count(*) as user_count from users"},
			{DatasourceID: 8, DatasourceName: "问卷数据库", Purpose: "统计问卷用户数", SQL: "select count(*) as user_count from users"},
		},
		ExecutionResults: []AgentExecutionSummary{
			{DatasourceID: 7, DatasourceName: "灵数数据库", Purpose: "统计灵数用户数", Columns: []string{"user_count"}, Rows: []map[string]any{{"user_count": 2080}}, RowCount: 1, ChartType: ChartTable},
			{DatasourceID: 8, DatasourceName: "问卷数据库", Purpose: "统计问卷用户数", Columns: []string{"user_count"}, Rows: []map[string]any{{"user_count": 1200}}, RowCount: 1, ChartType: ChartTable},
		},
	})
	if err != nil {
		t.Fatalf("synthesize results: %v", err)
	}
	if !strings.Contains(answer, "2080") || !strings.Contains(answer, "1200") {
		t.Fatalf("expected synthesized answer, got %s", answer)
	}
}

func mustDefaultPromptRenderer(t *testing.T) *PromptRenderer {
	t.Helper()
	renderer, err := NewPromptRendererFromTemplates(testPromptTemplates("tenant={{.TenantID}}, project={{.ProjectID}}, selected=[{{joinUint64 .SelectedDatasourceIDs \",\"}}], max={{.MaxRows}}"))
	if err != nil {
		t.Fatalf("new prompt renderer: %v", err)
	}
	return renderer
}

type fakeLLMProvider struct {
	streamContent  string
	streamContents []string
	streamIndex    *int
}

func (p fakeLLMProvider) Name() string                  { return "aliyun" }
func (p fakeLLMProvider) Configured() bool              { return true }
func (p fakeLLMProvider) DefaultChatModel() string      { return "qwen-plus" }
func (p fakeLLMProvider) DefaultEmbeddingModel() string { return "text-embedding-v4" }
func (p fakeLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Content: p.content()}, nil
}
func (p fakeLLMProvider) StreamChat(ctx context.Context, req llm.ChatRequest, onEvent func(llm.ChatStreamEvent) error) error {
	for _, part := range strings.Split(p.content(), " ") {
		if err := onEvent(llm.ChatStreamEvent{Delta: part + " "}); err != nil {
			return err
		}
	}
	return onEvent(llm.ChatStreamEvent{Done: true})
}
func (p fakeLLMProvider) Embeddings(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return &llm.EmbeddingResponse{}, nil
}

func (p fakeLLMProvider) content() string {
	if len(p.streamContents) == 0 {
		return p.streamContent
	}
	index := 0
	if p.streamIndex != nil {
		index = *p.streamIndex
		(*p.streamIndex)++
	}
	if index >= len(p.streamContents) {
		index = len(p.streamContents) - 1
	}
	return p.streamContents[index]
}
