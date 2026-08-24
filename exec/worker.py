from __future__ import annotations

import json
import logging
import sys
import time
import traceback

from analyzer import analyze_request

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger("ling-shu-exec.worker")


def _trace(request):
    trace = dict(request.get("trace") or {})
    return {
        "request_id": trace.get("request_id") or request.get("request_id") or "",
        "tenant_id": trace.get("tenant_id") or request.get("tenant_id") or 0,
        "project_id": trace.get("project_id") or request.get("project_id") or 0,
        "session_id": trace.get("session_id") or request.get("session_id") or 0,
        "user_id": trace.get("user_id") or request.get("user_id") or 0,
    }


def _row_count(request):
    return sum(len(dataset.get("rows") or []) for dataset in request.get("datasets") or [])


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: worker.py <input.json> <output.json>", file=sys.stderr)
        return 2
    input_path, output_path = sys.argv[1], sys.argv[2]
    started = time.time()
    try:
        with open(input_path, "r", encoding="utf-8") as f:
            request = json.load(f)
        trace = _trace(request)
        logger.info(
            "worker loaded request request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s datasets=%d rows=%d mode=%s",
            trace["request_id"],
            trace["tenant_id"],
            trace["project_id"],
            trace["session_id"],
            trace["user_id"],
            len(request.get("datasets") or []),
            _row_count(request),
            request.get("mode") or "auto",
        )
        response = analyze_request(request)
        response["duration_ms"] = int((time.time() - started) * 1000)
    except Exception as exc:  # noqa: BLE001
        request = locals().get("request") or {}
        trace = _trace(request)
        logger.exception(
            "worker failed request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s error_type=%s",
            trace["request_id"],
            trace["tenant_id"],
            trace["project_id"],
            trace["session_id"],
            trace["user_id"],
            type(exc).__name__,
        )
        response = {
            "success": False,
            "error": f"{type(exc).__name__}: {exc}",
            "stderr_preview": traceback.format_exc(),
            "duration_ms": int((time.time() - started) * 1000),
            "analysis_kind": "error",
        }
    trace = _trace(locals().get("request") or {})
    logger.info(
        "worker writing response request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s success=%s kind=%s template=%s duration_ms=%d",
        trace["request_id"],
        trace["tenant_id"],
        trace["project_id"],
        trace["session_id"],
        trace["user_id"],
        response.get("success"),
        response.get("analysis_kind", ""),
        response.get("template_name", ""),
        response.get("duration_ms", 0),
    )
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(response, f, ensure_ascii=False)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
