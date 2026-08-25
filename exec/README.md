# Ling-Shu Python Exec

[Chinese](README-zh.md)

`exec` is the stateless Python result-set analysis service for Ling-Shu. The Go backend still owns authorization, SQL review, business data source connections, audit logging, and session state. Python only receives reviewed SQL result copies for the current request and returns summary tables, metrics, and chart rendering metadata.

## Local Debugging

```bash
cd exec
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
EXEC_GRPC_LISTEN=127.0.0.1:50051 .venv/bin/python server.py
```

Enable it in the Go backend:

```bash
export LING_SHU_EXEC_ENABLED=true
export LING_SHU_EXEC_GRPC_ADDR=127.0.0.1:50051
```

## Configuration

- `EXEC_GRPC_LISTEN`: gRPC listen address. Default: `127.0.0.1:50051`.
- `EXEC_TIMEOUT_MS`: per-request analysis timeout in milliseconds. Default: `10000`.
- `EXEC_MAX_INPUT_ROWS`: maximum input rows accepted per request. Default: `5000`.
- `EXEC_MAX_OUTPUT_ROWS`: maximum output rows returned per table. Default: `1000`.
- `EXEC_MAX_STDOUT_CHARS`: stdout/stderr preview truncation limit. Default: `10000`.
- `EXEC_MEMORY_LIMIT_MB`: memory limit for the worker child process. Default: `512`.
- `EXEC_LOG_LEVEL`: log level. Default: `info`.

## Internal Analysis Strategies

The service is enabled or disabled by Go through `LING_SHU_EXEC_ENABLED`. When enabled, the internal request path supports `auto`, `template`, and `code` strategies.

- `auto`: Python chooses a built-in template from the result-set fields, data types, and question. This is the default enhancement path for general data questions.
- `template`: Go provides `template_name` and `template_params`. This is useful when the orchestration layer already knows it needs a trend, category, descriptive-statistics, or multi-dataset analysis.
- `code`: executes one-off Python code carried by the current request. The code comes from `analysis_goal` or `template_params.code`, and still runs inside the child process with timeout, memory limit, and temporary-directory cleanup constraints. The code context includes `pd`, first result set `df`, ordered `frames`, `dataset_names`, and name-indexed `datasets`. The final value must be assigned to `result`.

Common template names:

- `trend_analysis`: requires `time_field`/`x_field` and `value_field`/`y_field`.
- `category_analysis`: requires `category_field`/`name_field`/`x_field` and `value_field`/`y_field`.
- `descriptive_stats`: builds statistical summaries for numeric fields.
- `multi_dataset_overview`: summarizes multiple result sets.

## Trace And Logging

Go propagates `request-id`, `tenant-id`, `project-id`, `session-id`, and `user-id` through gRPC metadata. The Python server, sandbox parent process, worker child process, and analyzer include these fields in logs, along with dataset count, input/output rows, mode, template, duration, timeout state, and error type.

The service stores no session state, does not connect to business databases, and does not write detailed result rows to logs.
