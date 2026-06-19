# 第三方系统内嵌测试 Demo

这是一个临时的三方系统模拟器：Node.js 作为第三方后端，普通 HTML 作为第三方前端。前端只加载 Ling-Shu JS SDK；`App Secret` 只保存在 Node 进程环境变量里，不会下发到浏览器。

要求 Node.js 18+，不需要安装 npm 依赖。

## 启动

先在 Ling-Shu 控制台创建内嵌应用，并把 `http://localhost:8099` 加入允许嵌入来源。

```bash
cd examples/embed-third-party-demo

LINGSHU_WEB_BASE_URL=http://localhost:5173 \
LINGSHU_API_BASE_URL=http://localhost:8080/api/v1 \
LINGSHU_EMBED_APP_ID=emb_xxx \
LINGSHU_EMBED_APP_SECRET=your_app_secret \
DEMO_EXTERNAL_USER_ID=third-party-user-001 \
DEMO_EXTERNAL_USER_NAME=三方系统测试用户 \
DEMO_SESSION_KEY=dashboard:demo \
node server.js
```

打开：

```text
http://localhost:8099
```

## 可配置环境变量

- `PORT`：Demo 服务端口，默认 `8099`
- `LINGSHU_WEB_BASE_URL`：Ling-Shu 前端地址，用于加载 `/sdk/ling-shu-embed.js`，默认 `http://localhost:5173`
- `LINGSHU_API_BASE_URL`：Ling-Shu API 地址，默认 `http://localhost:8080/api/v1`
- `LINGSHU_EMBED_APP_ID`：内嵌应用 `app_id`
- `LINGSHU_EMBED_APP_SECRET`：内嵌应用 `App Secret`
- `DEMO_EXTERNAL_USER_ID`：模拟三方用户 ID
- `DEMO_EXTERNAL_USER_NAME`：模拟三方用户名称
- `DEMO_SESSION_KEY`：模拟业务上下文 Key，例如 `dashboard:123`
- `DEMO_SESSION_MODE`：可选，传给 SDK 的 `sessionMode`

## 调用链路

1. 浏览器打开这个 Demo 页面。
2. 页面从 `LINGSHU_WEB_BASE_URL` 加载 `sdk/ling-shu-embed.js`。
3. SDK 打开对话前调用本 Demo 的 `/api/lingshu/embed-token`。
4. Node 后端使用 `LINGSHU_EMBED_APP_ID` 和 `LINGSHU_EMBED_APP_SECRET` 调用 Ling-Shu `/embed/token`。
5. SDK 使用短期 token 创建 iframe 并进入嵌入问数会话。
