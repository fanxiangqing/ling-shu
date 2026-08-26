package memory

import (
	"context"
	"time"

	"ling-shu/internal/query"
)

const (
	ArtifactKindDataset  = "dataset"
	ArtifactStatusActive = "active"

	CompletenessComplete = "complete"
	CompletenessBounded  = "bounded"
	CompletenessPreview  = "preview"

	ActionNone      = "none"
	ActionVisualize = "visualize"
	ActionClarify   = "clarify"
)

type Scope struct {
	TenantID  uint64
	ProjectID uint64
	SessionID uint64
	UserID    uint64
}

func (s Scope) Valid() bool {
	return s.TenantID > 0 && s.ProjectID > 0 && s.SessionID > 0 && s.UserID > 0
}

type Measure struct {
	Field       string `json:"field"`
	Label       string `json:"label,omitempty"`
	Aggregation string `json:"aggregation,omitempty"`
	Unit        string `json:"unit,omitempty"`
}

type ArtifactSemantics struct {
	Dimensions []string  `json:"dimensions,omitempty"`
	Measures   []Measure `json:"measures,omitempty"`
}

type ArtifactLineage struct {
	QueryExecutionIDs []uint64 `json:"query_execution_ids,omitempty"`
	DatasourceIDs     []uint64 `json:"datasource_ids,omitempty"`
	DerivedFromID     uint64   `json:"derived_from_id,omitempty"`
}

type ArtifactPayload struct {
	Columns []string              `json:"columns,omitempty"`
	Rows    []map[string]any      `json:"rows,omitempty"`
	Chart   query.ChartSuggestion `json:"chart,omitempty"`
	Answer  string                `json:"answer,omitempty"`
}

type Artifact struct {
	ID              uint64
	Scope           Scope
	SourceMessageID uint64
	Purpose         string
	Kind            string
	Status          string
	Completeness    string
	Payload         ArtifactPayload
	Semantics       ArtifactSemantics
	Lineage         ArtifactLineage
	CreatedAt       time.Time
}

type SessionState struct {
	Scope             Scope
	Summary           string
	LastIntent        string
	ActiveArtifactIDs []uint64
	FocusArtifactID   uint64
	Version           uint64
	UpdatedAt         time.Time
}

type ExecutionSnapshot struct {
	QueryExecutionID uint64
	DatasourceID     uint64
	Purpose          string
	SQL              string
	Status           string
	RowCount         int
	Limit            int
	Columns          []string
	Rows             []map[string]any
	Chart            query.ChartSuggestion
	Answer           string
}

type TurnInput struct {
	Scope           Scope
	SourceMessageID uint64
	Question        string
	Answer          string
	Intent          string
	Executions      []ExecutionSnapshot
}

type Resolution struct {
	Action     string
	Confidence float64
	Reason     string
	Answer     string
	Artifact   *Artifact
}

func (r Resolution) Handled() bool {
	return r.Action == ActionVisualize || r.Action == ActionClarify
}

type PreparedContext struct {
	Conversation []query.AgentMessage
	State        SessionState
	Artifacts    []Artifact
	Resolution   Resolution
}

type Store interface {
	LoadSessionState(ctx context.Context, scope Scope) (SessionState, error)
	ListRecentArtifacts(ctx context.Context, scope Scope, limit int) ([]Artifact, error)
	SaveTurn(ctx context.Context, state SessionState, artifacts []Artifact, focusIndex int) (SessionState, []Artifact, error)
}
