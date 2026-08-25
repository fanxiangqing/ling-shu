# Ling-Shu Python Exec

[English](README.md)

`exec` 是无状态 Python 结果集分析服务。Go 后端仍负责权限、SQL 审核、业务数据源连接、审计和会话状态；Python 只接收本次请求传入的已审核 SQL 结果副本，返回摘要表、指标和图表展示元数据。

## 本地调试

```bash
cd exec
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
EXEC_GRPC_LISTEN=127.0.0.1:50051 .venv/bin/python server.py
```

Go 后端启用：

```bash
export LING_SHU_EXEC_ENABLED=true
export LING_SHU_EXEC_GRPC_ADDR=127.0.0.1:50051
```

## 配置

- `EXEC_GRPC_LISTEN`：gRPC 监听地址，默认 `127.0.0.1:50051`。
- `EXEC_TIMEOUT_MS`：单次分析超时，默认 `10000`。
- `EXEC_MAX_INPUT_ROWS`：最大输入行数，默认 `5000`。
- `EXEC_MAX_OUTPUT_ROWS`：最大输出行数，默认 `1000`。
- `EXEC_MAX_STDOUT_CHARS`：stdout/stderr 预览截断长度，默认 `10000`。
- `EXEC_MEMORY_LIMIT_MB`：worker 子进程内存限制，默认 `512`。
- `EXEC_LOG_LEVEL`：日志级别，默认 `info`。

## 内部分析策略

exec 的服务启停由 Go 侧 `LING_SHU_EXEC_ENABLED` 控制；启用后内部同时支持 `auto`、`template`、`code` 三种请求策略。

- `auto`：Python 根据结果集字段、数据类型和 question 自动选择内置模板，适合默认问数增强。
- `template`：Go 指定 `template_name` 和 `template_params`，适合明确要求趋势、分类、描述统计等固定分析。
- `code`：执行本次请求携带的一次性 Python 代码，代码来自 `analysis_goal` 或 `template_params.code`，仍在子进程、超时、内存限制和临时目录清理约束内运行。代码上下文包含 `pd`、首个结果集 `df`、按顺序排列的 `frames`、`dataset_names`、按名称索引的 `datasets`，最终结果需要赋给 `result`。

常用模板名：

- `trend_analysis`：需要 `time_field`/`x_field` 和 `value_field`/`y_field`。
- `category_analysis`：需要 `category_field`/`name_field`/`x_field` 和 `value_field`/`y_field`。
- `descriptive_stats`：对数值字段做统计摘要。
- `multi_dataset_overview`：对多个结果集做概览。

## 追踪与日志

Go 侧通过 gRPC metadata 透传 `request-id`、`tenant-id`、`project-id`、`session-id`、`user-id`。Python server、sandbox 父进程、worker 子进程和 analyzer 都会带这些字段输出日志，并记录结果集数量、输入/输出行数、模式、模板、耗时、超时和错误类型。

服务不保存会话状态，不连接业务数据库，也不把结果集明细写入日志。
