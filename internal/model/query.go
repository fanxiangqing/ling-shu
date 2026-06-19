package model

import "time"

type QueryExecution struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID          uint64     `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID         uint64     `gorm:"column:project_id;not null" json:"project_id"`
	DatasourceID      uint64     `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	SessionID         uint64     `gorm:"column:session_id" json:"session_id,omitempty"`
	UserID            uint64     `gorm:"column:user_id;not null" json:"user_id"`
	Question          string     `gorm:"column:question;type:text;not null" json:"question"`
	GeneratedSQL      string     `gorm:"column:generated_sql;type:mediumtext" json:"generated_sql,omitempty"`
	FinalSQL          string     `gorm:"column:final_sql;type:mediumtext" json:"final_sql,omitempty"`
	SQLHash           string     `gorm:"column:sql_hash;size:64" json:"sql_hash,omitempty"`
	LLMProvider       string     `gorm:"column:llm_provider;size:64" json:"llm_provider,omitempty"`
	LLMModel          string     `gorm:"column:llm_model;size:128" json:"llm_model,omitempty"`
	PromptTokens      *int       `gorm:"column:prompt_tokens" json:"prompt_tokens,omitempty"`
	CompletionTokens  *int       `gorm:"column:completion_tokens" json:"completion_tokens,omitempty"`
	TotalTokens       *int       `gorm:"column:total_tokens" json:"total_tokens,omitempty"`
	Status            string     `gorm:"column:status;size:32;not null;default:pending" json:"status"`
	RowCount          *int       `gorm:"column:row_count" json:"row_count,omitempty"`
	DurationMS        *int       `gorm:"column:duration_ms" json:"duration_ms,omitempty"`
	ChartType         string     `gorm:"column:chart_type;size:64" json:"chart_type,omitempty"`
	ResultPreviewJSON *string    `gorm:"column:result_preview_json;type:json" json:"result_preview_json,omitempty"`
	ErrorMessage      string     `gorm:"column:error_message;type:text" json:"error_message,omitempty"`
	CreatedAt         time.Time  `gorm:"column:created_at" json:"created_at"`
	FinishedAt        *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
}

func (QueryExecution) TableName() string {
	return "query_executions"
}

type SQLReviewResult struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	TenantID         uint64    `gorm:"column:tenant_id;not null" json:"tenant_id"`
	ProjectID        uint64    `gorm:"column:project_id;not null" json:"project_id"`
	QueryExecutionID uint64    `gorm:"column:query_execution_id" json:"query_execution_id,omitempty"`
	DatasourceID     uint64    `gorm:"column:datasource_id" json:"datasource_id,omitempty"`
	SQLText          string    `gorm:"column:sql_text;type:mediumtext;not null" json:"sql_text"`
	Passed           bool      `gorm:"column:passed;not null" json:"passed"`
	RiskLevel        string    `gorm:"column:risk_level;size:32;not null;default:low" json:"risk_level"`
	BlockedReason    string    `gorm:"column:blocked_reason;type:text" json:"blocked_reason,omitempty"`
	RulesJSON        *string   `gorm:"column:rules_json;type:json" json:"rules_json,omitempty"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SQLReviewResult) TableName() string {
	return "sql_review_results"
}
