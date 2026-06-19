import type {
  AgentEvent,
  AskPayload,
  DataSourceOption,
  SendChatMessageResult,
  VoiceChatResult,
  VoiceChatStreamEvent
} from '@/types/domain'
import { API_BASE, ApiError, UNAUTHORIZED_EVENT, clearAuthState, friendlyErrorMessage, getToken, queryString, request } from '@/api/client'

const USE_MOCK_API = import.meta.env.VITE_USE_MOCK_API === 'true'
const UNAUTHORIZED_CODE = 40100

export async function sendChatMessage(sessionId: number, payload: AskPayload) {
  try {
    const result = await request<SendChatMessageResult>(`/chat/sessions/${sessionId}/messages`, {
      method: 'POST',
      body: payload
    })
    return normalizeChatResult(result)
  } catch (error) {
    if (!USE_MOCK_API) throw error
    return mockChatResult(payload)
  }
}

export async function sendChatMessageStream(
  sessionId: number,
  payload: AskPayload,
  onStep: (event: AgentEvent) => void
) {
  let responseStarted = false
  let reader: ReadableStreamDefaultReader<Uint8Array> | null = null

  try {
    const response = await fetch(apiPath(`/chat/sessions/${sessionId}/messages/stream`), {
      method: 'POST',
      headers: streamHeaders(),
      body: JSON.stringify(payload)
    })
    if (!response.ok) {
      throw await streamResponseError(response)
    }
    const stream = response.body
    if (!stream || typeof stream.getReader !== 'function') {
      throw new Error(friendlyErrorMessage('stream response is not readable'))
    }

    responseStarted = true
    reader = stream.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let finalResult: SendChatMessageResult | null = null

    while (true) {
      const { done, value } = await reader.read()
      if (done) {
        buffer += decoder.decode()
        break
      }
      if (value) {
        buffer += decoder.decode(value, { stream: true })
        const parts = buffer.split(/\r?\n\r?\n/)
        buffer = parts.pop() || ''
        for (const part of parts) {
          const event = parseSSEEvent(part)
          if (!event) continue
          if (event.name === 'result') {
            finalResult = normalizeChatResult(event.data as SendChatMessageResult)
            await closeStreamReader(reader)
            reader = null
            return finalResult
          } else if (event.name === 'error') {
            throw new Error(streamEventMessage(event.data))
          } else {
            onStep(event.data as AgentEvent)
          }
        }
      }
    }

    if (buffer.trim()) {
      const event = parseSSEEvent(buffer)
      if (event?.name === 'result') {
        finalResult = normalizeChatResult(event.data as SendChatMessageResult)
        await closeStreamReader(reader)
        reader = null
        return finalResult
      }
      if (event?.name === 'error') {
        throw new Error(streamEventMessage(event.data))
      }
      if (event) {
        onStep(event.data as AgentEvent)
      }
    }
    if (!finalResult) throw new Error(friendlyErrorMessage('stream finished without result'))
    return finalResult
  } catch (error) {
    if (!responseStarted && !(error instanceof ApiError)) {
      return sendChatMessage(sessionId, payload)
    }
    throw error instanceof Error ? new Error(friendlyErrorMessage(error.message)) : new Error('问数请求失败')
  } finally {
    if (reader) {
      try {
        await reader.cancel()
      } catch {
        // The stream may already be closed by the browser or proxy.
      }
      try {
        reader.releaseLock()
      } catch {
        // Reader may already be released by the browser.
      }
    }
  }
}

export async function sendVoiceChatStream(
  sessionId: number,
  payload: Record<string, unknown>,
  onEvent: (event: VoiceChatStreamEvent) => void
) {
  let responseStarted = false
  let reader: ReadableStreamDefaultReader<Uint8Array> | null = null

  try {
    const response = await fetch(apiPath(`/chat/sessions/${sessionId}/voice/stream`), {
      method: 'POST',
      headers: streamHeaders(),
      body: JSON.stringify(payload)
    })
    if (!response.ok) {
      throw await streamResponseError(response)
    }
    const stream = response.body
    if (!stream || typeof stream.getReader !== 'function') {
      throw new Error(friendlyErrorMessage('stream response is not readable'))
    }

    responseStarted = true
    reader = stream.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let finalResult: VoiceChatResult | null = null

    while (true) {
      const { done, value } = await reader.read()
      if (done) {
        buffer += decoder.decode()
        break
      }
      if (!value) continue
      buffer += decoder.decode(value, { stream: true })
      const parts = buffer.split(/\r?\n\r?\n/)
      buffer = parts.pop() || ''
      for (const part of parts) {
        const event = parseSSEEvent(part)
        if (!event) continue
        if (event.name === 'result') {
          finalResult = event.data as VoiceChatResult
          await closeStreamReader(reader)
          reader = null
          return finalResult
        }
        if (event.name === 'error') {
          throw new Error(streamEventMessage(event.data))
        }
        onEvent(event.data as VoiceChatStreamEvent)
      }
    }

    if (buffer.trim()) {
      const event = parseSSEEvent(buffer)
      if (event?.name === 'result') {
        finalResult = event.data as VoiceChatResult
        await closeStreamReader(reader)
        reader = null
        return finalResult
      }
      if (event?.name === 'error') {
        throw new Error(streamEventMessage(event.data))
      }
      if (event) {
        onEvent(event.data as VoiceChatStreamEvent)
      }
    }
    if (!finalResult) throw new Error(friendlyErrorMessage('stream finished without result'))
    return finalResult
  } catch (error) {
    if (!responseStarted && !(error instanceof ApiError)) {
      return request<VoiceChatResult>(`/chat/sessions/${sessionId}/voice`, {
        method: 'POST',
        body: payload
      })
    }
    throw error instanceof Error ? new Error(friendlyErrorMessage(error.message)) : new Error('语音问数请求失败')
  } finally {
    if (reader) {
      try {
        await reader.cancel()
      } catch {
        // The stream may already be closed by the browser or proxy.
      }
      try {
        reader.releaseLock()
      } catch {
        // Reader may already be released by the browser.
      }
    }
  }
}

export interface VoiceRealtimePayload {
  tenant_id: number
  project_id: number
  user_id: number
  language?: string
  auto_execute?: boolean
  max_rows?: number
  datasource_id?: number
  selected_datasource_ids?: number[]
  voice?: string
  format?: string
}

export interface VoiceRealtimeHandlers {
  onOpen?: () => void
  onEvent?: (event: VoiceChatStreamEvent) => void
  onResult?: (result: VoiceChatResult) => void
  onError?: (error: Error) => void
  onClose?: () => void
}

export function openVoiceChatRealtime(sessionId: number, payload: VoiceRealtimePayload, handlers: VoiceRealtimeHandlers = {}) {
  const socket = new WebSocket(voiceRealtimeURL(sessionId, payload))
  socket.binaryType = 'arraybuffer'

  socket.onopen = () => handlers.onOpen?.()
  socket.onmessage = (message) => {
    try {
      const data = JSON.parse(String(message.data)) as {
        type?: string
        event?: VoiceChatStreamEvent
        result?: VoiceChatResult
        message?: string
      }
      if (data.type === 'error') {
        handlers.onError?.(new Error(friendlyErrorMessage(data.message || '实时语音问数失败')))
        return
      }
      if (data.result) {
        handlers.onResult?.(data.result)
        return
      }
      if (data.event) {
        handlers.onEvent?.(data.event)
      }
    } catch (error) {
      handlers.onError?.(error instanceof Error ? new Error(friendlyErrorMessage(error.message)) : new Error('实时语音事件解析失败'))
    }
  }
  socket.onerror = () => handlers.onError?.(new Error('实时语音连接异常'))
  socket.onclose = () => handlers.onClose?.()

  return {
    socket,
    sendAudio(chunk: Blob | ArrayBuffer | ArrayBufferView) {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(chunk)
      }
    },
    stop() {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'stop' }))
      }
    },
    close() {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'stop' }))
      }
      if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
        socket.close()
      }
    },
    abort() {
      if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
        socket.close()
      }
    }
  }
}

function streamEventMessage(data: unknown) {
  if (data && typeof data === 'object' && 'message' in data) {
    return friendlyErrorMessage(String((data as { message?: string }).message || 'stream failed'))
  }
  return friendlyErrorMessage('stream failed')
}

async function closeStreamReader(reader: ReadableStreamDefaultReader<Uint8Array>) {
  try {
    await reader.cancel()
  } catch {
    // The stream may already be closed by the browser or proxy.
  }
  try {
    reader.releaseLock()
  } catch {
    // The lock may already be released.
  }
}

function streamHeaders() {
  const headers = new Headers({
    Accept: 'text/event-stream',
    'Content-Type': 'application/json'
  })
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  return headers
}

function apiPath(path: string) {
  const base = API_BASE.replace(/\/$/, '')
  const normalized = path.startsWith('/') ? path : `/${path}`
  return `${base}${normalized}`
}

function voiceRealtimeURL(sessionId: number, payload: VoiceRealtimePayload) {
  const selectedDatasourceIds = payload.selected_datasource_ids?.filter(Boolean).join(',')
  const search = queryString({
    tenant_id: payload.tenant_id,
    project_id: payload.project_id,
    user_id: payload.user_id,
    language: payload.language,
    auto_execute: payload.auto_execute,
    max_rows: payload.max_rows,
    datasource_id: payload.datasource_id,
    selected_datasource_ids: selectedDatasourceIds,
    voice: payload.voice,
    format: payload.format
  })
  return websocketPath(`/chat/sessions/${sessionId}/voice/realtime${search}`)
}

function websocketPath(path: string) {
  const base = API_BASE.replace(/\/$/, '')
  const normalized = path.startsWith('/') ? path : `/${path}`
  if (/^https?:\/\//.test(base)) {
    return `${base.replace(/^http/, 'ws')}${normalized}`
  }
  const origin = window.location.origin.replace(/^http/, 'ws')
  return `${origin}${base}${normalized}`
}

async function streamResponseError(response: Response) {
  const text = await response.text().catch(() => '')
  let message = text || `请求失败：${response.status}`
  let code: number | undefined
  let requestId: string | undefined
  try {
    const body = JSON.parse(text) as { message?: string; code?: number; request_id?: string }
    message = body.message || message
    code = body.code
    requestId = body.request_id
  } catch {
    // Non-JSON error bodies are surfaced as plain text.
  }
  const error = new ApiError(friendlyErrorMessage(message, response.status), response.status, code, requestId)
  if (response.status === 401 || code === UNAUTHORIZED_CODE) {
    clearAuthState()
    window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT))
  }
  return error
}

function parseSSEEvent(block: string) {
  const lines = block.split(/\r?\n/)
  let name = 'message'
  const dataLines: string[] = []
  for (const line of lines) {
    if (line.startsWith('event:')) {
      name = line.slice(6).trim()
      continue
    }
    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trimStart())
    }
  }
  if (!dataLines.length) return null
  return { name, data: JSON.parse(dataLines.join('\n')) }
}

function normalizeChatResult(result: SendChatMessageResult): SendChatMessageResult {
  if (!result.execution && result.agent?.review) {
    return result
  }
  return result
}

function mockChatResult(payload: AskPayload): SendChatMessageResult {
  const datasourceId = payload.datasource_id || payload.selected_datasource_ids?.[0] || 1
  const rows = [
    { 日期: '06-10', 销售额: 128640, 订单数: 921 },
    { 日期: '06-11', 销售额: 139420, 订单数: 988 },
    { 日期: '06-12', 销售额: 156210, 订单数: 1041 },
    { 日期: '06-13', 销售额: 148930, 订单数: 997 },
    { 日期: '06-14', 销售额: 171080, 订单数: 1126 }
  ]
  const sql = `select date(created_at) as 日期, sum(pay_amount) as 销售额, count(*) as 订单数
from orders
where created_at >= current_date - interval 7 day
group by date(created_at)
order by 日期
limit ${payload.max_rows || 200}`

  return {
    agent: {
      question: payload.content,
      sql,
      explanation: '按订单创建日期聚合支付金额和订单数，返回最近 7 天趋势。',
      datasource_id: datasourceId,
      dialect: 'mysql',
      review: {
        passed: true,
        risk_level: 'low',
        normalized_sql: sql,
        limit: payload.max_rows || 200
      }
    },
    execution: payload.auto_execute
      ? {
          execution: {
            id: Date.now(),
            status: 'success',
            final_sql: sql,
            row_count: rows.length,
            duration_ms: 184,
            chart_type: 'line'
          },
          review: {
            passed: true,
            risk_level: 'low',
            normalized_sql: sql,
            limit: payload.max_rows || 200
          },
          chart: {
            type: 'line',
            title: '最近 7 天销售额趋势',
            x_field: '日期',
            y_fields: ['销售额', '订单数']
          },
          answer: '最近 7 天销售额整体上行，06-14 达到 171080，订单数同步增长到 1126。',
          columns: ['日期', '销售额', '订单数'],
          rows
        }
      : undefined
  }
}

export const demoDataSources: DataSourceOption[] = [
  {
    id: 1,
    projectId: 1,
    name: 'ecommerce-mysql',
    type: 'MySQL',
    dialect: 'mysql',
    role: '交易主库',
    tableCount: 42,
    syncedAt: '刚刚',
    health: 'healthy'
  },
  {
    id: 2,
    projectId: 1,
    name: 'warehouse-clickhouse',
    type: 'ClickHouse',
    dialect: 'clickhouse',
    role: '明细宽表',
    tableCount: 18,
    syncedAt: '12 分钟前',
    health: 'healthy'
  },
  {
    id: 3,
    projectId: 1,
    name: 'crm-postgres',
    type: 'PostgreSQL',
    dialect: 'postgresql',
    role: '用户画像',
    tableCount: 27,
    syncedAt: '1 小时前',
    health: 'warning'
  }
]
