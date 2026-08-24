package service

import (
	"context"
	"errors"
	"testing"
	"time"

	auditpkg "ling-shu/internal/audit"
	"ling-shu/internal/model"
	"ling-shu/internal/pyexecclient"
	"ling-shu/internal/query"
)

func TestResultAnalysisServiceAnalyzeTruncatesRowsAndAudits(t *testing.T) {
	client := &fakeResultAnalysisClient{
		response: &pyexecclient.AnalyzeResponse{
			Success:        true,
			Summary:        "已生成分类汇总。",
			AnalysisKind:   "category",
			TemplateName:   "category_analysis",
			InputRowCount:  1,
			OutputRowCount: 1,
		},
	}
	recorder := &recordingAuditRecorder{}
	service := NewResultAnalysisService(client, ResultAnalysisConfig{
		Enabled:        true,
		Timeout:        3 * time.Second,
		MaxInputRows:   1,
		MaxOutputRows:  10,
		MaxStdoutChars: 100,
		FailOpen:       true,
	})
	service.SetAuditRecorder(recorder)

	resp, err := service.AnalyzeQueryResults(context.Background(), AnalyzeQueryResultsInput{
		TenantID:     1,
		ProjectID:    2,
		SessionID:    3,
		UserID:       4,
		RequestID:    "rid-analysis",
		Question:     "按省份统计销售额",
		Mode:         "template",
		AnalysisGoal: "result = df",
		TemplateName: "category_analysis",
		TemplateParams: map[string]any{
			"category_field": "province",
			"value_field":    "amount",
		},
		Tasks: []query.AgentSQLTask{{
			DatasourceID:   7,
			DatasourceName: "订单库",
			Purpose:        "统计销售额",
		}},
		Executions: []*QueryExecutionResult{{
			Execution: &model.QueryExecution{ID: 99, DatasourceID: 7},
			Columns:   []string{"province", "amount"},
			Rows: []map[string]any{
				{"province": "浙江", "amount": 10},
				{"province": "江苏", "amount": 8},
			},
		}},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("expected successful analysis, got %+v", resp)
	}
	if client.calls != 1 {
		t.Fatalf("expected one client call, got %d", client.calls)
	}
	if got := len(client.lastInput.Datasets[0].Rows); got != 1 {
		t.Fatalf("expected rows truncated to 1, got %d", got)
	}
	if client.lastInput.RequestID != "rid-analysis" || client.lastInput.TenantID != 1 || client.lastInput.ProjectID != 2 || client.lastInput.SessionID != 3 || client.lastInput.UserID != 4 {
		t.Fatalf("expected trace fields to be forwarded, got %+v", client.lastInput)
	}
	if client.lastInput.Mode != "template" || client.lastInput.TemplateName != "category_analysis" || client.lastInput.TemplateParams["value_field"] != "amount" {
		t.Fatalf("expected template fields to be forwarded, got %+v", client.lastInput)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(recorder.events))
	}
	event := recorder.events[0]
	if event.EventType != auditpkg.EventPythonAnalyze || event.ResourceType != auditpkg.ResourcePythonAnalysis || event.ResourceID != 99 || event.RequestID != "rid-analysis" {
		t.Fatalf("unexpected audit event: %+v", event)
	}
	if event.Payload["input_row_count"] != 1 || event.Payload["analysis_kind"] != "category" || event.Payload["template_name"] != "category_analysis" {
		t.Fatalf("unexpected audit payload: %+v", event.Payload)
	}
	if event.Payload["analysis_goal"] != nil || event.Payload["analysis_goal_chars"] != 11 || event.Payload["analysis_goal_hash"] == "" {
		t.Fatalf("expected hashed analysis goal audit payload, got %+v", event.Payload)
	}
}

func TestResultAnalysisServiceFailOpenKeepsMainFlow(t *testing.T) {
	client := &fakeResultAnalysisClient{err: errors.New("exec unavailable")}
	recorder := &recordingAuditRecorder{}
	service := NewResultAnalysisService(client, ResultAnalysisConfig{Enabled: true, FailOpen: true})
	service.SetAuditRecorder(recorder)

	resp, err := service.AnalyzeQueryResults(context.Background(), minimalAnalysisInput())
	if err != nil {
		t.Fatalf("expected fail-open nil error, got %v", err)
	}
	if resp != nil {
		t.Fatalf("expected no analysis response on fail-open, got %+v", resp)
	}
	if len(recorder.events) != 1 || recorder.events[0].Payload["success"] != false {
		t.Fatalf("expected failed audit event, got %+v", recorder.events)
	}
}

func TestResultAnalysisServiceFailClosedReturnsError(t *testing.T) {
	client := &fakeResultAnalysisClient{err: errors.New("exec unavailable")}
	service := NewResultAnalysisService(client, ResultAnalysisConfig{Enabled: true, FailOpen: false})

	_, err := service.AnalyzeQueryResults(context.Background(), minimalAnalysisInput())
	if err == nil {
		t.Fatal("expected fail-closed error")
	}
}

func minimalAnalysisInput() AnalyzeQueryResultsInput {
	return AnalyzeQueryResultsInput{
		TenantID:  1,
		ProjectID: 2,
		SessionID: 3,
		UserID:    4,
		RequestID: "rid-analysis",
		Executions: []*QueryExecutionResult{{
			Execution: &model.QueryExecution{ID: 99},
			Columns:   []string{"value"},
			Rows:      []map[string]any{{"value": 1}},
		}},
	}
}

type fakeResultAnalysisClient struct {
	response  *pyexecclient.AnalyzeResponse
	err       error
	lastInput pyexecclient.AnalyzeRequest
	calls     int
}

func (c *fakeResultAnalysisClient) Analyze(ctx context.Context, input pyexecclient.AnalyzeRequest) (*pyexecclient.AnalyzeResponse, error) {
	c.lastInput = input
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return c.response, nil
}

func (c *fakeResultAnalysisClient) Health(ctx context.Context) (*pyexecclient.HealthStatus, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pyexecclient.HealthStatus{OK: true, Version: "test", Capabilities: map[string]bool{"stateless": true}}, nil
}
