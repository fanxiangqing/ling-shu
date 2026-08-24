from __future__ import annotations

import re
from typing import Any, Dict, List, Optional

import pandas as pd

DATE_HINT = re.compile(r"\d|[-/:年月日]")


def numeric_columns(df: pd.DataFrame) -> List[str]:
    return [c for c in df.columns if pd.api.types.is_numeric_dtype(df[c])]


def first_time_column(df: pd.DataFrame) -> Optional[str]:
    for col in df.columns:
        lower = str(col).lower()
        if any(token in lower for token in ("date", "time", "day", "month", "year", "日期", "时间", "月份", "年度")):
            return col
        if df[col].dtype == "object":
            sample = df[col].dropna().astype(str).head(8)
            if len(sample) > 0 and sample.map(lambda value: bool(DATE_HINT.search(value))).mean() >= 0.75:
                parsed = pd.to_datetime(sample, errors="coerce")
                if parsed.notna().mean() >= 0.75:
                    return col
    return None


def first_category_column(df: pd.DataFrame, numeric: List[str], time_col: Optional[str]) -> Optional[str]:
    numeric_set = set(numeric)
    for col in df.columns:
        if col == time_col or col in numeric_set:
            continue
        return col
    return None


def suggest_chart(rows: List[Dict[str, Any]], columns: List[str]) -> Dict[str, Any]:
    if not rows or not columns:
        return {"type": "table", "reason": "结果为空，默认展示表格"}
    df = pd.DataFrame(rows, columns=columns)
    numeric = numeric_columns(df)
    time_col = first_time_column(df)
    category_col = first_category_column(df, numeric, time_col)
    if time_col and numeric:
        return {
            "type": "line",
            "title": "趋势分析",
            "x_field": str(time_col),
            "y_fields": [str(numeric[0])],
            "reason": "包含时间字段和数值指标，适合展示趋势",
            "rows": rows,
        }
    if category_col and numeric:
        chart_type = "pie" if len(rows) <= 8 else "bar"
        return {
            "type": chart_type,
            "title": "分类对比",
            "x_field": str(category_col),
            "y_fields": [str(numeric[0])],
            "name_field": str(category_col),
            "value_field": str(numeric[0]),
            "reason": "包含分类字段和数值指标，适合比较大小或占比",
            "rows": rows,
        }
    return {"type": "table", "title": "结果表格", "reason": "未识别出稳定的分类或时间维度，默认展示表格", "rows": rows}
