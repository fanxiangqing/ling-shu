from __future__ import annotations

from typing import Any, Dict, List, Tuple

import pandas as pd

from charts import first_category_column, first_time_column, numeric_columns, suggest_chart


def _records(df: pd.DataFrame, limit: int) -> List[Dict[str, Any]]:
    if df.empty:
        return []
    safe = df.head(limit).where(pd.notnull(df), None)
    return safe.to_dict(orient="records")


def _metric(name: str, label: str, value: float, unit: str = "") -> Dict[str, Any]:
    if float(value).is_integer():
        display = f"{int(value)}{unit}"
    else:
        display = f"{value:.4g}{unit}"
    return {"name": name, "label": label, "value": float(value), "unit": unit, "display": display}


def base_metrics(df: pd.DataFrame) -> List[Dict[str, Any]]:
    metrics = [_metric("row_count", "行数", len(df))]
    for col in numeric_columns(df)[:4]:
        series = pd.to_numeric(df[col], errors="coerce").dropna()
        if len(series) == 0:
            continue
        metrics.append(_metric(f"{col}_sum", f"{col} 合计", float(series.sum())))
        metrics.append(_metric(f"{col}_avg", f"{col} 平均", float(series.mean())))
    return metrics


def analyze_trend(name: str, df: pd.DataFrame, time_col: Any, value_col: Any, max_rows: int) -> Dict[str, Any]:
    if time_col not in df.columns or value_col not in df.columns:
        return {
            "success": False,
            "error": f"trend_analysis requires existing time_field and value_field, got time_field={time_col}, value_field={value_col}",
            "analysis_kind": "template_error",
            "template_name": "trend_analysis",
            "tables": [],
            "charts": [],
            "metrics": [],
            "warnings": [],
        }
    tmp = df[[time_col, value_col]].copy()
    tmp[value_col] = pd.to_numeric(tmp[value_col], errors="coerce")
    grouped = tmp.dropna(subset=[value_col]).groupby(time_col, dropna=False)[value_col].sum().reset_index()
    grouped = grouped.sort_values(by=time_col)
    rows = _records(grouped, max_rows)
    summary = f"{name} 返回 {len(df)} 行数据，已按 {time_col} 汇总 {value_col} 趋势。"
    return {
        "analysis_kind": "trend",
        "template_name": "trend_analysis",
        "summary": summary,
        "tables": [{"name": "趋势汇总", "columns": [str(time_col), str(value_col)], "rows": rows}],
        "charts": [suggest_chart(rows, [str(time_col), str(value_col)])],
        "metrics": base_metrics(df),
        "warnings": [],
    }


def analyze_category(name: str, df: pd.DataFrame, category_col: Any, value_col: Any, max_rows: int) -> Dict[str, Any]:
    if category_col not in df.columns or value_col not in df.columns:
        return {
            "success": False,
            "error": f"category_analysis requires existing category_field and value_field, got category_field={category_col}, value_field={value_col}",
            "analysis_kind": "template_error",
            "template_name": "category_analysis",
            "tables": [],
            "charts": [],
            "metrics": [],
            "warnings": [],
        }
    tmp = df[[category_col, value_col]].copy()
    tmp[value_col] = pd.to_numeric(tmp[value_col], errors="coerce")
    grouped = tmp.dropna(subset=[value_col]).groupby(category_col, dropna=False)[value_col].sum().reset_index()
    grouped = grouped.sort_values(by=value_col, ascending=False)
    grouped["占比"] = grouped[value_col] / grouped[value_col].sum() if grouped[value_col].sum() else 0
    rows = _records(grouped, max_rows)
    summary = f"{name} 返回 {len(df)} 行数据，已按 {category_col} 汇总 {value_col} 并计算占比。"
    return {
        "analysis_kind": "category",
        "template_name": "category_analysis",
        "summary": summary,
        "tables": [{"name": "分类汇总", "columns": [str(category_col), str(value_col), "占比"], "rows": rows}],
        "charts": [suggest_chart(rows, [str(category_col), str(value_col), "占比"])],
        "metrics": base_metrics(df),
        "warnings": [],
    }


def analyze_stats(name: str, df: pd.DataFrame, max_rows: int) -> Dict[str, Any]:
    nums = numeric_columns(df)
    stats_rows: List[Dict[str, Any]] = []
    for col in nums:
        series = pd.to_numeric(df[col], errors="coerce").dropna()
        if len(series) == 0:
            continue
        stats_rows.append({
            "字段": str(col),
            "非空数": int(series.count()),
            "合计": float(series.sum()),
            "平均": float(series.mean()),
            "最小值": float(series.min()),
            "最大值": float(series.max()),
            "标准差": float(series.std()) if len(series) > 1 else 0.0,
        })
    table = {"name": "统计摘要", "columns": ["字段", "非空数", "合计", "平均", "最小值", "最大值", "标准差"], "rows": stats_rows[:max_rows]}
    summary = f"{name} 返回 {len(df)} 行数据，已生成数值字段统计摘要。" if stats_rows else f"{name} 返回 {len(df)} 行数据，未识别到可聚合的数值字段。"
    return {
        "analysis_kind": "stats",
        "template_name": "descriptive_stats",
        "summary": summary,
        "tables": [table] if stats_rows else [{"name": name, "columns": [str(c) for c in df.columns], "rows": _records(df, max_rows)}],
        "charts": [suggest_chart(stats_rows, table["columns"])] if stats_rows else [],
        "metrics": base_metrics(df),
        "warnings": [],
    }


def analyze_single(name: str, df: pd.DataFrame, question: str, max_rows: int) -> Dict[str, Any]:
    columns = [str(c) for c in df.columns]
    if df.empty:
        return {
            "analysis_kind": "empty",
            "summary": f"{name} 没有返回数据。",
            "tables": [{"name": name, "columns": columns, "rows": []}],
            "charts": [],
            "metrics": [_metric("row_count", "行数", 0)],
            "warnings": ["输入结果集为空"],
        }

    nums = numeric_columns(df)
    time_col = first_time_column(df)
    category_col = first_category_column(df, nums, time_col)

    if time_col and nums:
        return analyze_trend(name, df, time_col, nums[0], max_rows)

    if category_col and nums:
        return analyze_category(name, df, category_col, nums[0], max_rows)

    return analyze_stats(name, df, max_rows)


def analyze_multiple(datasets: List[Tuple[str, pd.DataFrame]], question: str, max_rows: int) -> Dict[str, Any]:
    overview_rows: List[Dict[str, Any]] = []
    concat_frames: List[pd.DataFrame] = []
    common_columns = set(datasets[0][1].columns) if datasets else set()
    for _, df in datasets[1:]:
        common_columns &= set(df.columns)

    for name, df in datasets:
        nums = numeric_columns(df)
        row: Dict[str, Any] = {"数据集": name, "行数": int(len(df)), "字段数": int(len(df.columns))}
        for col in nums[:3]:
            series = pd.to_numeric(df[col], errors="coerce").dropna()
            if len(series):
                row[f"{col}_合计"] = float(series.sum())
        overview_rows.append(row)
        if common_columns:
            part = df[list(common_columns)].copy()
            part.insert(0, "数据集", name)
            concat_frames.append(part)

    overview_columns = list(overview_rows[0].keys()) if overview_rows else ["数据集", "行数", "字段数"]
    tables = [{"name": "多数据集概览", "columns": overview_columns, "rows": overview_rows[:max_rows]}]
    if concat_frames:
        combined = pd.concat(concat_frames, ignore_index=True)
        tables.append({"name": "共同字段合并结果", "columns": [str(c) for c in combined.columns], "rows": _records(combined, max_rows)})

    summary = f"已分析 {len(datasets)} 个结果集，并生成行数、字段数和主要数值合计对比。"
    charts = [suggest_chart(overview_rows, overview_columns)] if overview_rows else []
    metrics = [_metric("dataset_count", "结果集数量", len(datasets)), _metric("total_rows", "总行数", sum(len(df) for _, df in datasets))]
    return {
        "analysis_kind": "multi_dataset",
        "template_name": "multi_dataset_overview",
        "summary": summary,
        "tables": tables,
        "charts": charts,
        "metrics": metrics,
        "warnings": [],
    }
