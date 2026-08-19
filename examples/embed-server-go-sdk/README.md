# Ling-Shu 服务端问数 Go SDK 示例

这个示例演示第三方系统后端如何用 Go 调用 Ling-Shu 的服务端 Embed API，并把结果交给自己的前端页面渲染。

示例只使用 Go 标准库，不需要安装额外依赖。`App Secret` 只保存在 Go 进程环境变量中，不会下发到浏览器。

## 前置准备

1. 在 Ling-Shu 控制台进入项目管理，创建 Embed App。
2. 保存返回的 `app_id` 和 `app_secret`。
3. 确认该 Embed App 所属项目已经绑定并同步了可问数的数据源。

服务端模式不需要传 `tenant_id`、`project_id`、`user_id` 或数据源 ID。Ling-Shu 会根据 `app_id` 定位项目，并在项目已绑定且启用的数据源范围内自动路由。

## 普通问数

```bash
cd examples/embed-server-go-sdk

LINGSHU_API_BASE_URL=http://localhost:8080/api/v1 \
LINGSHU_EMBED_APP_ID=emb_xxx \
LINGSHU_EMBED_APP_SECRET=your_app_secret \
DEMO_EXTERNAL_USER_ID=third-party-user-001 \
DEMO_EXTERNAL_USER_NAME=三方系统测试用户 \
DEMO_SESSION_KEY=dashboard:demo \
DEMO_QUESTION="统计本月销售额和订单数" \
go run .
```

示例会调用：

```text
POST /api/v1/embed/server/chat
```

并输出：

```text
session_id: 123
session_key: dashboard:demo
answer: 本月销售额为 ...
sql: SELECT ...
chart: bar
rows:
[
  {
    "month": "2026-08",
    "sales_amount": 128000
  }
]
```

## 流式问数

设置 `DEMO_STREAM=true` 后，示例会调用 SSE 接口：

```bash
LINGSHU_API_BASE_URL=http://localhost:8080/api/v1 \
LINGSHU_EMBED_APP_ID=emb_xxx \
LINGSHU_EMBED_APP_SECRET=your_app_secret \
DEMO_EXTERNAL_USER_ID=third-party-user-001 \
DEMO_SESSION_KEY=dashboard:demo \
DEMO_QUESTION="最近 7 天每天订单数是多少" \
DEMO_STREAM=true \
go run .
```

流式模式会打印 `thought`、`action`、`observation` 等中间事件，最后输出最终结果。真实业务中可以由第三方后端把这些事件转发给自己的前端。

## 可配置环境变量

- `LINGSHU_API_BASE_URL`：Ling-Shu API 地址，默认 `http://localhost:8080/api/v1`
- `LINGSHU_EMBED_APP_ID`：Embed App 的 `app_id`
- `LINGSHU_EMBED_APP_SECRET`：Embed App 的 `app_secret`
- `DEMO_EXTERNAL_USER_ID`：第三方系统当前用户 ID，默认 `third-party-user-001`
- `DEMO_EXTERNAL_USER_NAME`：第三方系统当前用户名称
- `DEMO_SESSION_KEY`：业务上下文 Key，默认 `dashboard:demo`
- `DEMO_QUESTION`：自然语言问题
- `DEMO_MAX_ROWS`：最大返回行数，默认 `200`
- `DEMO_STREAM`：是否使用 SSE 流式接口，默认 `false`

## 代码结构

[main.go](main.go) 里包含两部分：

- `Client`：一个最小 Go SDK 风格封装，提供 `Chat` 和 `StreamChat`。
- `main`：命令行 demo，读取环境变量并调用 client。

认证信息通过 Header 传递：

```text
X-Ling-Shu-App-Id
X-Ling-Shu-App-Secret
X-Ling-Shu-External-User-Id
X-Ling-Shu-External-User-Name
```

生产系统中建议把 `Client` 部分复制到自己的后端服务里，并从第三方系统登录态中填充 `ExternalUserID` 和 `ExternalUserName`。
