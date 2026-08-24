from __future__ import annotations

import os
from dataclasses import dataclass


def _int_env(key: str, default: int) -> int:
    raw = os.getenv(key)
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


@dataclass(frozen=True)
class Config:
    listen: str = os.getenv("EXEC_GRPC_LISTEN", "127.0.0.1:50051")
    log_level: str = os.getenv("EXEC_LOG_LEVEL", "info")
    max_workers: int = _int_env("EXEC_MAX_WORKERS", 8)
    default_timeout_ms: int = _int_env("EXEC_TIMEOUT_MS", 10000)
    max_input_rows: int = _int_env("EXEC_MAX_INPUT_ROWS", 5000)
    max_output_rows: int = _int_env("EXEC_MAX_OUTPUT_ROWS", 1000)
    max_stdout_chars: int = _int_env("EXEC_MAX_STDOUT_CHARS", 10000)
    memory_limit_mb: int = _int_env("EXEC_MEMORY_LIMIT_MB", 512)


CONFIG = Config()
