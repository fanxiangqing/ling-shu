package query

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

const (
	templatePlannerSystem          = "agent/planner_system.tmpl"
	templateDatasourceRouterSystem = "agent/datasource_router_system.tmpl"
	templateText2SQLSystem         = "agent/text2sql_system.tmpl"
	templateResultSynthesisSystem  = "agent/result_synthesis_system.tmpl"
)

type PromptRenderer struct {
	root *template.Template
}

type PromptContext struct {
	TenantID              uint64
	ProjectID             uint64
	UserID                uint64
	Question              string
	MaxRows               int
	Attempt               int
	PreviousSQL           string
	PreviousError         string
	DefaultDialect        string
	AvailableDatasources  []AgentDatasource
	SelectedDatasources   []AgentDatasource
	SelectedDatasourceIDs []uint64
	DialectRules          []DialectRule
	BusinessTerms         []AgentKnowledge
	Metrics               []AgentKnowledge
	FewShots              []AgentFewShot
	Conversation          []AgentMessage
	UserMemories          []AgentMemory
	TimeContext           AgentTimeContext
	Permission            AgentPermission
	SQLTasks              []AgentSQLTask
	ExecutionResults      []AgentExecutionSummary
	ResultAnalysisStatus  string
	ResultAnalysisDetail  string
}

type DialectRule struct {
	Dialect string
	Content string
}

func NewPromptRendererFromDir(promptDir string) (*PromptRenderer, error) {
	files := map[string]string{}
	err := filepath.WalkDir(promptDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".tmpl" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read prompt template %s: %w", path, err)
		}
		relative, err := filepath.Rel(promptDir, path)
		if err != nil {
			return fmt.Errorf("resolve prompt template %s: %w", path, err)
		}
		files[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load prompt templates: %w", err)
	}
	return NewPromptRendererFromTemplates(files)
}

func NewPromptRendererFromTemplates(files map[string]string) (*PromptRenderer, error) {
	root := template.New("prompts").Funcs(template.FuncMap{
		"joinUint64":               joinUint64,
		"plannerDatasourceCatalog": plannerDatasourceCatalog,
	})
	for name, content := range files {
		if _, err := root.New(name).Parse(content); err != nil {
			return nil, fmt.Errorf("parse prompt template %s: %w", name, err)
		}
	}
	renderer := &PromptRenderer{root: root}
	for _, name := range []string{templatePlannerSystem, templateDatasourceRouterSystem, templateText2SQLSystem} {
		if renderer.root.Lookup(name) == nil {
			return nil, fmt.Errorf("prompt template %s is required", name)
		}
	}
	return renderer, nil
}

func (r *PromptRenderer) PlannerSystem(data PromptContext) (string, error) {
	return r.render(templatePlannerSystem, data)
}

func (r *PromptRenderer) DatasourceRouterSystem(data PromptContext) (string, error) {
	return r.render(templateDatasourceRouterSystem, data)
}

func (r *PromptRenderer) Text2SQLSystem(data PromptContext) (string, error) {
	return r.render(templateText2SQLSystem, data)
}

func (r *PromptRenderer) ResultSynthesisSystem(data PromptContext) (string, error) {
	return r.render(templateResultSynthesisSystem, data)
}

func (r *PromptRenderer) DialectRuleMap() (map[string]string, error) {
	if r == nil || r.root == nil {
		return nil, ErrPromptNotConfigured
	}
	rules := map[string]string{}
	for _, tmpl := range r.root.Templates() {
		name := tmpl.Name()
		if !strings.HasPrefix(name, "dialect/") || filepath.Ext(name) != ".tmpl" {
			continue
		}
		dialect := strings.TrimSuffix(strings.TrimPrefix(name, "dialect/"), ".tmpl")
		content, err := r.render(name, PromptContext{})
		if err != nil {
			return nil, err
		}
		rules[dialect] = content
	}
	return rules, nil
}

func (r *PromptRenderer) render(name string, data PromptContext) (string, error) {
	if r == nil || r.root == nil {
		return "", ErrPromptNotConfigured
	}
	var buf bytes.Buffer
	if err := r.root.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render prompt template %s: %w", name, err)
	}
	return strings.TrimSpace(buf.String()), nil
}

func NewPromptContext(req AgentRequest, dialectRules map[string]string) PromptContext {
	maxRows := req.MaxRows
	if maxRows <= 0 {
		maxRows = 200
	}
	if strings.TrimSpace(req.TimeContext.CurrentTime) == "" {
		req.TimeContext = NewAgentTimeContext(time.Now(), "", DefaultAgentTimezone)
	}

	available := normalizeDatasources(req)
	selectedIDs := normalizeSelectedDatasourceIDs(req, available)
	selected := selectDatasources(available, selectedIDs)
	defaultDialect := inferDefaultDialect(available, selected)
	resultAnalysisStatus := strings.ToLower(strings.TrimSpace(req.ResultAnalysisStatus))
	if resultAnalysisStatus == "" {
		resultAnalysisStatus = "disabled"
	}
	resultAnalysisDetail := strings.TrimSpace(req.ResultAnalysisDetail)
	if resultAnalysisDetail == "" {
		switch resultAnalysisStatus {
		case "available":
			resultAnalysisDetail = "Python exec 已启用并可用于 SQL 执行后的无状态结果增强。"
		case "unavailable":
			resultAnalysisDetail = "Python exec 已启用但当前不可用；不要生成内部结果分析计划。"
		default:
			resultAnalysisDetail = "Python exec 未启用；不要生成内部结果分析计划。"
		}
	}

	return PromptContext{
		TenantID:              req.TenantID,
		ProjectID:             req.ProjectID,
		UserID:                req.UserID,
		Question:              req.Question,
		MaxRows:               maxRows,
		Attempt:               req.Attempt,
		PreviousSQL:           req.PreviousSQL,
		PreviousError:         req.PreviousError,
		DefaultDialect:        defaultDialect,
		AvailableDatasources:  available,
		SelectedDatasources:   selected,
		SelectedDatasourceIDs: selectedIDs,
		DialectRules:          buildDialectRules(defaultDialect, available, selected, dialectRules),
		BusinessTerms:         req.BusinessTerms,
		Metrics:               req.Metrics,
		FewShots:              req.FewShots,
		Conversation:          req.Conversation,
		UserMemories:          req.UserMemories,
		TimeContext:           req.TimeContext,
		Permission:            req.Permission,
		ResultAnalysisStatus:  resultAnalysisStatus,
		ResultAnalysisDetail:  resultAnalysisDetail,
	}
}

func normalizeDatasources(req AgentRequest) []AgentDatasource {
	seen := map[uint64]bool{}
	out := make([]AgentDatasource, 0, len(req.Datasources)+1)
	for _, ds := range req.Datasources {
		normalized := normalizeDatasource(ds)
		if normalized.ID == 0 {
			out = append(out, normalized)
			continue
		}
		if seen[normalized.ID] {
			continue
		}
		seen[normalized.ID] = true
		out = append(out, normalized)
	}
	if req.DatasourceID > 0 && !seen[req.DatasourceID] {
		out = append(out, normalizeDatasource(AgentDatasource{
			ID:      req.DatasourceID,
			Name:    fmt.Sprintf("datasource_%d", req.DatasourceID),
			Type:    "mysql",
			Dialect: "mysql",
		}))
	}
	return out
}

func normalizeDatasource(ds AgentDatasource) AgentDatasource {
	if ds.Name == "" && ds.ID > 0 {
		ds.Name = fmt.Sprintf("datasource_%d", ds.ID)
	}
	ds.Type = strings.ToLower(strings.TrimSpace(ds.Type))
	ds.Dialect = strings.ToLower(strings.TrimSpace(ds.Dialect))
	if ds.Dialect == "" {
		ds.Dialect = dialectFromDatasourceType(ds.Type)
	}
	if ds.Type == "" {
		ds.Type = ds.Dialect
	}
	if ds.Dialect == "" {
		ds.Dialect = "mysql"
	}
	return ds
}

func normalizeSelectedDatasourceIDs(req AgentRequest, available []AgentDatasource) []uint64 {
	ids := append([]uint64(nil), req.SelectedDatasourceIDs...)
	if len(ids) == 0 && req.DatasourceID > 0 {
		ids = append(ids, req.DatasourceID)
	}
	if len(ids) == 0 && len(available) == 1 && available[0].ID > 0 {
		ids = append(ids, available[0].ID)
	}
	return uniqueUint64(ids)
}

func selectDatasources(available []AgentDatasource, selectedIDs []uint64) []AgentDatasource {
	if len(selectedIDs) == 0 {
		return nil
	}
	selectedSet := map[uint64]bool{}
	for _, id := range selectedIDs {
		selectedSet[id] = true
	}
	out := make([]AgentDatasource, 0, len(selectedIDs))
	for _, ds := range available {
		if ds.ID > 0 && selectedSet[ds.ID] {
			out = append(out, ds)
		}
	}
	known := map[uint64]bool{}
	for _, ds := range out {
		known[ds.ID] = true
	}
	for _, id := range selectedIDs {
		if id > 0 && !known[id] {
			out = append(out, normalizeDatasource(AgentDatasource{
				ID:      id,
				Name:    fmt.Sprintf("datasource_%d", id),
				Type:    "mysql",
				Dialect: "mysql",
			}))
		}
	}
	return out
}

func inferDefaultDialect(available []AgentDatasource, selected []AgentDatasource) string {
	if len(selected) > 0 {
		return selected[0].Dialect
	}
	if len(available) > 0 {
		return available[0].Dialect
	}
	return "mysql"
}

func buildDialectRules(defaultDialect string, available []AgentDatasource, selected []AgentDatasource, rules map[string]string) []DialectRule {
	dialects := map[string]bool{}
	for _, ds := range selected {
		if ds.Dialect != "" {
			dialects[ds.Dialect] = true
		}
	}
	if len(dialects) == 0 {
		for _, ds := range available {
			if ds.Dialect != "" {
				dialects[ds.Dialect] = true
			}
		}
	}
	if len(dialects) == 0 {
		dialects[defaultDialect] = true
	}

	names := make([]string, 0, len(dialects))
	for dialect := range dialects {
		names = append(names, dialect)
	}
	sort.Strings(names)

	out := make([]DialectRule, 0, len(names))
	for _, dialect := range names {
		content := rules[dialect]
		if content == "" && defaultDialect != "" {
			content = rules[defaultDialect]
		}
		out = append(out, DialectRule{Dialect: dialect, Content: strings.TrimSpace(content)})
	}
	return out
}

func dialectFromDatasourceType(datasourceType string) string {
	switch strings.ToLower(strings.TrimSpace(datasourceType)) {
	case "postgres", "postgresql":
		return "postgresql"
	case "oracle":
		return "oracle"
	case "sqlserver", "mssql":
		return "sqlserver"
	case "kingbase", "kingbasees":
		return "kingbase"
	case "dm", "dm8", "dameng":
		return "dm8"
	case "clickhouse":
		return "clickhouse"
	case "doris":
		return "doris"
	default:
		if datasourceType == "" {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(datasourceType))
	}
}

func uniqueUint64(values []uint64) []uint64 {
	seen := map[uint64]bool{}
	out := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func joinUint64(values []uint64, sep string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return strings.Join(parts, sep)
}

const (
	plannerCatalogDatasourceLimit    = 12
	plannerCatalogTableLimit         = 24
	plannerCatalogSelectedTableLimit = 40
)

func plannerDatasourceCatalog(datasources []AgentDatasource, selectedIDs []uint64) string {
	if len(datasources) == 0 {
		return "- 暂无项目数据源。"
	}

	selectedSet := map[uint64]bool{}
	for _, id := range selectedIDs {
		if id > 0 {
			selectedSet[id] = true
		}
	}
	ordered := prioritizePlannerDatasources(datasources, selectedSet)
	limit := len(ordered)
	if limit > plannerCatalogDatasourceLimit {
		limit = plannerCatalogDatasourceLimit
	}

	var b strings.Builder
	for i, ds := range ordered[:limit] {
		if i > 0 {
			b.WriteByte('\n')
		}
		role := firstNonEmpty(ds.Role, "available")
		if selectedSet[ds.ID] || ds.IsDefault {
			role = "selected"
		}
		fmt.Fprintf(&b, "- id=%d, name=%s, type=%s, dialect=%s", ds.ID, ds.Name, ds.Type, ds.Dialect)
		if ds.Version != "" {
			fmt.Fprintf(&b, ", version=%s", ds.Version)
		}
		fmt.Fprintf(&b, ", role=%s, tables=%d", role, len(ds.Tables))
		if ds.Description != "" {
			fmt.Fprintf(&b, ", description=%s", ds.Description)
		}
		b.WriteByte('\n')
		b.WriteString("  tables: ")
		if len(ds.Tables) == 0 {
			b.WriteString("暂无已同步表元数据")
			continue
		}
		tableLimit := plannerCatalogTableLimit
		if selectedSet[ds.ID] || ds.IsDefault || len(ordered) == 1 {
			tableLimit = plannerCatalogSelectedTableLimit
		}
		if tableLimit > len(ds.Tables) {
			tableLimit = len(ds.Tables)
		}
		for j, table := range ds.Tables[:tableLimit] {
			if j > 0 {
				b.WriteString("; ")
			}
			b.WriteString(formatPlannerTableLabel(table))
		}
		if omitted := len(ds.Tables) - tableLimit; omitted > 0 {
			fmt.Fprintf(&b, "; ... 其余 %d 张表未展示", omitted)
		}
	}
	if omitted := len(ordered) - limit; omitted > 0 {
		fmt.Fprintf(&b, "\n- ... 其余 %d 个数据源未展示", omitted)
	}
	return strings.TrimSpace(b.String())
}

func prioritizePlannerDatasources(datasources []AgentDatasource, selectedSet map[uint64]bool) []AgentDatasource {
	out := make([]AgentDatasource, 0, len(datasources))
	appended := make([]bool, len(datasources))
	for i, ds := range datasources {
		if selectedSet[ds.ID] || ds.IsDefault || strings.EqualFold(strings.TrimSpace(ds.Role), "selected") {
			out = append(out, ds)
			appended[i] = true
		}
	}
	for i, ds := range datasources {
		if appended[i] {
			continue
		}
		out = append(out, ds)
	}
	return out
}

func formatPlannerTableLabel(table AgentTable) string {
	name := strings.TrimSpace(table.Name)
	if strings.TrimSpace(table.Schema) != "" {
		name = strings.TrimSpace(table.Schema) + "." + name
	}
	if strings.TrimSpace(table.Comment) == "" {
		return name
	}
	return fmt.Sprintf("%s(%s)", name, strings.TrimSpace(table.Comment))
}
