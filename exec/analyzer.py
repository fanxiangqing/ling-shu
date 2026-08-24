from __future__ import annotations

import hashlib
import json
import logging
from typing import Any, Dict, List, Tuple

import pandas as pd

from templates import analyze_category, analyze_multiple, analyze_single, analyze_stats, analyze_trend

logger = logging.getLogger("ling-shu-exec.analyzer")


def _dataframe(dataset: Dict[str, Any], max_rows: int) -> pd.DataFrame:
    rows = list(dataset.get("rows") or [])[:max_rows]
    columns = [str(c) for c in dataset.get("columns") or []]
    if columns:
        return pd.DataFrame(rows, columns=columns)
    return pd.DataFrame(rows)


def _observation(result: Dict[str, Any]) -> str:
    lines = []
    if result.get("summary"):
        lines.append("Python 分析摘要：" + result["summary"])
    metrics = result.get("metrics") or []
    if metrics:
        display = "；".join(f"{m.get('label') or m.get('name')}={m.get('display')}" for m in metrics[:8])
        lines.append("关键指标：" + display)
    tables = result.get("tables") or []
    if tables:
        lines.append("输出表：" + "，".join(t.get("name", "table") for t in tables[:4]))
    warnings = result.get("warnings") or []
    if warnings:
        lines.append("警告：" + "；".join(warnings[:4]))
    return "\n".join(lines)


def _template_params(request: Dict[str, Any]) -> Dict[str, Any]:
    return dict(request.get("template_params") or {})


def _selected_frame(request: Dict[str, Any], frames: List[Tuple[str, pd.DataFrame]]) -> Tuple[str, pd.DataFrame]:
    params = _template_params(request)
    if not frames:
        return "", pd.DataFrame()
    dataset_name = str(params.get("dataset_name") or params.get("dataset") or "").strip()
    if dataset_name:
        for name, frame in frames:
            if name == dataset_name:
                return name, frame
    try:
        index = int(params.get("dataset_index", 0))
    except (TypeError, ValueError):
        index = 0
    if index < 0 or index >= len(frames):
        index = 0
    return frames[index]


def _run_code(request: Dict[str, Any], frames: List[Tuple[str, pd.DataFrame]], max_rows: int) -> Dict[str, Any]:
    params = _template_params(request)
    code = str(params.get("code") or request.get("analysis_goal") or "")
    if not code.strip():
        return {
            "success": False,
            "error": "code mode requires analysis_goal or template_params.code",
            "analysis_kind": "code",
            "template_name": "",
        }
    digest = hashlib.sha256(code.encode("utf-8")).hexdigest()
    frame_copies = [(name, df.copy()) for name, df in frames]
    namespace = {
        "pd": pd,
        "df": frame_copies[0][1] if frame_copies else pd.DataFrame(),
        "frames": [df for _, df in frame_copies],
        "dataset_names": [name for name, _ in frame_copies],
        "datasets": {name: df for name, df in frame_copies},
        "result": None,
    }
    safe_builtins = {
        "abs": abs,
        "all": all,
        "any": any,
        "bool": bool,
        "dict": dict,
        "enumerate": enumerate,
        "float": float,
        "int": int,
        "len": len,
        "list": list,
        "max": max,
        "min": min,
        "range": range,
        "round": round,
        "set": set,
        "sorted": sorted,
        "str": str,
        "sum": sum,
        "tuple": tuple,
    }
    exec(compile(code, "<analysis_code>", "exec"), {"__builtins__": safe_builtins}, namespace)
    value = namespace.get("result")
    if isinstance(value, pd.DataFrame):
        rows = value.head(max_rows).where(pd.notnull(value), None).to_dict(orient="records")
        return {
            "success": True,
            "summary": "已执行一次性 Python 分析代码并返回 result DataFrame。",
            "observation": "Python code result DataFrame",
            "tables": [{"name": "code_result", "columns": [str(c) for c in value.columns], "rows": rows}],
            "charts": [],
            "metrics": [],
            "warnings": [],
            "analysis_kind": "code",
            "code_hash": digest,
        }
    return {
        "success": True,
        "summary": f"已执行一次性 Python 分析代码，result={value!r}",
        "observation": f"Python code result: {value!r}",
        "tables": [],
        "charts": [],
        "metrics": [],
        "warnings": [],
        "analysis_kind": "code",
        "code_hash": digest,
    }


def _run_template(request: Dict[str, Any], frames: List[Tuple[str, pd.DataFrame]], max_rows: int) -> Dict[str, Any]:
    params = _template_params(request)
    template_name = str(request.get("template_name") or params.get("template_name") or request.get("analysis_goal") or "").strip()
    template_name = template_name or "auto"
    normalized = template_name.lower()
    name, frame = _selected_frame(request, frames)
    if normalized in {"auto", "auto_detect", "single_auto", "analyze_single"}:
        return analyze_single(name, frame, request.get("question") or "", max_rows)
    if normalized in {"multi", "multi_dataset", "multi_dataset_overview"}:
        return analyze_multiple(frames, request.get("question") or "", max_rows)
    if normalized in {"trend", "trend_analysis"}:
        time_field = params.get("time_field") or params.get("x_field")
        value_field = params.get("value_field") or params.get("y_field")
        return analyze_trend(name, frame, time_field, value_field, max_rows)
    if normalized in {"category", "category_analysis"}:
        category_field = params.get("category_field") or params.get("name_field") or params.get("x_field")
        value_field = params.get("value_field") or params.get("y_field")
        return analyze_category(name, frame, category_field, value_field, max_rows)
    if normalized in {"stats", "descriptive_stats", "summary_stats"}:
        return analyze_stats(name, frame, max_rows)
    return {
        "success": False,
        "error": f"unknown template_name: {template_name}",
        "analysis_kind": "template_error",
        "template_name": template_name,
        "tables": [],
        "charts": [],
        "metrics": [],
        "warnings": [],
    }


def analyze_request(request: Dict[str, Any]) -> Dict[str, Any]:
    trace = dict(request.get("trace") or {})
    request_id = trace.get("request_id") or request.get("request_id") or ""
    tenant_id = trace.get("tenant_id") or request.get("tenant_id") or 0
    project_id = trace.get("project_id") or request.get("project_id") or 0
    session_id = trace.get("session_id") or request.get("session_id") or 0
    user_id = trace.get("user_id") or request.get("user_id") or 0
    limits = request.get("limits") or {}
    max_input_rows = int(limits.get("max_input_rows") or 5000)
    max_output_rows = int(limits.get("max_output_rows") or 1000)
    datasets = request.get("datasets") or []
    if not datasets:
        return {"success": False, "error": "no datasets provided", "analysis_kind": "empty"}

    frames: List[Tuple[str, pd.DataFrame]] = []
    warnings: List[str] = []
    input_rows = 0
    for index, dataset in enumerate(datasets):
        name = dataset.get("name") or dataset.get("datasource_name") or f"dataset_{index + 1}"
        rows = dataset.get("rows") or []
        input_rows += len(rows)
        if len(rows) > max_input_rows:
            warnings.append(f"{name} 输入行数超过上限，已截断到 {max_input_rows} 行")
        frames.append((name, _dataframe(dataset, max_input_rows)))

    mode = (request.get("mode") or "auto").strip().lower()
    logger.info(
        "analyzer prepared frames request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s datasets=%d input_rows=%d mode=%s max_input_rows=%d max_output_rows=%d",
        request_id,
        tenant_id,
        project_id,
        session_id,
        user_id,
        len(frames),
        input_rows,
        mode,
        max_input_rows,
        max_output_rows,
    )
    if mode == "code":
        result = _run_code(request, frames, max_output_rows)
    elif mode == "template":
        result = _run_template(request, frames, max_output_rows)
    elif len(frames) > 1:
        result = analyze_multiple(frames, request.get("question") or "", max_output_rows)
    else:
        name, frame = frames[0]
        result = analyze_single(name, frame, request.get("question") or "", max_output_rows)

    result.setdefault("success", True)
    result.setdefault("warnings", [])
    result["warnings"] = warnings + list(result.get("warnings") or [])
    result["input_row_count"] = input_rows
    result["output_row_count"] = sum(len(t.get("rows") or []) for t in result.get("tables") or [])
    result["observation"] = result.get("observation") or _observation(result)
    logger.info(
        "analyzer finished request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s success=%s kind=%s template=%s output_rows=%d warnings=%d",
        request_id,
        tenant_id,
        project_id,
        session_id,
        user_id,
        result.get("success"),
        result.get("analysis_kind", ""),
        result.get("template_name", ""),
        result["output_row_count"],
        len(result.get("warnings") or []),
    )
    return result
