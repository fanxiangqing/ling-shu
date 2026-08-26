package memory

import (
	"fmt"
	"sort"
	"strings"

	"ling-shu/internal/query"
)

type scoredArtifact struct {
	artifact Artifact
	score    int
}

func ResolveFollowUp(question string, state SessionState, artifacts []Artifact) Resolution {
	chartType, requested := requestedChartType(question)
	if !requested {
		return Resolution{Action: ActionNone}
	}
	candidates := scoreArtifacts(question, state, artifacts)
	if len(candidates) == 0 {
		return Resolution{Action: ActionNone, Reason: "当前会话没有可复用的数据产物"}
	}
	topIsFocus := candidates[0].artifact.ID > 0 && candidates[0].artifact.ID == state.FocusArtifactID
	if len(candidates) > 1 && candidates[0].score-candidates[1].score < 5 && !topIsFocus {
		return Resolution{
			Action:     ActionClarify,
			Confidence: 0.55,
			Reason:     "存在多个同等相关的数据产物",
			Answer:     fmt.Sprintf("你想展示“%s”还是“%s”？", artifactLabel(candidates[0].artifact), artifactLabel(candidates[1].artifact)),
		}
	}
	transformed, ok := transformForChart(candidates[0].artifact, chartType)
	if !ok {
		return Resolution{Action: ActionNone, Reason: "最近结果不足以安全完成本地可视化转换"}
	}
	label := chartDisplayName(chartType)
	return Resolution{
		Action:     ActionVisualize,
		Confidence: minFloat(0.99, 0.6+float64(candidates[0].score)/200),
		Reason:     "命中当前会话最近的可视化数据产物",
		Answer:     fmt.Sprintf("已基于上一轮“%s”的结果生成%s。", artifactLabel(transformed), label),
		Artifact:   &transformed,
	}
}

func requestedChartType(question string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(question))
	switch {
	case containsAny(value, "柱状图", "柱形图", "bar chart", "barchart"):
		return query.ChartBar, true
	case containsAny(value, "折线图", "趋势图", "line chart"):
		return query.ChartLine, true
	case containsAny(value, "饼图", "环形图", "pie chart"):
		return query.ChartPie, true
	case containsAny(value, "表格", "明细表") && containsAny(value, "展示", "换成", "改成"):
		return query.ChartTable, true
	case containsAny(value, "画图", "图表", "可视化", "做个图", "制作一个图"):
		return "", true
	default:
		return "", false
	}
}

func scoreArtifacts(question string, state SessionState, artifacts []Artifact) []scoredArtifact {
	out := make([]scoredArtifact, 0, len(artifacts))
	for index, artifact := range artifacts {
		if artifact.Kind != ArtifactKindDataset || artifact.Status != ArtifactStatusActive || len(artifact.Payload.Rows) == 0 {
			continue
		}
		score := max(0, 12-index)
		if artifact.ID > 0 && artifact.ID == state.FocusArtifactID {
			score += 45
		}
		if len(artifact.Payload.Rows) > 1 {
			score += 35
		}
		if len(artifact.Semantics.Dimensions) > 0 {
			score += 20
		}
		if len(artifact.Semantics.Measures) > 0 {
			score += 15
		}
		if artifact.Completeness == CompletenessComplete {
			score += 10
		}
		searchable := strings.ToLower(artifact.Purpose + " " + strings.Join(artifact.Payload.Columns, " "))
		for _, token := range meaningfulTokens(question) {
			if strings.Contains(searchable, token) {
				score += 100
			}
		}
		out = append(out, scoredArtifact{artifact: artifact, score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			return out[i].artifact.CreatedAt.After(out[j].artifact.CreatedAt)
		}
		return out[i].score > out[j].score
	})
	return out
}

func meaningfulTokens(question string) []string {
	value := strings.ToLower(question)
	for _, noise := range []string{"请", "帮我", "给我", "把", "制作", "做成", "做个", "一个", "一下", "展示", "换成", "改成", "柱状图", "柱形图", "折线图", "趋势图", "饼图", "环形图", "图表", "可视化"} {
		value = strings.ReplaceAll(value, noise, " ")
	}
	fields := strings.Fields(value)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) >= 2 {
			out = append(out, field)
		}
	}
	return out
}

func transformForChart(source Artifact, requestedType string) (Artifact, bool) {
	artifact := source
	artifact.ID = 0
	artifact.SourceMessageID = 0
	artifact.Lineage.DerivedFromID = source.ID
	artifact.CreatedAt = source.CreatedAt
	artifact.Payload.Columns = append([]string(nil), source.Payload.Columns...)
	artifact.Payload.Rows = copyRows(source.Payload.Rows)

	if requestedType == query.ChartTable {
		artifact.Payload.Chart = query.ChartSuggestion{Type: query.ChartTable, Reason: "用户要求以表格展示最近结果"}
		return artifact, true
	}

	dimension := bestDimension(source)
	dimension, measure, rows, columns, semantics, ok := chartSeries(source, dimension)
	if !ok {
		return Artifact{}, false
	}
	artifact.Payload.Rows = rows
	artifact.Payload.Columns = columns
	artifact.Semantics = semantics
	if requestedType == "" {
		artifact.Payload.Chart = query.SuggestChart(columns, rows)
		return artifact, true
	}
	switch requestedType {
	case query.ChartPie:
		artifact.Payload.Chart = query.ChartSuggestion{
			Type:       query.ChartPie,
			NameField:  dimension,
			ValueField: measure,
			Reason:     "用户要求将最近结果按饼图展示",
		}
	case query.ChartLine:
		artifact.Payload.Chart = query.ChartSuggestion{
			Type:    query.ChartLine,
			XField:  dimension,
			YFields: []string{measure},
			Reason:  "用户要求将最近结果按折线图展示",
		}
	default:
		artifact.Payload.Chart = query.ChartSuggestion{
			Type:    query.ChartBar,
			XField:  dimension,
			YFields: []string{measure},
			Reason:  "用户要求将最近结果按柱状图展示",
		}
	}
	return artifact, true
}

func chartSeries(source Artifact, dimension string) (string, string, []map[string]any, []string, ArtifactSemantics, bool) {
	if dimension != "" && len(source.Semantics.Measures) > 0 {
		measure := source.Semantics.Measures[0]
		return dimension, measure.Field, copyRows(source.Payload.Rows), append([]string(nil), source.Payload.Columns...), source.Semantics, true
	}
	if dimension != "" {
		if source.Completeness != CompletenessComplete {
			return "", "", nil, nil, ArtifactSemantics{}, false
		}
		rows := aggregateRowsByDimension(source.Payload.Rows, dimension)
		if len(rows) == 0 {
			return "", "", nil, nil, ArtifactSemantics{}, false
		}
		measure := "数量"
		return dimension, measure, rows, []string{dimension, measure}, ArtifactSemantics{
			Dimensions: []string{dimension},
			Measures:   []Measure{{Field: measure, Label: measure, Aggregation: "count", Unit: "条"}},
		}, true
	}
	if len(source.Payload.Rows) != 1 || len(source.Semantics.Measures) == 0 {
		return "", "", nil, nil, ArtifactSemantics{}, false
	}
	compatible := compatibleMeasures(source.Semantics.Measures)
	if len(compatible) == 0 {
		return "", "", nil, nil, ArtifactSemantics{}, false
	}
	dimension = "指标"
	measureField := "数值"
	rows := make([]map[string]any, 0, len(compatible))
	for _, measure := range compatible {
		rows = append(rows, map[string]any{dimension: measure.Label, measureField: source.Payload.Rows[0][measure.Field]})
	}
	unit := compatible[0].Unit
	return dimension, measureField, rows, []string{dimension, measureField}, ArtifactSemantics{
		Dimensions: []string{dimension},
		Measures:   []Measure{{Field: measureField, Label: measureField, Aggregation: "value", Unit: unit}},
	}, true
}

func aggregateRowsByDimension(rows []map[string]any, dimension string) []map[string]any {
	counts := map[string]int{}
	order := make([]string, 0)
	for _, row := range rows {
		value, ok := row[dimension]
		if !ok || value == nil {
			continue
		}
		key := fmt.Sprint(value)
		if _, exists := counts[key]; !exists {
			order = append(order, key)
		}
		counts[key]++
	}
	out := make([]map[string]any, 0, len(order))
	for _, key := range order {
		out = append(out, map[string]any{dimension: key, "数量": counts[key]})
	}
	return out
}

func compatibleMeasures(measures []Measure) []Measure {
	groups := map[string][]Measure{}
	order := make([]string, 0)
	for _, measure := range measures {
		key := measure.Unit
		if key == "" {
			key = "field:" + measure.Field
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], measure)
	}
	best := []Measure(nil)
	for _, key := range order {
		if len(groups[key]) > len(best) {
			best = groups[key]
		}
	}
	return best
}

func artifactLabel(artifact Artifact) string {
	label := strings.TrimSpace(artifact.Purpose)
	if label != "" {
		runes := []rune(label)
		if len(runes) > 60 {
			return string(runes[:60]) + "..."
		}
		return label
	}
	return "最近查询"
}

func chartDisplayName(chartType string) string {
	switch chartType {
	case query.ChartLine:
		return "折线图"
	case query.ChartPie:
		return "饼图"
	case query.ChartTable:
		return "表格"
	case "":
		return "图表"
	default:
		return "柱状图"
	}
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
