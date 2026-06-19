package query

import (
	"strings"
	"testing"
)

func TestPromptRendererText2SQLSystem(t *testing.T) {
	renderer, err := NewPromptRendererFromTemplates(testPromptTemplates("project={{.ProjectID}}, max={{.MaxRows}}, selected=[{{joinUint64 .SelectedDatasourceIDs \",\"}}], dialect={{.DefaultDialect}}"))
	if err != nil {
		t.Fatalf("new prompt renderer: %v", err)
	}

	rules, err := renderer.DialectRuleMap()
	if err != nil {
		t.Fatalf("dialect rules: %v", err)
	}
	content, err := renderer.Text2SQLSystem(NewPromptContext(AgentRequest{
		ProjectID:    10,
		DatasourceID: 20,
		MaxRows:      200,
	}, rules))
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if !strings.Contains(content, "project=10") {
		t.Fatalf("expected project id in prompt, got %q", content)
	}
	if !strings.Contains(content, "max=200") {
		t.Fatalf("expected max rows in prompt, got %q", content)
	}
	if !strings.Contains(content, "selected=[20]") {
		t.Fatalf("expected selected datasource in prompt, got %q", content)
	}
}

func TestPromptContextSupportsProjectDatasources(t *testing.T) {
	renderer, err := NewPromptRendererFromTemplates(testPromptTemplates("{{range .AvailableDatasources}}{{.Name}}/{{.Dialect}};{{end}} default={{.DefaultDialect}} {{range .DialectRules}}{{.Dialect}}={{.Content}}{{end}}"))
	if err != nil {
		t.Fatalf("new prompt renderer: %v", err)
	}
	rules, err := renderer.DialectRuleMap()
	if err != nil {
		t.Fatalf("dialect rules: %v", err)
	}
	content, err := renderer.Text2SQLSystem(NewPromptContext(AgentRequest{
		ProjectID: 10,
		Datasources: []AgentDatasource{
			{ID: 1, Name: "orders_ck", Type: "clickhouse"},
			{ID: 2, Name: "crm_pg", Type: "postgresql"},
		},
		SelectedDatasourceIDs: []uint64{2},
	}, rules))
	if err != nil {
		t.Fatalf("render prompt: %v", err)
	}
	if !strings.Contains(content, "orders_ck/clickhouse") {
		t.Fatalf("expected clickhouse datasource in prompt, got %q", content)
	}
	if !strings.Contains(content, "crm_pg/postgresql") {
		t.Fatalf("expected postgresql datasource in prompt, got %q", content)
	}
	if !strings.Contains(content, "default=postgresql") {
		t.Fatalf("expected selected datasource dialect as default, got %q", content)
	}
	if !strings.Contains(content, "postgresql=postgres rules") {
		t.Fatalf("expected dialect rules in prompt, got %q", content)
	}
}

func TestPromptRendererFromDirLoadsProjectTemplates(t *testing.T) {
	renderer, err := NewPromptRendererFromDir("../../prompts")
	if err != nil {
		t.Fatalf("load prompt renderer: %v", err)
	}
	rules, err := renderer.DialectRuleMap()
	if err != nil {
		t.Fatalf("dialect rules: %v", err)
	}
	content, err := renderer.Text2SQLSystem(NewPromptContext(AgentRequest{
		TenantID:     1,
		ProjectID:    2,
		DatasourceID: 3,
		Question:     "今天销售额是多少",
	}, rules))
	if err != nil {
		t.Fatalf("render real prompt: %v", err)
	}
	if !strings.Contains(content, "project_id: 2") {
		t.Fatalf("expected project context in prompt")
	}
	if !strings.Contains(content, "datasource_3") {
		t.Fatalf("expected compatibility datasource in prompt")
	}
	if !strings.Contains(content, "JSON Schema") {
		t.Fatalf("expected output contract in prompt")
	}
}

func testPromptTemplates(text2sql string) map[string]string {
	return map[string]string{
		templatePlannerSystem:          "planner project={{.ProjectID}}",
		templateDatasourceRouterSystem: "router project={{.ProjectID}}",
		templateText2SQLSystem:         text2sql,
		templateResultSynthesisSystem:  "synthesis question={{.Question}} {{range .ExecutionResults}}{{.DatasourceName}}={{.RowCount}};{{end}}",
		"dialect/mysql.tmpl":           "mysql rules",
		"dialect/postgresql.tmpl":      "postgres rules",
		"dialect/clickhouse.tmpl":      "clickhouse rules",
	}
}
