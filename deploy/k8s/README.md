# Ling-Shu Kubernetes

这一组清单用于部署 Ling-Shu API。默认假设 MySQL、Redis、Milvus 使用集群内已有服务或托管服务，不在这里创建 StatefulSet。

## 使用方式

1. 修改 `secret.yaml` 中的占位密钥和 MySQL DSN。
2. 修改 `configmap.yaml` 中的 Redis、Milvus 地址。
3. 修改 `deployment.yaml` 中的镜像地址。
4. 修改 `ingress.yaml` 中的域名和 ingressClassName。
5. 执行：

```bash
kubectl apply -k deploy/k8s
```

SSE 问数和 Agent Loop 可能会持续较久，Ingress 默认设置了 600 秒代理读写超时。

