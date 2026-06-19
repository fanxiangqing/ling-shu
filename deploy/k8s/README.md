# Ling-Shu Kubernetes

[English](#english) | [中文](#中文)

## 中文

这一组清单用于在 Kubernetes 上部署 Ling-Shu API，并附带集群内置的 MySQL、Redis、Milvus（含 etcd / minio）依赖。所有资源默认部署在 `ling-shu` 命名空间。

> 如果你已有托管的 MySQL / Redis / Milvus，可从 `kustomization.yaml` 移除对应的 `mysql.yaml` / `redis.yaml` / `milvus.yaml`，并把 `secret.yaml`、`configmap.yaml` 里的地址改为外部服务。

### 清单说明

- `namespace.yaml`：命名空间。
- `configmap.yaml`：非敏感配置（Redis / Milvus 地址、日志、限流等）。
- `secret.yaml`：DSN、JWT、加密密钥、MySQL / MinIO 凭据。
- `mysql.yaml`：内置 MySQL 8.4 StatefulSet + Service + PVC。
- `redis.yaml`：内置 Redis 7.4 StatefulSet + Service + PVC。
- `milvus.yaml`：Milvus 单机栈（etcd + minio + milvus standalone）。
- `deployment.yaml` / `service.yaml` / `ingress.yaml`：API 部署、服务与入口。

### 使用方式

1. 修改 `secret.yaml` 中的占位密钥：`LING_SHU_JWT_SECRET`、`LING_SHU_DSN_SECRET`、`MYSQL_ROOT_PASSWORD`，并确保 `LING_SHU_MYSQL_DSN` 中的密码与 `MYSQL_ROOT_PASSWORD` 一致。
2. 如需语音能力，在 `secret.yaml` 填入阿里云密钥，并在 `configmap.yaml` 把 `LING_SHU_ASR_ENABLED` / `LING_SHU_TTS_ENABLED` 设为 `true`。
3. 修改 `deployment.yaml` 中的镜像地址（默认 `ling-shu-api:latest`）。
4. 修改 `ingress.yaml` 中的域名和 `ingressClassName`。
5. 部署：

```bash
kubectl apply -k deploy/k8s
```

6. 初始化数据库表结构（首次部署，MySQL 就绪后执行一次）：

```bash
kubectl -n ling-shu exec -i statefulset/mysql -- \
  sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" ling_shu' \
  < scripts/mysql/001_init_schema.sql
```

> 后续的增量脚本（`scripts/mysql/002_*.sql` 等）按需用同样方式导入。

SSE 问数和 Agent Loop 可能会持续较久，Ingress 默认设置了 600 秒代理读写超时。

## English

These manifests deploy the Ling-Shu API on Kubernetes, together with in-cluster MySQL, Redis, and Milvus (etcd / minio included). All resources default to the `ling-shu` namespace.

> If you already run managed MySQL / Redis / Milvus, remove `mysql.yaml` / `redis.yaml` / `milvus.yaml` from `kustomization.yaml` and point the addresses in `secret.yaml` and `configmap.yaml` to your external services.

### Manifests

- `namespace.yaml`: namespace.
- `configmap.yaml`: non-sensitive config (Redis / Milvus addresses, logging, rate limiting).
- `secret.yaml`: DSN, JWT, encryption secret, MySQL / MinIO credentials.
- `mysql.yaml`: in-cluster MySQL 8.4 StatefulSet + Service + PVC.
- `redis.yaml`: in-cluster Redis 7.4 StatefulSet + Service + PVC.
- `milvus.yaml`: Milvus standalone stack (etcd + minio + milvus standalone).
- `deployment.yaml` / `service.yaml` / `ingress.yaml`: API Deployment, Service, and Ingress.

### Usage

1. Edit the placeholder secrets in `secret.yaml`: `LING_SHU_JWT_SECRET`, `LING_SHU_DSN_SECRET`, `MYSQL_ROOT_PASSWORD`, and make sure the password in `LING_SHU_MYSQL_DSN` matches `MYSQL_ROOT_PASSWORD`.
2. For voice features, fill the Aliyun secrets in `secret.yaml` and set `LING_SHU_ASR_ENABLED` / `LING_SHU_TTS_ENABLED` to `true` in `configmap.yaml`.
3. Update the image in `deployment.yaml` (defaults to `ling-shu-api:latest`).
4. Update the host and `ingressClassName` in `ingress.yaml`.
5. Deploy:

```bash
kubectl apply -k deploy/k8s
```

6. Initialize the database schema (once, after MySQL is ready):

```bash
kubectl -n ling-shu exec -i statefulset/mysql -- \
  sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" ling_shu' \
  < scripts/mysql/001_init_schema.sql
```

> Apply later incremental scripts (`scripts/mysql/002_*.sql`, etc.) the same way when needed.

Agent question answering uses long-lived SSE connections, so the Ingress sets a 600s proxy read/write timeout by default.
