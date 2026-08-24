from analyzer import analyze_request


def test_analyze_single_category_result():
    result = analyze_request({
        "request_id": "rid-python",
        "trace": {
            "request_id": "rid-python",
            "tenant_id": 1,
            "project_id": 2,
            "session_id": 3,
            "user_id": 4,
        },
        "mode": "auto",
        "limits": {"max_input_rows": 100, "max_output_rows": 10},
        "datasets": [{
            "name": "orders",
            "columns": ["province", "amount"],
            "rows": [
                {"province": "浙江", "amount": 10},
                {"province": "浙江", "amount": 5},
                {"province": "江苏", "amount": 8},
            ],
        }],
    })

    assert result["success"] is True
    assert result["analysis_kind"] == "category"
    assert result["template_name"] == "category_analysis"
    assert result["input_row_count"] == 3
    assert result["output_row_count"] == 2
    assert result["tables"][0]["columns"] == ["province", "amount", "占比"]
    assert result["charts"][0]["type"] == "pie"


def test_analyze_multiple_result_sets():
    result = analyze_request({
        "mode": "auto",
        "limits": {"max_input_rows": 100, "max_output_rows": 10},
        "datasets": [
            {
                "name": "db_a",
                "columns": ["metric", "value"],
                "rows": [{"metric": "orders", "value": 10}],
            },
            {
                "name": "db_b",
                "columns": ["metric", "value"],
                "rows": [{"metric": "orders", "value": 8}],
            },
        ],
    })

    assert result["success"] is True
    assert result["analysis_kind"] == "multi_dataset"
    assert result["metrics"][0]["name"] == "dataset_count"
    assert result["input_row_count"] == 2


def test_template_mode_uses_named_template_and_params():
    result = analyze_request({
        "mode": "template",
        "template_name": "category_analysis",
        "template_params": {"category_field": "city", "value_field": "gmv"},
        "limits": {"max_input_rows": 100, "max_output_rows": 10},
        "datasets": [{
            "name": "orders",
            "columns": ["city", "gmv"],
            "rows": [
                {"city": "杭州", "gmv": 20},
                {"city": "杭州", "gmv": 10},
                {"city": "南京", "gmv": 5},
            ],
        }],
    })

    assert result["success"] is True
    assert result["analysis_kind"] == "category"
    assert result["template_name"] == "category_analysis"
    assert result["tables"][0]["rows"][0]["city"] == "杭州"
    assert result["tables"][0]["rows"][0]["gmv"] == 30


def test_code_mode_runs_one_off_code_without_service_switch():
    result = analyze_request({
        "mode": "code",
        "template_params": {
            "code": "result = df.groupby('city', as_index=False)['gmv'].sum()"
        },
        "limits": {"max_input_rows": 100, "max_output_rows": 10},
        "datasets": [{
            "name": "orders",
            "columns": ["city", "gmv"],
            "rows": [
                {"city": "杭州", "gmv": 20},
                {"city": "杭州", "gmv": 10},
                {"city": "南京", "gmv": 5},
            ],
        }],
    })

    assert result["success"] is True
    assert result["analysis_kind"] == "code"
    assert result["code_hash"]
    assert result["tables"][0]["columns"] == ["city", "gmv"]
    assert result["output_row_count"] == 2
