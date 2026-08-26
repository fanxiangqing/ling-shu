package memory

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var limitPattern = regexp.MustCompile(`(?i)\blimit\s+(\d+)`)

func ExtractArtifacts(input TurnInput) []Artifact {
	if !input.Scope.Valid() {
		return nil
	}
	artifacts := make([]Artifact, 0, len(input.Executions))
	for _, execution := range input.Executions {
		if !successfulExecution(execution) || len(execution.Columns) == 0 || len(execution.Rows) == 0 {
			continue
		}
		purpose := strings.TrimSpace(execution.Purpose)
		if purpose == "" {
			purpose = strings.TrimSpace(input.Question)
		}
		artifact := Artifact{
			Scope:           input.Scope,
			SourceMessageID: input.SourceMessageID,
			Purpose:         purpose,
			Kind:            ArtifactKindDataset,
			Status:          ArtifactStatusActive,
			Completeness:    executionCompleteness(execution),
			Payload: ArtifactPayload{
				Columns: append([]string(nil), execution.Columns...),
				Rows:    copyRows(execution.Rows),
				Chart:   execution.Chart,
				Answer:  strings.TrimSpace(execution.Answer),
			},
			Semantics: inferSemantics(execution.Columns, execution.Rows),
			Lineage: ArtifactLineage{
				QueryExecutionIDs: nonZeroUint64(execution.QueryExecutionID),
				DatasourceIDs:     nonZeroUint64(execution.DatasourceID),
			},
			CreatedAt: time.Now(),
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func successfulExecution(execution ExecutionSnapshot) bool {
	status := strings.ToLower(strings.TrimSpace(execution.Status))
	return status == "" || status == "success" || status == "completed"
}

func executionCompleteness(execution ExecutionSnapshot) string {
	if execution.RowCount > len(execution.Rows) {
		return CompletenessPreview
	}
	if execution.Limit > 0 {
		if len(execution.Rows) < execution.Limit {
			return CompletenessComplete
		}
		return CompletenessBounded
	}
	match := limitPattern.FindStringSubmatch(execution.SQL)
	if len(match) == 2 {
		limit, err := strconv.Atoi(match[1])
		if err == nil && len(execution.Rows) < limit {
			return CompletenessComplete
		}
		return CompletenessBounded
	}
	return CompletenessComplete
}

func inferSemantics(columns []string, rows []map[string]any) ArtifactSemantics {
	numeric := numericColumns(columns, rows)
	numericSet := make(map[string]bool, len(numeric))
	measures := make([]Measure, 0, len(numeric))
	for _, field := range numeric {
		numericSet[field] = true
		measures = append(measures, Measure{
			Field:       field,
			Label:       field,
			Aggregation: inferAggregation(field),
			Unit:        inferUnit(field),
		})
	}
	dimensions := make([]string, 0, len(columns)-len(numeric))
	for _, column := range columns {
		if !numericSet[column] {
			dimensions = append(dimensions, column)
		}
	}
	return ArtifactSemantics{Dimensions: dimensions, Measures: measures}
}

func numericColumns(columns []string, rows []map[string]any) []string {
	out := make([]string, 0, len(columns))
	for _, column := range columns {
		checked := 0
		numeric := 0
		for _, row := range rows {
			value, ok := row[column]
			if !ok || value == nil {
				continue
			}
			checked++
			if isNumeric(value) {
				numeric++
			}
		}
		if checked > 0 && checked == numeric {
			out = append(out, column)
		}
	}
	return out
}

func isNumeric(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case string:
		_, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return err == nil
	default:
		return false
	}
}

func inferAggregation(field string) string {
	name := strings.ToLower(field)
	if containsAny(name, "count", "数量", "总数", "个数") {
		return "count"
	}
	if containsAny(name, "avg", "average", "平均") {
		return "avg"
	}
	if containsAny(name, "sum", "amount", "金额", "总额") {
		return "sum"
	}
	if containsAny(name, "max", "最大", "最高") {
		return "max"
	}
	if containsAny(name, "min", "最小", "最低") {
		return "min"
	}
	return "value"
}

func inferUnit(field string) string {
	name := strings.ToLower(field)
	switch {
	case containsAny(name, "ring", "环号", "环数", "掘进环"):
		return "环"
	case containsAny(name, "rate", "ratio", "percent", "占比", "比例", "率"):
		return "%"
	case containsAny(name, "amount", "revenue", "sales", "金额", "收入", "销售额"):
		return "元"
	case containsAny(name, "count", "数量", "总数", "个数"):
		return "个"
	default:
		return ""
	}
}

func bestDimension(artifact Artifact) string {
	type candidate struct {
		field string
		score int
	}
	candidates := make([]candidate, 0, len(artifact.Semantics.Dimensions))
	for index, field := range artifact.Semantics.Dimensions {
		distinct := distinctCount(artifact.Payload.Rows, field)
		if distinct == 0 {
			continue
		}
		score := min(distinct, 20) - index
		name := strings.ToLower(field)
		if containsAny(name, "类型", "类别", "等级", "状态", "名称", "name", "type", "level", "status") {
			score += 20
		}
		if distinct > 1 {
			score += 15
		}
		if distinct > 40 {
			score -= 20
		}
		candidates = append(candidates, candidate{field: field, score: score})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	return candidates[0].field
}

func distinctCount(rows []map[string]any, field string) int {
	seen := map[string]bool{}
	for _, row := range rows {
		value, ok := row[field]
		if !ok || value == nil {
			continue
		}
		seen[fmt.Sprint(value)] = true
	}
	return len(seen)
}

func copyRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		copied := make(map[string]any, len(row))
		for key, value := range row {
			copied[key] = value
		}
		out = append(out, copied)
	}
	return out
}

func nonZeroUint64(value uint64) []uint64 {
	if value == 0 {
		return nil
	}
	return []uint64{value}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
