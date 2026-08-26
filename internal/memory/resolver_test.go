package memory

import (
	"testing"
	"time"

	"ling-shu/internal/query"
)

func TestResolveFollowUpBuildsBarFromMostInformativeDataset(t *testing.T) {
	now := time.Now()
	artifacts := []Artifact{
		datasetArtifact("查询当前最大环号", []string{"current_ring"}, []map[string]any{{"current_ring": 1254}}, now),
		datasetArtifact("查询当前施工风险列表", []string{"风险名称", "风险等级"}, []map[string]any{
			{"风险名称": "下穿厂房", "风险等级": "II"},
			{"风险名称": "下穿厂房", "风险等级": "II"},
			{"风险名称": "公铁交叉", "风险等级": "II"},
		}, now.Add(time.Second)),
		datasetArtifact("统计风险点总数", []string{"risk_point_count"}, []map[string]any{{"risk_point_count": 17}}, now.Add(2*time.Second)),
	}

	resolution := ResolveFollowUp("给我制作一个柱状图展示", SessionState{}, artifacts)
	if resolution.Action != ActionVisualize || resolution.Artifact == nil {
		t.Fatalf("expected visualization resolution, got %+v", resolution)
	}
	if resolution.Artifact.Payload.Chart.Type != query.ChartBar {
		t.Fatalf("expected bar chart, got %+v", resolution.Artifact.Payload.Chart)
	}
	if resolution.Artifact.Payload.Columns[0] != "风险名称" || len(resolution.Artifact.Payload.Rows) != 2 {
		t.Fatalf("expected risk rows grouped by name, got %+v", resolution.Artifact.Payload)
	}
	if resolution.Artifact.Payload.Rows[0]["数量"] != 2 {
		t.Fatalf("expected grouped count, got %+v", resolution.Artifact.Payload.Rows)
	}
}

func TestResolveFollowUpDoesNotCombineIncompatibleScalarUnits(t *testing.T) {
	now := time.Now()
	artifacts := []Artifact{
		datasetArtifact("当前最大环号", []string{"current_ring"}, []map[string]any{{"current_ring": 1254}}, now),
		datasetArtifact("风险点总数", []string{"risk_point_count"}, []map[string]any{{"risk_point_count": 17}}, now.Add(time.Second)),
	}

	resolution := ResolveFollowUp("做一个柱状图", SessionState{}, artifacts)
	if resolution.Action != ActionClarify || resolution.Artifact != nil {
		t.Fatalf("expected clarification instead of mixed-unit chart, got %+v", resolution)
	}
}

func TestResolveFollowUpRefusesAggregationForBoundedDataset(t *testing.T) {
	artifact := datasetArtifact("风险列表", []string{"风险名称"}, []map[string]any{
		{"风险名称": "A"},
		{"风险名称": "B"},
	}, time.Now())
	artifact.Completeness = CompletenessBounded

	resolution := ResolveFollowUp("用柱状图展示", SessionState{}, []Artifact{artifact})
	if resolution.Action != ActionNone {
		t.Fatalf("expected unresolved bounded aggregation, got %+v", resolution)
	}
}

func TestResolveFollowUpCreatesDimensionForSingleScalar(t *testing.T) {
	artifact := datasetArtifact("风险点总数", []string{"risk_point_count"}, []map[string]any{{"risk_point_count": 17}}, time.Now())

	resolution := ResolveFollowUp("把风险点总数做成柱状图", SessionState{}, []Artifact{artifact})
	if resolution.Action != ActionVisualize || resolution.Artifact == nil {
		t.Fatalf("expected scalar visualization, got %+v", resolution)
	}
	chart := resolution.Artifact.Payload.Chart
	if chart.XField != "指标" || len(chart.YFields) != 1 || chart.YFields[0] != "数值" {
		t.Fatalf("expected generated scalar axes, got %+v", chart)
	}
	if len(resolution.Artifact.Payload.Rows) != 1 || resolution.Artifact.Payload.Rows[0]["数值"] != 17 {
		t.Fatalf("unexpected scalar rows: %+v", resolution.Artifact.Payload.Rows)
	}
}

func TestResolveFollowUpExplicitTargetOverridesFocus(t *testing.T) {
	ring := datasetArtifact("查询当前最大环号", []string{"current_ring"}, []map[string]any{{"current_ring": 1254}}, time.Now())
	ring.ID = 10
	risks := datasetArtifact("查询当前施工风险列表", []string{"风险名称"}, []map[string]any{{"风险名称": "A"}, {"风险名称": "B"}}, time.Now().Add(time.Second))
	risks.ID = 11

	resolution := ResolveFollowUp("把最大环号做成柱状图", SessionState{FocusArtifactID: 11}, []Artifact{ring, risks})
	if resolution.Action != ActionVisualize || resolution.Artifact == nil {
		t.Fatalf("expected explicit artifact visualization, got %+v", resolution)
	}
	if resolution.Artifact.Purpose != ring.Purpose {
		t.Fatalf("expected explicit ring artifact, got %q", resolution.Artifact.Purpose)
	}
}

func datasetArtifact(purpose string, columns []string, rows []map[string]any, createdAt time.Time) Artifact {
	return Artifact{
		Scope:        Scope{TenantID: 1, ProjectID: 2, SessionID: 3, UserID: 4},
		Purpose:      purpose,
		Kind:         ArtifactKindDataset,
		Status:       ArtifactStatusActive,
		Completeness: CompletenessComplete,
		Payload:      ArtifactPayload{Columns: columns, Rows: rows},
		Semantics:    inferSemantics(columns, rows),
		CreatedAt:    createdAt,
	}
}
