package query

import "testing"

func TestSuggestChartLine(t *testing.T) {
	chart := SuggestChart([]string{"date", "sales"}, []map[string]any{
		{"date": "2026-06-01", "sales": 100},
		{"date": "2026-06-02", "sales": 120},
	})
	if chart.Type != ChartLine || chart.XField != "date" || chart.YFields[0] != "sales" {
		t.Fatalf("unexpected chart: %+v", chart)
	}
}

func TestSuggestChartPieForSmallCategory(t *testing.T) {
	chart := SuggestChart([]string{"province", "amount"}, []map[string]any{
		{"province": "上海", "amount": "100.5"},
		{"province": "北京", "amount": "88"},
	})
	if chart.Type != ChartPie || chart.NameField != "province" || chart.ValueField != "amount" {
		t.Fatalf("unexpected chart: %+v", chart)
	}
}

func TestSuggestChartBarForRanking(t *testing.T) {
	chart := SuggestChart([]string{"product_name", "sales_amount"}, []map[string]any{
		{"product_name": "商品1", "sales_amount": 100},
		{"product_name": "商品2", "sales_amount": 90},
		{"product_name": "商品3", "sales_amount": 80},
		{"product_name": "商品4", "sales_amount": 70},
		{"product_name": "商品5", "sales_amount": 60},
		{"product_name": "商品6", "sales_amount": 50},
		{"product_name": "商品7", "sales_amount": 40},
		{"product_name": "商品8", "sales_amount": 30},
		{"product_name": "商品9", "sales_amount": 20},
	})
	if chart.Type != ChartBar || chart.XField != "product_name" || chart.YFields[0] != "sales_amount" {
		t.Fatalf("unexpected chart: %+v", chart)
	}
}

func TestSuggestChartFunnelForStageRows(t *testing.T) {
	chart := SuggestChart([]string{"stage", "user_count"}, []map[string]any{
		{"stage": "访问", "user_count": 1000},
		{"stage": "注册", "user_count": 300},
		{"stage": "下单", "user_count": 120},
		{"stage": "支付", "user_count": 80},
	})
	if chart.Type != ChartFunnel || chart.NameField != "stage" || chart.ValueField != "user_count" {
		t.Fatalf("unexpected chart: %+v", chart)
	}
}

func TestSuggestChartRadarForSingleRowMultiMetric(t *testing.T) {
	chart := SuggestChart([]string{"sales", "orders", "users"}, []map[string]any{
		{"sales": 1000, "orders": 80, "users": 60},
	})
	if chart.Type != ChartRadar || len(chart.YFields) != 3 {
		t.Fatalf("unexpected chart: %+v", chart)
	}
}

func TestSuggestChartTableForEmptyRows(t *testing.T) {
	chart := SuggestChart([]string{"name"}, nil)
	if chart.Type != ChartTable {
		t.Fatalf("unexpected chart: %+v", chart)
	}
}
