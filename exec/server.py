from __future__ import annotations

import logging
import time
from concurrent import futures
from typing import Any, Dict

import grpc

from config import CONFIG
from pb import result_analysis_pb2, result_analysis_pb2_grpc
from sandbox import run_in_sandbox
from schemas import rows_from_proto, rows_to_proto, struct_to_dict

VERSION = "0.1.0"

logging.basicConfig(
    level=getattr(logging, CONFIG.log_level.upper(), logging.INFO),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("ling-shu-exec")


def _metadata(context) -> Dict[str, str]:
    return {item.key: item.value for item in context.invocation_metadata() or []}


def _request_to_dict(request: result_analysis_pb2.AnalyzeResultSetsRequest) -> Dict[str, Any]:
    limits = request.limits
    return {
        "request_id": request.request_id,
        "tenant_id": request.tenant_id,
        "project_id": request.project_id,
        "session_id": request.session_id,
        "user_id": request.user_id,
        "question": request.question,
        "mode": request.mode or "auto",
        "analysis_goal": request.analysis_goal,
        "limits": {
            "timeout_ms": limits.timeout_ms or CONFIG.default_timeout_ms,
            "max_input_rows": limits.max_input_rows or CONFIG.max_input_rows,
            "max_output_rows": limits.max_output_rows or CONFIG.max_output_rows,
            "max_stdout_chars": limits.max_stdout_chars or CONFIG.max_stdout_chars,
        },
        "template_name": request.template_name,
        "template_params": struct_to_dict(request.template_params),
        "datasets": [
            {
                "name": item.name,
                "datasource_id": item.datasource_id,
                "datasource_name": item.datasource_name,
                "purpose": item.purpose,
                "execution_id": item.execution_id,
                "columns": list(item.columns),
                "rows": rows_from_proto(item.rows),
            }
            for item in request.datasets
        ],
    }


def _trace_context(payload: Dict[str, Any], md: Dict[str, str]) -> Dict[str, Any]:
    return {
        "request_id": md.get("request-id") or payload.get("request_id") or "",
        "tenant_id": md.get("tenant-id") or payload.get("tenant_id") or 0,
        "project_id": md.get("project-id") or payload.get("project_id") or 0,
        "session_id": md.get("session-id") or payload.get("session_id") or 0,
        "user_id": md.get("user-id") or payload.get("user_id") or 0,
    }


def _trace_fields(trace: Dict[str, Any]) -> tuple[Any, Any, Any, Any, Any]:
    return (
        trace.get("request_id", ""),
        trace.get("tenant_id", 0),
        trace.get("project_id", 0),
        trace.get("session_id", 0),
        trace.get("user_id", 0),
    )


def _table_to_pb(item: Dict[str, Any]) -> result_analysis_pb2.AnalysisTable:
    return result_analysis_pb2.AnalysisTable(
        name=item.get("name", ""),
        columns=[str(c) for c in item.get("columns") or []],
        rows=rows_to_proto(item.get("rows") or []),
    )


def _chart_to_pb(item: Dict[str, Any]) -> result_analysis_pb2.AnalysisChart:
    return result_analysis_pb2.AnalysisChart(
        type=item.get("type", ""),
        title=item.get("title", ""),
        x_field=item.get("x_field", ""),
        y_fields=[str(c) for c in item.get("y_fields") or []],
        name_field=item.get("name_field", ""),
        value_field=item.get("value_field", ""),
        reason=item.get("reason", ""),
        rows=rows_to_proto(item.get("rows") or []),
    )


def _metric_to_pb(item: Dict[str, Any]) -> result_analysis_pb2.AnalysisMetric:
    return result_analysis_pb2.AnalysisMetric(
        name=item.get("name", ""),
        label=item.get("label", ""),
        value=float(item.get("value") or 0),
        unit=item.get("unit", ""),
        display=item.get("display", ""),
    )


def _response_to_pb(response: Dict[str, Any], stdout: str, stderr: str) -> result_analysis_pb2.AnalyzeResultSetsResponse:
    return result_analysis_pb2.AnalyzeResultSetsResponse(
        success=bool(response.get("success")),
        summary=response.get("summary", ""),
        observation=response.get("observation", ""),
        tables=[_table_to_pb(item) for item in response.get("tables") or []],
        charts=[_chart_to_pb(item) for item in response.get("charts") or []],
        metrics=[_metric_to_pb(item) for item in response.get("metrics") or []],
        warnings=[str(w) for w in response.get("warnings") or []],
        stdout_preview=response.get("stdout_preview") or stdout,
        stderr_preview=response.get("stderr_preview") or stderr,
        error=response.get("error", ""),
        duration_ms=int(response.get("duration_ms") or 0),
        input_row_count=int(response.get("input_row_count") or 0),
        output_row_count=int(response.get("output_row_count") or 0),
        analysis_kind=response.get("analysis_kind", ""),
        code_hash=response.get("code_hash", ""),
        template_name=response.get("template_name", ""),
    )


class ResultAnalysisServicer(result_analysis_pb2_grpc.ResultAnalysisServiceServicer):
    def AnalyzeResultSets(self, request, context):
        started = time.time()
        payload = _request_to_dict(request)
        md = _metadata(context)
        trace = _trace_context(payload, md)
        payload["trace"] = trace
        request_id, tenant_id, project_id, session_id, user_id = _trace_fields(trace)
        row_count = sum(len(d.get("rows") or []) for d in payload["datasets"])
        logger.info(
            "analysis started request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s datasets=%d rows=%d mode=%s template=%s timeout_ms=%d",
            request_id,
            tenant_id,
            project_id,
            session_id,
            user_id,
            len(payload["datasets"]),
            row_count,
            payload["mode"],
            payload.get("template_name", ""),
            payload["limits"].get("timeout_ms"),
        )
        result = run_in_sandbox(payload, CONFIG)
        response = _response_to_pb(result.response, result.stdout, result.stderr)
        if response.duration_ms == 0:
            response.duration_ms = int((time.time() - started) * 1000)
        log_method = logger.info if response.success else logger.warning
        log_method(
            "analysis finished request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s success=%s kind=%s template=%s duration_ms=%d input_rows=%d output_rows=%d timed_out=%s error=%s",
            request_id,
            tenant_id,
            project_id,
            session_id,
            user_id,
            response.success,
            response.analysis_kind,
            response.template_name,
            response.duration_ms,
            response.input_row_count,
            response.output_row_count,
            result.timed_out,
            response.error,
        )
        return response

    def Health(self, request, context):
        capabilities = {
            "pandas": False,
            "numpy": False,
            "subprocess": True,
            "stateless": True,
        }
        try:
            import pandas  # noqa: F401

            capabilities["pandas"] = True
        except Exception:
            pass
        try:
            import numpy  # noqa: F401

            capabilities["numpy"] = True
        except Exception:
            pass
        ok = capabilities["pandas"] and capabilities["numpy"]
        return result_analysis_pb2.HealthResponse(ok=ok, version=VERSION, capabilities=capabilities)


def serve() -> None:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=CONFIG.max_workers))
    result_analysis_pb2_grpc.add_ResultAnalysisServiceServicer_to_server(ResultAnalysisServicer(), server)
    server.add_insecure_port(CONFIG.listen)
    server.start()
    logger.info("ling-shu exec listening on %s", CONFIG.listen)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
