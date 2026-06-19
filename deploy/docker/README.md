# Ling-Shu Docker 部署 / Docker Deployment

[English](#english) | [中文](#中文)

## 中文

这一组文件用于在单机上通过 Docker Compose 一键拉起 Ling-Shu 全栈：后端 API、前端 Web、MySQL、Redis 以及 Milvus 全套依赖（etcd / minio / standalone）。

> 仅做本地快速验证时，可直接使用仓库根目录的 `docker-compose.yml`（不含前端和 Milvus）。生产/完整体验请使用本目录。

### 目录内容

- `docker-compose.yml`：完整服务编排，API 和 Web 镜像从源码构建。
- `.env.example`：环境变量模板，包含端口、密钥和可选的阿里云配置。

### 使用方式

1. 准备环境变量：

```bash
cp .env.example .env
```

2. 修改 `.env` 中的必填密钥（容器在缺失时会拒绝启动）：

- `LING_SHU_JWT_SECRET`：JWT 签名密钥。
- `LING_SHU_DSN_SECRET`：数据源 DSN 加密密钥。
- `MYSQL_ROOT_PASSWORD`：MySQL root 密码。

3. 如需语音能力，填入阿里云 `LING_SHU_ALIYUN_API_KEY`、`ALIYUN_AK_ID`、`ALIYUN_AK_SECRET`、`LING_SHU_ALIYUN_NLS_APP_KEY`，并将 `LING_SHU_ASR_ENABLED` / `LING_SHU_TTS_ENABLED` 设为 `true`。

4. 启动：

```bash
docker compose --env-file .env up -d --build
```

5. 访问：

- 前端控制台：`http://localhost:${LING_SHU_WEB_PORT:-80}`
- 后端 API：`http://localhost:${LING_SHU_API_PORT:-8080}/api/v1`
- 健康检查：`http://localhost:8080/healthz`、`http://localhost:8080/readyz`

### 说明

- MySQL 首次启动会自动执行 `scripts/mysql/001_init_schema.sql` 初始化表结构。
- 如使用外部 MySQL，可在 `.env` 设置 `LING_SHU_MYSQL_DSN` 覆盖默认连接。
- 不需要 RAG / 向量召回时，可设置 `LING_SHU_MILVUS_ENABLED=false`，并按需停用 `etcd`、`minio`、`milvus` 服务。
- 所有数据保存在命名卷中（`mysql_data`、`redis_data`、`milvus_data` 等），`docker compose down -v` 会清空数据。

### 常用命令

```bash
# 查看日志
docker compose logs -f api

# 重新构建并更新
docker compose up -d --build api web

# 停止并保留数据
docker compose down

# 停止并清空数据卷
docker compose down -v
```

## English

These files bring up the full Ling-Shu stack on a single host with Docker Compose: the backend API, the frontend web, MySQL, Redis, and the full Milvus dependency set (etcd / minio / standalone).

> For a quick local check, use the root `docker-compose.yml` (no frontend, no Milvus). For a production-like / complete experience, use this directory.

### Contents

- `docker-compose.yml`: full service orchestration; API and Web images are built from source.
- `.env.example`: environment variable template for ports, secrets, and optional Aliyun config.

### Usage

1. Prepare environment variables:

```bash
cp .env.example .env
```

2. Set the required secrets in `.env` (containers refuse to start without them):

- `LING_SHU_JWT_SECRET`: JWT signing secret.
- `LING_SHU_DSN_SECRET`: datasource DSN encryption key.
- `MYSQL_ROOT_PASSWORD`: MySQL root password.

3. For voice features, fill in the Aliyun values (`LING_SHU_ALIYUN_API_KEY`, `ALIYUN_AK_ID`, `ALIYUN_AK_SECRET`, `LING_SHU_ALIYUN_NLS_APP_KEY`) and set `LING_SHU_ASR_ENABLED` / `LING_SHU_TTS_ENABLED` to `true`.

4. Start:

```bash
docker compose --env-file .env up -d --build
```

5. Access:

- Web console: `http://localhost:${LING_SHU_WEB_PORT:-80}`
- Backend API: `http://localhost:${LING_SHU_API_PORT:-8080}/api/v1`
- Health checks: `http://localhost:8080/healthz`, `http://localhost:8080/readyz`

### Notes

- MySQL runs `scripts/mysql/001_init_schema.sql` on first startup to initialize the schema.
- To use an external MySQL, set `LING_SHU_MYSQL_DSN` in `.env` to override the default connection.
- If you do not need RAG / vector retrieval, set `LING_SHU_MILVUS_ENABLED=false` and stop the `etcd`, `minio`, and `milvus` services as needed.
- All data is stored in named volumes (`mysql_data`, `redis_data`, `milvus_data`, ...). `docker compose down -v` wipes the data.

### Common commands

```bash
# Tail logs
docker compose logs -f api

# Rebuild and update
docker compose up -d --build api web

# Stop and keep data
docker compose down

# Stop and wipe volumes
docker compose down -v
```
