package query

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	ChartTable  = "table"
	ChartLine   = "line"
	ChartBar    = "bar"
	ChartPie    = "pie"
	ChartRadar  = "radar"
	ChartFunnel = "funnel"
)

type ChartSuggestion struct {
	Type       string   `json:"type"`
	XField     string   `json:"x_field,omitempty"`
	YFields    []string `json:"y_fields,omitempty"`
	NameField  string   `json:"name_field,omitempty"`
	ValueField string   `json:"value_field,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

func SuggestChart(columns []string, rows []map[string]any) ChartSuggestion {
	if len(columns) == 0 || len(rows) == 0 {
		return ChartSuggestion{Type: ChartTable, Reason: "结果为空，默认展示表格"}
	}

	numericFields := numericColumns(columns, rows)
	timeField := firstTimeColumn(columns, rows)
	categoryField := firstCategoryColumn(columns, numericFields, timeField)

	if timeField != "" && len(numericFields) > 0 {
		return ChartSuggestion{
			Type:    ChartLine,
			XField:  timeField,
			YFields: numericFields[:1],
			Reason:  "包含时间字段和数值指标，适合展示趋势",
		}
	}
	if categoryField != "" && len(numericFields) > 0 {
		if looksLikeFunnel(categoryField, rows) {
			return ChartSuggestion{
				Type:       ChartFunnel,
				NameField:  categoryField,
				ValueField: numericFields[0],
				Reason:     "分类字段呈现阶段/步骤语义，适合漏斗图",
			}
		}
		if len(rows) <= 8 {
			return ChartSuggestion{
				Type:       ChartPie,
				NameField:  categoryField,
				ValueField: numericFields[0],
				Reason:     "分类数量较少且只有一个主指标，适合占比展示",
			}
		}
		return ChartSuggestion{
			Type:    ChartBar,
			XField:  categoryField,
			YFields: numericFields[:1],
			Reason:  "包含分类字段和数值指标，适合比较大小",
		}
	}
	if len(numericFields) >= 3 && len(rows) == 1 {
		return ChartSuggestion{
			Type:    ChartRadar,
			YFields: numericFields,
			Reason:  "单行多指标结果，适合雷达图比较多个指标",
		}
	}
	return ChartSuggestion{Type: ChartTable, Reason: "未识别出稳定的分类或时间维度，默认展示表格"}
}

func numericColumns(columns []string, rows []map[string]any) []string {
	var out []string
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
		if checked > 0 && numeric == checked {
			out = append(out, column)
		}
	}
	return out
}

func firstTimeColumn(columns []string, rows []map[string]any) string {
	for _, column := range columns {
		name := strings.ToLower(column)
		if strings.Contains(name, "date") || strings.Contains(name, "time") || strings.Contains(name, "day") || strings.Contains(name, "month") {
			return column
		}
		for _, row := range rows {
			if value, ok := row[column].(string); ok && looksLikeTime(value) {
				return column
			}
		}
	}
	return ""
}

func firstCategoryColumn(columns []string, numericFields []string, timeField string) string {
	numericSet := map[string]bool{}
	for _, field := range numericFields {
		numericSet[field] = true
	}
	for _, column := range columns {
		if column == timeField || numericSet[column] {
			continue
		}
		return column
	}
	return ""
}

func isNumeric(value any) bool {
	switch typed := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	case string:
		if strings.TrimSpace(typed) == "" {
			return false
		}
		_, err := strconv.ParseFloat(typed, 64)
		return err == nil
	default:
		return false
	}
}

func looksLikeTime(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01",
		"2006/01/02",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func looksLikeFunnel(categoryField string, rows []map[string]any) bool {
	name := strings.ToLower(categoryField)
	if strings.Contains(name, "stage") || strings.Contains(name, "step") || strings.Contains(name, "funnel") {
		return true
	}
	keywords := []string{"访问", "注册", "下单", "支付", "转化", "线索", "成交"}
	matches := 0
	for _, row := range rows {
		value := fmt.Sprint(row[categoryField])
		for _, keyword := range keywords {
			if strings.Contains(value, keyword) {
				matches++
				break
			}
		}
	}
	return matches >= int(math.Min(float64(len(rows)), 3))
}
