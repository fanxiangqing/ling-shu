const http = require('node:http')
const fs = require('node:fs/promises')
const path = require('node:path')
const { URL } = require('node:url')

const root = __dirname
const publicDir = path.join(root, 'public')

const config = {
  port: Number(process.env.PORT || 8099),
  lingShuApiBaseUrl: trimSlash(process.env.LINGSHU_API_BASE_URL || 'http://localhost:8080/api/v1'),
  lingShuWebBaseUrl: trimSlash(process.env.LINGSHU_WEB_BASE_URL || 'http://localhost:5173'),
  appId: process.env.LINGSHU_EMBED_APP_ID || 'emb_xxx',
  appSecret: process.env.LINGSHU_EMBED_APP_SECRET || 'replace-with-your-app-secret',
  externalUserId: process.env.DEMO_EXTERNAL_USER_ID || 'third-party-user-001',
  externalUserName: process.env.DEMO_EXTERNAL_USER_NAME || '三方系统测试用户',
  sessionKey: process.env.DEMO_SESSION_KEY || 'dashboard:demo',
  sessionMode: process.env.DEMO_SESSION_MODE || ''
}

const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`)
    if (req.method === 'GET' && url.pathname === '/') {
      await sendFile(res, path.join(publicDir, 'index.html'))
      return
    }
    if (req.method === 'GET' && url.pathname === '/config.js') {
      sendJavaScript(res, publicConfig(req))
      return
    }
    if (req.method === 'POST' && url.pathname === '/api/lingshu/embed-token') {
      await createEmbedToken(req, res)
      return
    }
    if (req.method === 'GET' && url.pathname.startsWith('/assets/')) {
      await sendFile(res, path.join(publicDir, url.pathname))
      return
    }
    sendJSON(res, 404, { message: 'not found' })
  } catch (error) {
    console.error(error)
    sendJSON(res, 500, { message: error.message || 'server error' })
  }
})

server.listen(config.port, () => {
  console.log(`Third-party embed demo: http://localhost:${config.port}`)
  console.log(`Ling-Shu web base URL: ${config.lingShuWebBaseUrl}`)
  console.log(`Ling-Shu API base URL: ${config.lingShuApiBaseUrl}`)
  console.log(`Allowed origin to add in Embed App: http://localhost:${config.port}`)
})

async function createEmbedToken(req, res) {
  await readBody(req)
  if (!config.appId || config.appId === 'emb_xxx') {
    sendJSON(res, 500, { message: '请先设置 LINGSHU_EMBED_APP_ID' })
    return
  }
  if (!config.appSecret || config.appSecret === 'replace-with-your-app-secret') {
    sendJSON(res, 500, { message: '请先设置 LINGSHU_EMBED_APP_SECRET' })
    return
  }

  const upstream = await fetch(`${config.lingShuApiBaseUrl}/embed/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      app_id: config.appId,
      app_secret: config.appSecret,
      external_user_id: config.externalUserId,
      external_user_name: config.externalUserName,
      ttl_seconds: 3600
    })
  })
  const text = await upstream.text()
  res.writeHead(upstream.status, {
    'Content-Type': upstream.headers.get('content-type') || 'application/json; charset=utf-8'
  })
  res.end(text)
}

function publicConfig(req) {
  const origin = `http://${req.headers.host || `localhost:${config.port}`}`
  return `window.THIRD_PARTY_DEMO_CONFIG = ${JSON.stringify({
    appId: config.appId,
    lingShuWebBaseUrl: config.lingShuWebBaseUrl,
    externalUserId: config.externalUserId,
    externalUserName: config.externalUserName,
    sessionKey: config.sessionKey,
    sessionMode: config.sessionMode,
    parentOrigin: origin
  }, null, 2)};\n`
}

async function sendFile(res, filePath) {
  const resolved = path.resolve(filePath)
  if (!resolved.startsWith(publicDir)) {
    sendJSON(res, 403, { message: 'forbidden' })
    return
  }
  const content = await fs.readFile(resolved)
  res.writeHead(200, { 'Content-Type': contentType(resolved) })
  res.end(content)
}

function sendJavaScript(res, script) {
  res.writeHead(200, {
    'Content-Type': 'application/javascript; charset=utf-8',
    'Cache-Control': 'no-store'
  })
  res.end(script)
}

function sendJSON(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify(body))
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let body = ''
    req.on('data', (chunk) => {
      body += chunk
      if (body.length > 1024 * 1024) {
        req.destroy(new Error('request body too large'))
      }
    })
    req.on('end', () => resolve(body))
    req.on('error', reject)
  })
}

function trimSlash(value) {
  return String(value || '').replace(/\/$/, '')
}

function contentType(filePath) {
  if (filePath.endsWith('.html')) return 'text/html; charset=utf-8'
  if (filePath.endsWith('.css')) return 'text/css; charset=utf-8'
  if (filePath.endsWith('.js')) return 'application/javascript; charset=utf-8'
  if (filePath.endsWith('.svg')) return 'image/svg+xml'
  return 'application/octet-stream'
}
