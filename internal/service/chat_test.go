package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/model"
	"ling-shu/internal/query"
	"ling-shu/internal/rag"
	"ling-shu/internal/repository"
)

func TestChatServiceSendMessageStoresUserAndAssistantMessages(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Title:     "销售问数",
			Status:    "active",
		},
		recentMessages: []model.ChatMessage{
			{Role: "user", Content: "昨天销售额是多少", ContentType: "text"},
			{Role: "assistant", Content: "昨天销售额是 100", ContentType: "text"},
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "今天销售额是多少",
			SQL:          "select sum(amount) from orders LIMIT 200",
			Explanation:  "统计订单金额",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select sum(amount) from orders LIMIT 200"},
		},
	}
	service := NewChatService(chatRepo, agent, nil)
	auditRecorder := &recordingAuditRecorder{}
	service.SetAuditRecorder(auditRecorder)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "今天销售额是多少",
		DatasourceID: 7,
		RequestID:    "rid-chat",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.UserMessage.Role != "user" || result.UserMessage.Content != "今天销售额是多少" {
		t.Fatalf("unexpected user message: %+v", result.UserMessage)
	}
	if result.AssistantMessage.Role != "assistant" || result.AssistantMessage.ContentType != "agent_result" {
		t.Fatalf("unexpected assistant message: %+v", result.AssistantMessage)
	}
	if len(chatRepo.messages) != 2 {
		t.Fatalf("expected two created messages, got %d", len(chatRepo.messages))
	}
	if len(agent.lastInput.Conversation) != 3 {
		t.Fatalf("expected previous messages plus current user question, got %+v", agent.lastInput.Conversation)
	}
	if agent.lastInput.Conversation[2].Content != "今天销售额是多少" {
		t.Fatalf("expected current question in conversation, got %+v", agent.lastInput.Conversation)
	}
	if len(auditRecorder.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(auditRecorder.events))
	}
	if auditRecorder.events[0].EventType != auditpkg.EventChatMessage || auditRecorder.events[0].ResourceID != 10 || auditRecorder.events[0].RequestID != "rid-chat" {
		t.Fatalf("unexpected audit event: %+v", auditRecorder.events[0])
	}
}

func TestChatServiceStreamMessageEmitsStepsAndStoresResult(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "订单数",
			SQL:          "select count(*) from orders LIMIT 200",
			Explanation:  "统计订单数",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from orders LIMIT 200"},
			Steps: []query.AgentEvent{
				{Type: query.EventThought, Step: 1, Name: "intent.plan", Content: "判断用户任务类型。"},
				{Type: query.EventAction, Step: 2, Name: "llm.text2sql", Content: "生成候选 SQL。"},
			},
		},
	}
	var emitted []query.AgentEvent
	service := NewChatService(chatRepo, agent, nil)

	result, err := service.StreamMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "订单数",
		DatasourceID: 7,
	}, func(event query.AgentEvent) error {
		emitted = append(emitted, event)
		return nil
	})
	if err != nil {
		t.Fatalf("stream message: %v", err)
	}
	if len(emitted) == 0 {
		t.Fatal("expected streamed agent events")
	}
	if result.AssistantMessage == nil || result.AssistantMessage.ContentType != "agent_result" {
		t.Fatalf("expected persisted assistant result, got %+v", result.AssistantMessage)
	}
	if len(chatRepo.messages) != 2 {
		t.Fatalf("expected user and assistant messages, got %+v", chatRepo.messages)
	}
}

func TestChatServiceSendMessageAutoExecutesReviewedSQL(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "订单数",
			SQL:          "select count(*) from orders LIMIT 200",
			Explanation:  "统计订单数",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from orders LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 99, Status: "success"},
			Rows:      []map[string]any{{"count": int64(1)}},
		},
	}
	service := NewChatService(chatRepo, agent, executor)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "订单数",
		DatasourceID: 7,
		AutoExecute:  true,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if executor.lastInput.SQL != "select count(*) from orders LIMIT 200" {
		t.Fatalf("unexpected executed sql: %s", executor.lastInput.SQL)
	}
	if result.AssistantMessage.QueryExecutionID != 99 {
		t.Fatalf("expected assistant message to reference execution 99, got %d", result.AssistantMessage.QueryExecutionID)
	}
}

func TestChatServiceAutoExecuteAnswerUsesExecutionResult(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "当前项目有多少用户",
			SQL:          "select count(*) as user_count from users LIMIT 200",
			Answer:       "当前项目中活跃成员用户数为：{user_count}。",
			Explanation:  "当前项目中活跃成员用户数为：{user_count}。",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution:     &model.QueryExecution{ID: 99, Status: "success"},
			Answer:        "用户数是 1。",
			SpeechSummary: "用户数是 1。",
			Columns:       []string{"user_count"},
			Rows:          []map[string]any{{"user_count": int64(1)}},
		},
	}
	service := NewChatService(chatRepo, agent, executor)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "当前项目有多少用户",
		DatasourceID: 7,
		AutoExecute:  true,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if result.Agent.Answer != "用户数是 1。" || strings.Contains(result.Agent.Explanation, "{user_count}") {
		t.Fatalf("expected execution answer to replace placeholder, got answer=%q explanation=%q", result.Agent.Answer, result.Agent.Explanation)
	}
}

func TestChatServiceAutoExecuteSynthesizesSingleDatasourceResult(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		synthesisAnswer: "当前项目用户数为 1。",
		result: &query.AgentResult{
			Question:     "当前项目有多少用户",
			SQL:          "select count(*) as user_count from users LIMIT 200",
			Explanation:  "统计当前项目用户数",
			DatasourceID: 7,
			Dialect:      "mysql",
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 99, Status: "success", DatasourceID: 7},
			Answer:    "用户数是 1。",
			Columns:   []string{"user_count"},
			Rows:      []map[string]any{{"user_count": int64(1)}},
		},
	}
	service := NewChatService(chatRepo, agent, executor)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "当前项目有多少用户",
		DatasourceID: 7,
		AutoExecute:  true,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if agent.synthesisCalls != 1 {
		t.Fatalf("expected one synthesis call, got %d", agent.synthesisCalls)
	}
	if len(agent.synthesisInput.Tasks) != 1 || len(agent.synthesisInput.Executions) != 1 {
		t.Fatalf("expected single execution synthesis context, got %+v", agent.synthesisInput)
	}
	if result.Agent.Answer != agent.synthesisAnswer || result.Execution.Answer != agent.synthesisAnswer {
		t.Fatalf("expected synthesized answer on agent and execution, got agent=%q execution=%q", result.Agent.Answer, result.Execution.Answer)
	}
	if !hasAgentEvent(result.Agent.Steps, query.EventAction, "result.synthesize") || !hasAgentEvent(result.Agent.Steps, query.EventObservation, "result.synthesize") {
		t.Fatalf("expected result synthesis action and observation steps, got %+v", result.Agent.Steps)
	}
}

func TestChatServiceSendMessageExecutesMultiDatasourceTasks(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:                "对比两个系统用户数",
			Intent:                  query.AgentIntentMultiQuery,
			Explanation:             "分别查询两个数据源后对比",
			RequiresMultiDatasource: true,
			DatasourceIDs:           []uint64{7, 8},
			Review:                  query.ReviewResult{Passed: true, RiskLevel: "low"},
			SQLTasks: []query.AgentSQLTask{
				{
					DatasourceID:   7,
					DatasourceName: "灵数数据库",
					Purpose:        "统计灵数用户数",
					SQL:            "select count(*) as user_count from users LIMIT 200",
					Review:         query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
				},
				{
					DatasourceID:   8,
					DatasourceName: "问卷数据库",
					Purpose:        "统计问卷用户数",
					SQL:            "select count(*) as user_count from users LIMIT 200",
					Review:         query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
				},
			},
		},
	}
	executor := &chatFakeQueryExecutor{
		results: []*QueryExecutionResult{
			{
				Execution: &model.QueryExecution{ID: 101, Status: "success"},
				Columns:   []string{"user_count"},
				Rows:      []map[string]any{{"user_count": int64(2080)}},
			},
			{
				Execution: &model.QueryExecution{ID: 102, Status: "success"},
				Columns:   []string{"user_count"},
				Rows:      []map[string]any{{"user_count": int64(1200)}},
			},
		},
	}
	service := NewChatService(chatRepo, agent, executor)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:              1,
		ProjectID:             2,
		SessionID:             10,
		UserID:                3,
		Content:               "对比两个系统用户数",
		SelectedDatasourceIDs: []uint64{7, 8},
		AutoExecute:           true,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if executor.calls != 2 {
		t.Fatalf("expected two executions, got %d", executor.calls)
	}
	if executor.inputs[0].DatasourceID != 7 || executor.inputs[1].DatasourceID != 8 {
		t.Fatalf("unexpected datasource execution order: %+v", executor.inputs)
	}
	if len(result.Executions) != 2 {
		t.Fatalf("expected two execution results, got %+v", result.Executions)
	}
	if result.Execution == nil || result.Execution.Chart.Type != query.ChartPie {
		t.Fatalf("expected combined distribution chart execution, got %+v", result.Execution)
	}
	if len(result.Execution.Rows) != 2 || result.Execution.Columns[0] != "数据源" || result.Execution.Columns[1] != "用户数" {
		t.Fatalf("unexpected combined chart rows: columns=%+v rows=%+v", result.Execution.Columns, result.Execution.Rows)
	}
	if !strings.Contains(result.Agent.Answer, "已完成跨数据源查询") || !strings.Contains(result.Agent.Answer, "差异") {
		t.Fatalf("expected cross datasource summary, got %s", result.Agent.Answer)
	}
}

func TestChatServiceSendMessageSynthesizesMultiDatasourceResults(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		synthesisAnswer: "灵数数据库用户数为 2080，问卷数据库用户数为 1200，灵数数据库多 880，建议使用柱状图对比。",
		result: &query.AgentResult{
			Question:                "对比两个系统用户数",
			Intent:                  query.AgentIntentMultiQuery,
			Explanation:             "分别查询两个数据源后对比",
			RequiresMultiDatasource: true,
			DatasourceIDs:           []uint64{7, 8},
			Review:                  query.ReviewResult{Passed: true, RiskLevel: "low"},
			SQLTasks: []query.AgentSQLTask{
				{
					DatasourceID:   7,
					DatasourceName: "灵数数据库",
					Purpose:        "统计灵数用户数",
					SQL:            "select count(*) as user_count from users LIMIT 200",
					Review:         query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
				},
				{
					DatasourceID:   8,
					DatasourceName: "问卷数据库",
					Purpose:        "统计问卷用户数",
					SQL:            "select count(*) as user_count from users LIMIT 200",
					Review:         query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
				},
			},
		},
	}
	executor := &chatFakeQueryExecutor{
		results: []*QueryExecutionResult{
			{Execution: &model.QueryExecution{ID: 101, Status: "success"}, Columns: []string{"user_count"}, Rows: []map[string]any{{"user_count": int64(2080)}}},
			{Execution: &model.QueryExecution{ID: 102, Status: "success"}, Columns: []string{"user_count"}, Rows: []map[string]any{{"user_count": int64(1200)}}},
		},
	}
	service := NewChatService(chatRepo, agent, executor)
	service.SetAgentContextBuilder(&chatFakeAgentContextBuilder{
		context: AgentContext{
			Datasources: []query.AgentDatasource{
				{ID: 7, Name: "灵数数据库", Type: "mysql", Dialect: "mysql"},
				{ID: 8, Name: "问卷数据库", Type: "mysql", Dialect: "mysql"},
			},
			Permission: query.AgentPermission{AllowedDatasourceIDs: []uint64{7, 8}},
		},
	})

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:              1,
		ProjectID:             2,
		SessionID:             10,
		UserID:                3,
		Content:               "对比两个系统用户数",
		SelectedDatasourceIDs: []uint64{7, 8},
		AutoExecute:           true,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if agent.synthesisCalls != 1 {
		t.Fatalf("expected one synthesis call, got %d", agent.synthesisCalls)
	}
	if len(agent.synthesisInput.Executions) != 2 || len(agent.synthesisInput.Datasources) != 2 {
		t.Fatalf("expected synthesis context, got %+v", agent.synthesisInput)
	}
	if result.Agent.Answer != agent.synthesisAnswer || result.Agent.Explanation != agent.synthesisAnswer {
		t.Fatalf("expected synthesized answer, got answer=%s explanation=%s", result.Agent.Answer, result.Agent.Explanation)
	}
	if result.Execution == nil || result.Execution.Answer != agent.synthesisAnswer {
		t.Fatalf("expected combined execution to carry synthesized answer, got %+v", result.Execution)
	}
}

func TestChatServiceSendMessageKeepsPartialMultiDatasourceResults(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		synthesisAnswer: "灵数数据库返回 2080；问卷数据库查询失败，已基于可用结果给出结论。",
		result: &query.AgentResult{
			Question:                "对比两个系统用户数",
			Intent:                  query.AgentIntentMultiQuery,
			Explanation:             "分别查询两个数据源后对比",
			RequiresMultiDatasource: true,
			DatasourceIDs:           []uint64{7, 8},
			Review:                  query.ReviewResult{Passed: true, RiskLevel: "low"},
			SQLTasks: []query.AgentSQLTask{
				{
					DatasourceID:   7,
					DatasourceName: "灵数数据库",
					Purpose:        "统计灵数用户数",
					SQL:            "select count(*) as user_count from users LIMIT 200",
					Review:         query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from users LIMIT 200"},
				},
				{
					DatasourceID:   8,
					DatasourceName: "问卷数据库",
					Purpose:        "统计问卷用户数",
					SQL:            "select count(*) as user_count from respondents LIMIT 200",
					Review:         query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) as user_count from respondents LIMIT 200"},
				},
			},
		},
	}
	executor := &chatFakeQueryExecutor{
		results: []*QueryExecutionResult{
			{Execution: &model.QueryExecution{ID: 101, Status: "success"}, Columns: []string{"user_count"}, Rows: []map[string]any{{"user_count": int64(2080)}}},
			{Execution: &model.QueryExecution{ID: 102, Status: "failed", ErrorMessage: "table respondents does not exist"}, Error: "table respondents does not exist"},
		},
		errs: []error{nil, errors.New("table respondents does not exist")},
	}
	service := NewChatService(chatRepo, agent, executor)
	service.SetAgentContextBuilder(&chatFakeAgentContextBuilder{
		context: AgentContext{
			Datasources: []query.AgentDatasource{
				{ID: 7, Name: "灵数数据库", Type: "mysql", Dialect: "mysql"},
				{ID: 8, Name: "问卷数据库", Type: "mysql", Dialect: "mysql"},
			},
			Permission: query.AgentPermission{AllowedDatasourceIDs: []uint64{7, 8}},
		},
	})

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:              1,
		ProjectID:             2,
		SessionID:             10,
		UserID:                3,
		Content:               "对比两个系统用户数",
		SelectedDatasourceIDs: []uint64{7, 8},
		AutoExecute:           true,
	})
	if err != nil {
		t.Fatalf("send message should preserve successful datasource results: %v", err)
	}
	if executor.calls != 2 {
		t.Fatalf("expected both datasource tasks to run, got %d", executor.calls)
	}
	if agent.calls != 1 {
		t.Fatalf("expected no retry when partial data is available, got %d calls", agent.calls)
	}
	if agent.synthesisCalls != 1 {
		t.Fatalf("expected synthesis with partial results, got %d calls", agent.synthesisCalls)
	}
	if len(result.Executions) != 2 || result.Executions[0].Error != "" || result.Executions[1].Error == "" {
		t.Fatalf("expected one successful and one failed execution, got %+v", result.Executions)
	}
	if result.Execution == nil || result.Execution.Execution == nil || result.Execution.Execution.ID != 101 {
		t.Fatalf("expected primary execution to use the successful result, got %+v", result.Execution)
	}
	if result.Agent.Answer != agent.synthesisAnswer {
		t.Fatalf("expected synthesized partial answer, got %s", result.Agent.Answer)
	}
}

func TestChatServiceSendMessageKeepsAssistantMessageWhenAutoExecuteFails(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "订单数",
			SQL:          "select count(*) from orders LIMIT 200",
			Explanation:  "统计订单数",
			DatasourceID: 7,
			Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from orders LIMIT 200"},
		},
	}
	executor := &chatFakeQueryExecutor{
		result: &QueryExecutionResult{
			Execution: &model.QueryExecution{ID: 99, Status: "failed", ErrorMessage: "table orders does not exist"},
			Answer:    "SQL 已生成，但自动执行失败：table orders does not exist",
			Error:     "table orders does not exist",
		},
		err: errors.New("table orders does not exist"),
	}
	service := NewChatService(chatRepo, agent, executor)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "订单数",
		DatasourceID: 7,
		AutoExecute:  true,
	})
	if err != nil {
		t.Fatalf("send message should keep the chat alive when execution fails: %v", err)
	}
	if result.Execution == nil || result.Execution.Error == "" {
		t.Fatalf("expected failed execution details, got %+v", result.Execution)
	}
	if result.AssistantMessage.QueryExecutionID != 99 {
		t.Fatalf("expected assistant message to reference failed execution 99, got %d", result.AssistantMessage.QueryExecutionID)
	}
	if len(chatRepo.messages) != 2 || chatRepo.messages[1].Role != "assistant" {
		t.Fatalf("expected user and assistant messages to be stored, got %+v", chatRepo.messages)
	}
}

func TestChatServiceSendMessageRetriesFailedSQLExecution(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		results: []*query.AgentResult{
			{
				Question:     "订单数",
				SQL:          "select count(*) from missing_orders LIMIT 200",
				Explanation:  "统计订单数",
				DatasourceID: 7,
				Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from missing_orders LIMIT 200"},
				Steps:        []query.AgentEvent{{Type: query.EventAction, Step: 1, Name: "llm.text2sql", Content: "生成第一版 SQL"}},
			},
			{
				Question:     "订单数",
				SQL:          "select count(*) from orders LIMIT 200",
				Explanation:  "修正表名后统计订单数",
				DatasourceID: 7,
				Review:       query.ReviewResult{Passed: true, NormalizedSQL: "select count(*) from orders LIMIT 200"},
				Steps:        []query.AgentEvent{{Type: query.EventAction, Step: 1, Name: "llm.text2sql", Content: "修复 SQL"}},
			},
		},
	}
	executor := &chatFakeQueryExecutor{
		results: []*QueryExecutionResult{
			{
				Execution: &model.QueryExecution{ID: 98, Status: "failed", ErrorMessage: "table missing_orders does not exist"},
				Error:     "table missing_orders does not exist",
			},
			{
				Execution: &model.QueryExecution{ID: 99, Status: "success"},
				Rows:      []map[string]any{{"count": int64(1)}},
			},
		},
		errs: []error{errors.New("table missing_orders does not exist"), nil},
	}
	service := NewChatService(chatRepo, agent, executor)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "订单数",
		DatasourceID: 7,
		AutoExecute:  true,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if agent.calls != 2 {
		t.Fatalf("expected two agent attempts, got %d", agent.calls)
	}
	if executor.calls != 2 {
		t.Fatalf("expected two execute attempts, got %d", executor.calls)
	}
	if agent.inputs[1].PreviousSQL == "" || agent.inputs[1].PreviousError == "" {
		t.Fatalf("expected retry feedback, got %+v", agent.inputs[1])
	}
	if result.Loops != 2 || result.Agent.SQL != "select count(*) from orders LIMIT 200" {
		t.Fatalf("unexpected retry result: loops=%d sql=%s", result.Loops, result.Agent.SQL)
	}
	if result.Execution == nil || result.Execution.Execution == nil || result.Execution.Execution.ID != 99 {
		t.Fatalf("expected successful second execution, got %+v", result.Execution)
	}
	if len(result.Agent.Steps) == 0 {
		t.Fatal("expected merged agent steps")
	}
}

func TestChatServiceSendMessageLoadsKnowledgeContext(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "GMV 是多少",
			Explanation:  "需要查询 GMV",
			Review:       query.ReviewResult{Passed: false},
			DatasourceID: 7,
		},
	}
	knowledge := &chatFakeKnowledgeProvider{
		context: &rag.Context{
			BusinessTerms: []query.AgentKnowledge{{Name: "GMV", Description: "成交金额"}},
			Metrics:       []query.AgentKnowledge{{Name: "销售额", Expression: "sum(pay_amount)"}},
			FewShots:      []query.AgentFewShot{{Question: "今天 GMV", SQL: "select sum(pay_amount) from orders"}},
		},
	}
	service := NewChatService(chatRepo, agent, nil, knowledge)

	_, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    10,
		UserID:       3,
		Content:      "GMV 是多少",
		DatasourceID: 7,
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if len(agent.lastInput.BusinessTerms) != 1 || agent.lastInput.BusinessTerms[0].Name != "GMV" {
		t.Fatalf("expected knowledge terms in agent input, got %+v", agent.lastInput.BusinessTerms)
	}
	if len(agent.lastInput.Metrics) != 1 || len(agent.lastInput.FewShots) != 1 {
		t.Fatalf("expected metric and fewshot context, got metrics=%+v fewshots=%+v", agent.lastInput.Metrics, agent.lastInput.FewShots)
	}
	if knowledge.lastInput.DatasourceID != 7 {
		t.Fatalf("expected datasource filter 7, got %d", knowledge.lastInput.DatasourceID)
	}
}

func TestChatServiceSendMessageContinuesWhenRAGFails(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:    "订单数",
			Explanation: "继续使用元数据上下文生成回答",
			Review:      query.ReviewResult{Passed: false},
		},
	}
	knowledge := &chatFakeKnowledgeProvider{err: errors.New("milvus search failed")}
	service := NewChatService(chatRepo, agent, nil, knowledge)

	result, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:  1,
		ProjectID: 2,
		SessionID: 10,
		UserID:    3,
		Content:   "订单数",
	})
	if err != nil {
		t.Fatalf("send message should continue without rag: %v", err)
	}
	if result.AssistantMessage == nil || result.AssistantMessage.Role != "assistant" {
		t.Fatalf("expected assistant message, got %+v", result.AssistantMessage)
	}
	if len(agent.lastInput.BusinessTerms) != 0 || len(agent.lastInput.Metrics) != 0 || len(agent.lastInput.FewShots) != 0 {
		t.Fatalf("expected empty rag context after retrieval failure, got terms=%+v metrics=%+v fewshots=%+v", agent.lastInput.BusinessTerms, agent.lastInput.Metrics, agent.lastInput.FewShots)
	}
}

func TestChatServiceSendMessageLoadsAgentDatasourceContext(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    3,
			Status:    "active",
		},
	}
	agent := &chatFakeAgentRunner{
		result: &query.AgentResult{
			Question:     "今天销售额是多少",
			Explanation:  "需要查询销售额",
			Review:       query.ReviewResult{Passed: false},
			DatasourceID: 7,
		},
	}
	service := NewChatService(chatRepo, agent, nil)
	service.SetAgentContextBuilder(&chatFakeAgentContextBuilder{
		context: AgentContext{
			Datasources: []query.AgentDatasource{
				{
					ID:      7,
					Name:    "orders",
					Type:    "mysql",
					Dialect: "mysql",
					Tables: []query.AgentTable{
						{Name: "orders", Columns: []query.AgentColumn{{Name: "pay_amount", Type: "decimal", Comment: "支付金额"}}},
					},
				},
			},
			Permission: query.AgentPermission{AllowedDatasourceIDs: []uint64{7}},
		},
	})

	_, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:  1,
		ProjectID: 2,
		SessionID: 10,
		UserID:    3,
		Content:   "今天销售额是多少",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if len(agent.lastInput.Datasources) != 1 || agent.lastInput.Datasources[0].Name != "orders" {
		t.Fatalf("expected datasource context in agent input, got %+v", agent.lastInput.Datasources)
	}
	if len(agent.lastInput.Permission.AllowedDatasourceIDs) != 1 || agent.lastInput.Permission.AllowedDatasourceIDs[0] != 7 {
		t.Fatalf("expected permission context in agent input, got %+v", agent.lastInput.Permission)
	}
}

func TestChatServiceSendMessageRejectsSessionOutsideUser(t *testing.T) {
	chatRepo := &chatFakeRepository{
		session: &model.ChatSession{
			BaseModel: model.BaseModel{ID: 10},
			TenantID:  1,
			ProjectID: 2,
			UserID:    9,
			Status:    "active",
		},
	}
	service := NewChatService(chatRepo, &chatFakeAgentRunner{}, nil)

	_, err := service.SendMessage(context.Background(), SendChatMessageInput{
		TenantID:  1,
		ProjectID: 2,
		SessionID: 10,
		UserID:    3,
		Content:   "订单数",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if len(chatRepo.messages) != 0 {
		t.Fatalf("expected no messages created, got %d", len(chatRepo.messages))
	}
}

type chatFakeRepository struct {
	session        *model.ChatSession
	sessions       []model.ChatSession
	messages       []model.ChatMessage
	recentMessages []model.ChatMessage
	nextMessageID  uint64
}

func (r *chatFakeRepository) CreateSession(ctx context.Context, session *model.ChatSession) error {
	if session.ID == 0 {
		session.ID = 1
	}
	r.session = session
	r.sessions = append(r.sessions, *session)
	return nil
}

func (r *chatFakeRepository) GetSession(ctx context.Context, id uint64) (*model.ChatSession, error) {
	if r.session == nil || r.session.ID != id {
		return nil, errors.New("session not found")
	}
	return r.session, nil
}

func (r *chatFakeRepository) ListSessions(ctx context.Context, filter repository.ChatSessionFilter, page repository.Page) ([]model.ChatSession, int64, error) {
	if r.session == nil {
		return nil, 0, nil
	}
	return []model.ChatSession{*r.session}, 1, nil
}

func (r *chatFakeRepository) CreateMessage(ctx context.Context, message *model.ChatMessage) error {
	r.nextMessageID++
	message.ID = r.nextMessageID
	message.CreatedAt = time.Now()
	r.messages = append(r.messages, *message)
	return nil
}

func (r *chatFakeRepository) ListMessages(ctx context.Context, filter repository.ChatMessageFilter, page repository.Page) ([]model.ChatMessage, int64, error) {
	return r.messages, int64(len(r.messages)), nil
}

func (r *chatFakeRepository) GetRecentMessages(ctx context.Context, sessionID uint64, limit int) ([]model.ChatMessage, error) {
	return r.recentMessages, nil
}

type chatFakeAgentRunner struct {
	result          *query.AgentResult
	results         []*query.AgentResult
	lastInput       AskInput
	inputs          []AskInput
	calls           int
	synthesisAnswer string
	synthesisErr    error
	synthesisInput  MultiResultSynthesisInput
	synthesisCalls  int
}

func (r *chatFakeAgentRunner) Ask(ctx context.Context, input AskInput) (*query.AgentResult, error) {
	r.lastInput = input
	r.inputs = append(r.inputs, input)
	r.calls++
	if len(r.results) > 0 {
		index := r.calls - 1
		if index >= len(r.results) {
			index = len(r.results) - 1
		}
		return r.results[index], nil
	}
	if r.result == nil {
		return &query.AgentResult{}, nil
	}
	return r.result, nil
}

func (r *chatFakeAgentRunner) StreamAsk(ctx context.Context, input AskInput, emit func(query.AgentEvent) error) error {
	result, err := r.Ask(ctx, input)
	if err != nil {
		return err
	}
	for _, event := range result.Steps {
		if err := emit(event); err != nil {
			return err
		}
	}
	return emit(query.AgentEvent{
		Type:       query.EventFinal,
		Step:       len(result.Steps) + 1,
		Name:       "agent.final",
		Content:    "任务完成。",
		Final:      result,
		OccurredAt: time.Now(),
	})
}

func (r *chatFakeAgentRunner) SynthesizeMultiResult(ctx context.Context, input MultiResultSynthesisInput) (string, error) {
	r.synthesisInput = input
	r.synthesisCalls++
	if r.synthesisErr != nil {
		return "", r.synthesisErr
	}
	return r.synthesisAnswer, nil
}

func hasAgentEvent(events []query.AgentEvent, eventType string, name string) bool {
	for _, event := range events {
		if event.Type == eventType && event.Name == name {
			return true
		}
	}
	return false
}

type chatFakeQueryExecutor struct {
	result    *QueryExecutionResult
	results   []*QueryExecutionResult
	err       error
	errs      []error
	lastInput ExecuteSQLInput
	inputs    []ExecuteSQLInput
	calls     int
}

func (e *chatFakeQueryExecutor) ExecuteSQL(ctx context.Context, input ExecuteSQLInput) (*QueryExecutionResult, error) {
	e.lastInput = input
	e.inputs = append(e.inputs, input)
	e.calls++
	if len(e.results) > 0 || len(e.errs) > 0 {
		index := e.calls - 1
		resultIndex := index
		if len(e.results) > 0 && resultIndex >= len(e.results) {
			resultIndex = len(e.results) - 1
		}
		errIndex := index
		if len(e.errs) > 0 && errIndex >= len(e.errs) {
			errIndex = len(e.errs) - 1
		}
		var result *QueryExecutionResult
		if len(e.results) > 0 {
			result = e.results[resultIndex]
		}
		var err error
		if len(e.errs) > 0 {
			err = e.errs[errIndex]
		}
		return result, err
	}
	if e.err != nil {
		return e.result, e.err
	}
	if e.result == nil {
		return &QueryExecutionResult{}, nil
	}
	return e.result, nil
}

type chatFakeKnowledgeProvider struct {
	context   *rag.Context
	err       error
	lastInput rag.Request
}

func (p *chatFakeKnowledgeProvider) Retrieve(ctx context.Context, input rag.Request) (*rag.Context, error) {
	p.lastInput = input
	if p.err != nil {
		return nil, p.err
	}
	return p.context, nil
}

type chatFakeAgentContextBuilder struct {
	context   AgentContext
	lastInput AgentContextInput
}

func (b *chatFakeAgentContextBuilder) BuildAgentContext(ctx context.Context, input AgentContextInput) (AgentContext, error) {
	b.lastInput = input
	return b.context, nil
}
