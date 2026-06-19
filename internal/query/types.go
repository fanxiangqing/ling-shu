package query

import "time"

type AgentRequest struct {
	TenantID              uint64            `json:"tenant_id"`
	ProjectID             uint64            `json:"project_id"`
	DatasourceID          uint64            `json:"datasource_id"`
	SelectedDatasourceIDs []uint64          `json:"selected_datasource_ids,omitempty"`
	UserID                uint64            `json:"user_id"`
	Question              string            `json:"question"`
	MaxRows               int               `json:"max_rows"`
	Attempt               int               `json:"attempt,omitempty"`
	PreviousSQL           string            `json:"previous_sql,omitempty"`
	PreviousError         string            `json:"previous_error,omitempty"`
	Datasources           []AgentDatasource `json:"datasources,omitempty"`
	BusinessTerms         []AgentKnowledge  `json:"business_terms,omitempty"`
	Metrics               []AgentKnowledge  `json:"metrics,omitempty"`
	FewShots              []AgentFewShot    `json:"few_shots,omitempty"`
	Conversation          []AgentMessage    `json:"conversation,omitempty"`
	Permission            AgentPermission   `json:"permission,omitempty"`
}

type AgentResultSynthesisRequest struct {
	AgentRequest
	SQLTasks         []AgentSQLTask          `json:"sql_tasks,omitempty"`
	ExecutionResults []AgentExecutionSummary `json:"execution_results,omitempty"`
}

type AgentResult struct {
	Question                string         `json:"question"`
	Intent                  string         `json:"intent,omitempty"`
	SQL                     string         `json:"sql"`
	SQLTasks                []AgentSQLTask `json:"sql_tasks,omitempty"`
	Answer                  string         `json:"answer,omitempty"`
	Explanation             string         `json:"explanation"`
	DatasourceID            uint64         `json:"datasource_id,omitempty"`
	DatasourceIDs           []uint64       `json:"datasource_ids,omitempty"`
	Dialect                 string         `json:"dialect,omitempty"`
	RequiresMultiDatasource bool           `json:"requires_multi_datasource,omitempty"`
	NeedClarification       bool           `json:"need_clarification,omitempty"`
	Review                  ReviewResult   `json:"review"`
	Steps                   []AgentEvent   `json:"steps"`
}

type AgentSQLTask struct {
	DatasourceID   uint64       `json:"datasource_id"`
	DatasourceName string       `json:"datasource_name,omitempty"`
	Dialect        string       `json:"dialect,omitempty"`
	Purpose        string       `json:"purpose,omitempty"`
	SQL            string       `json:"sql"`
	Review         ReviewResult `json:"review"`
}

type AgentExecutionSummary struct {
	DatasourceID   uint64           `json:"datasource_id"`
	DatasourceName string           `json:"datasource_name,omitempty"`
	Purpose        string           `json:"purpose,omitempty"`
	Columns        []string         `json:"columns,omitempty"`
	Rows           []map[string]any `json:"rows,omitempty"`
	RowCount       int              `json:"row_count,omitempty"`
	ChartType      string           `json:"chart_type,omitempty"`
	Answer         string           `json:"answer,omitempty"`
	Error          string           `json:"error,omitempty"`
}

type AgentEvent struct {
	Type       string        `json:"type"`
	Step       int           `json:"step"`
	Name       string        `json:"name,omitempty"`
	Content    string        `json:"content,omitempty"`
	SQL        string        `json:"sql,omitempty"`
	Review     *ReviewResult `json:"review,omitempty"`
	Final      *AgentResult  `json:"final,omitempty"`
	OccurredAt time.Time     `json:"occurred_at"`
}

type ReviewResult struct {
	Passed        bool     `json:"passed"`
	RiskLevel     string   `json:"risk_level"`
	NormalizedSQL string   `json:"normalized_sql"`
	BlockedReason string   `json:"blocked_reason,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
	Limit         int      `json:"limit,omitempty"`
}

type AgentDatasource struct {
	ID          uint64       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Dialect     string       `json:"dialect"`
	Version     string       `json:"version,omitempty"`
	Description string       `json:"description,omitempty"`
	Role        string       `json:"role,omitempty"`
	IsDefault   bool         `json:"is_default,omitempty"`
	Tables      []AgentTable `json:"tables,omitempty"`
}

type AgentTable struct {
	Schema      string            `json:"schema,omitempty"`
	Name        string            `json:"name"`
	Comment     string            `json:"comment,omitempty"`
	Columns     []AgentColumn     `json:"columns,omitempty"`
	PrimaryKeys []string          `json:"primary_keys,omitempty"`
	Indexes     []AgentIndex      `json:"indexes,omitempty"`
	ForeignKeys []AgentForeignKey `json:"foreign_keys,omitempty"`
}

type AgentColumn struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Comment   string `json:"comment,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

type AgentIndex struct {
	Name    string   `json:"name"`
	Type    string   `json:"type,omitempty"`
	Unique  bool     `json:"unique,omitempty"`
	Columns []string `json:"columns,omitempty"`
}

type AgentForeignKey struct {
	Name             string `json:"name,omitempty"`
	Column           string `json:"column"`
	ReferencedSchema string `json:"referenced_schema,omitempty"`
	ReferencedTable  string `json:"referenced_table"`
	ReferencedColumn string `json:"referenced_column"`
}

type AgentKnowledge struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Expression  string `json:"expression,omitempty"`
}

type AgentFewShot struct {
	Question     string `json:"question"`
	SQL          string `json:"sql"`
	DatasourceID uint64 `json:"datasource_id,omitempty"`
	Dialect      string `json:"dialect,omitempty"`
}

type AgentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AgentPermission struct {
	AllowedDatasourceIDs []uint64 `json:"allowed_datasource_ids,omitempty"`
	AllowedSchemas       []string `json:"allowed_schemas,omitempty"`
	AllowedTables        []string `json:"allowed_tables,omitempty"`
	DeniedTables         []string `json:"denied_tables,omitempty"`
	DeniedColumns        []string `json:"denied_columns,omitempty"`
}

const (
	EventThought     = "thought"
	EventAction      = "action"
	EventObservation = "observation"
	EventLLMDelta    = "llm_delta"
	EventFinal       = "final"
	EventError       = "error"
)
