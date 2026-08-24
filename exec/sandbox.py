from __future__ import annotations

import json
import logging
import os
import shutil
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Optional

from config import Config

logger = logging.getLogger("ling-shu-exec.sandbox")


@dataclass
class SandboxResult:
    response: Dict[str, Any]
    stdout: str
    stderr: str
    timed_out: bool


def _limit_resources(memory_limit_mb: int):
    if os.name != "posix":
        return None

    def apply_limits() -> None:
        try:
            import resource

            mem = max(memory_limit_mb, 64) * 1024 * 1024
            resource.setrlimit(resource.RLIMIT_AS, (mem, mem))
        except Exception:
            pass

    return apply_limits


def _safe_env() -> Dict[str, str]:
    keep = {
        "PATH": os.getenv("PATH", ""),
        "PYTHONPATH": os.getenv("PYTHONPATH", ""),
        "PYTHONUNBUFFERED": "1",
        "PYTHONDONTWRITEBYTECODE": "1",
        "TZ": os.getenv("TZ", "Asia/Shanghai"),
    }
    return {k: v for k, v in keep.items() if v}


def _trace(request: Dict[str, Any]) -> Dict[str, Any]:
    trace = dict(request.get("trace") or {})
    return {
        "request_id": trace.get("request_id") or request.get("request_id") or "",
        "tenant_id": trace.get("tenant_id") or request.get("tenant_id") or 0,
        "project_id": trace.get("project_id") or request.get("project_id") or 0,
        "session_id": trace.get("session_id") or request.get("session_id") or 0,
        "user_id": trace.get("user_id") or request.get("user_id") or 0,
    }


def _row_count(request: Dict[str, Any]) -> int:
    return sum(len(dataset.get("rows") or []) for dataset in request.get("datasets") or [])


def run_in_sandbox(request: Dict[str, Any], cfg: Config) -> SandboxResult:
    started = time.time()
    limits = dict(request.get("limits") or {})
    timeout_ms = int(limits.get("timeout_ms") or cfg.default_timeout_ms)
    timeout_sec = max(timeout_ms, 1000) / 1000
    max_stdout = int(limits.get("max_stdout_chars") or cfg.max_stdout_chars)
    trace = _trace(request)

    tmp = tempfile.mkdtemp(prefix="ling-shu-exec-")
    try:
        input_path = Path(tmp) / "input.json"
        output_path = Path(tmp) / "output.json"
        input_path.write_text(json.dumps(request, ensure_ascii=False), encoding="utf-8")

        worker_path = Path(__file__).with_name("worker.py")
        logger.info(
            "sandbox worker starting request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s datasets=%d rows=%d mode=%s timeout_ms=%d memory_limit_mb=%d",
            trace["request_id"],
            trace["tenant_id"],
            trace["project_id"],
            trace["session_id"],
            trace["user_id"],
            len(request.get("datasets") or []),
            _row_count(request),
            request.get("mode") or "auto",
            timeout_ms,
            cfg.memory_limit_mb,
        )
        proc = subprocess.Popen(
            [sys.executable, str(worker_path), str(input_path), str(output_path)],
            cwd=tmp,
            env=_safe_env(),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            preexec_fn=_limit_resources(cfg.memory_limit_mb),
        )
        try:
            stdout, stderr = proc.communicate(timeout=timeout_sec)
            timed_out = False
        except subprocess.TimeoutExpired:
            proc.kill()
            stdout, stderr = proc.communicate()
            timed_out = True

        stdout = (stdout or "")[:max_stdout]
        stderr = (stderr or "")[:max_stdout]
        if timed_out:
            logger.warning(
                "sandbox worker timed out request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s timeout_ms=%d duration_ms=%d stdout_chars=%d stderr_chars=%d",
                trace["request_id"],
                trace["tenant_id"],
                trace["project_id"],
                trace["session_id"],
                trace["user_id"],
                timeout_ms,
                int((time.time() - started) * 1000),
                len(stdout),
                len(stderr),
            )
            return SandboxResult(
                response={
                    "success": False,
                    "error": f"python analysis timed out after {timeout_ms}ms",
                    "analysis_kind": "timeout",
                },
                stdout=stdout,
                stderr=stderr,
                timed_out=True,
            )
        if output_path.exists():
            response = json.loads(output_path.read_text(encoding="utf-8"))
        else:
            response = {
                "success": False,
                "error": f"worker exited without output, code={proc.returncode}",
                "analysis_kind": "worker_error",
            }
        log_method = logger.info if response.get("success") else logger.warning
        log_method(
            "sandbox worker finished request_id=%s tenant_id=%s project_id=%s session_id=%s user_id=%s code=%s success=%s kind=%s template=%s duration_ms=%d stdout_chars=%d stderr_chars=%d",
            trace["request_id"],
            trace["tenant_id"],
            trace["project_id"],
            trace["session_id"],
            trace["user_id"],
            proc.returncode,
            response.get("success"),
            response.get("analysis_kind", ""),
            response.get("template_name", ""),
            int((time.time() - started) * 1000),
            len(stdout),
            len(stderr),
        )
        return SandboxResult(response=response, stdout=stdout, stderr=stderr, timed_out=False)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
