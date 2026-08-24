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
	if !strings.Contains(content, "Python exec 结果分析状态") || !strings.Contains(content, "status=disabled") {
		t.Fatalf("expected disabled python exec status in prompt")
	}
	if strings.Contains(content, "analysis_goal 必须是一段可直接执行的 Python 代码") {
		t.Fatalf("disabled python exec prompt should not expose code contract")
	}
	if !strings.Contains(content, "result_analysis：当前不可用，不要输出该字段") {
		t.Fatalf("expected prompt to forbid result_analysis when exec is disabled")
	}
}

func TestPromptRendererPlannerShowsMetadataAndAvoidsOverClarification(t *testing.T) {
	renderer, err := NewPromptRendererFromDir("../../prompts")
	if err != nil {
		t.Fatalf("load prompt renderer: %v", err)
	}
	rules, err := renderer.DialectRuleMap()
	if err != nil {
		t.Fatalf("dialect rules: %v", err)
	}
	content, err := renderer.PlannerSystem(NewPromptContext(AgentRequest{
		TenantID:  1,
		ProjectID: 2,
		Question:  "现在有多少用户",
		Datasources: []AgentDatasource{
			{
				ID:      3,
				Name:    "问卷数据库",
				Type:    "mysql",
				Dialect: "mysql",
				Tables: []AgentTable{
					{Schema: "questionnaire_temp", Name: "users", Comment: "账号注册用户"},
				},
			},
		},
	}, rules))
	if err != nil {
		t.Fatalf("render planner prompt: %v", err)
	}
	if !strings.Contains(content, "questionnaire_temp.users") {
		t.Fatalf("expected planner prompt to include table metadata, got %q", content)
	}
	if !strings.Contains(content, "少问多做") || !strings.Contains(content, "优先返回 intent=query") {
		t.Fatalf("expected planner prompt to discourage premature clarification")
	}
}

func TestPromptRendererText2SQLShowsExecPlanOnlyWhenAvailable(t *testing.T) {
	renderer, err := NewPromptRendererFromDir("../../prompts")
	if err != nil {
		t.Fatalf("load prompt renderer: %v", err)
	}
	rules, err := renderer.DialectRuleMap()
	if err != nil {
		t.Fatalf("dialect rules: %v", err)
	}
	content, err := renderer.Text2SQLSystem(NewPromptContext(AgentRequest{
		TenantID:             1,
		ProjectID:            2,
		DatasourceID:         3,
		Question:             "按省份统计销售额占比",
		ResultAnalysisStatus: "available",
		ResultAnalysisDetail: "Python exec 可用。",
	}, rules))
	if err != nil {
		t.Fatalf("render real prompt: %v", err)
	}
	if !strings.Contains(content, "内部结果分析沙箱") || !strings.Contains(content, "analysis_goal 必须是一段可直接执行的 Python 代码") {
		t.Fatalf("expected available python exec analysis contract in prompt")
	}
	if !strings.Contains(content, "\"result_analysis\"") {
		t.Fatalf("expected result_analysis schema when exec is available")
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
