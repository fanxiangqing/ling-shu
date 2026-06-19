import type {
  AgentEvent,
  ChatMessageRecord,
  EmbedBootstrapResult,
  PageResult,
  SendChatMessageResult,
  VoiceChatResult,
  VoiceChatStreamEvent
} from '@/types/domain'
import { API_BASE, ApiError, friendlyErrorMessage, jsonBody, queryString, request } from '@/api/client'

export interface EmbedStreamPayload {
  access_token: string
  content: string
  datasource_id?: number
  selected_datasource_ids?: number[]
  auto_execute: boolean
  max_rows: number
}

export interface EmbedVoiceRealtimePayload {
  embed_token: string
  language?: string
  auto_execute?: boolean
  max_rows?: number
  datasource_id?: number
  selected_datasource_ids?: number[]
  voice?: string
  format?: string
}

export interface EmbedVoiceRealtimeHandlers {
  onOpen?: () => void
  onEvent?: (event: VoiceChatStreamEvent) => void
  onResult?: (result: VoiceChatResult) => void
  onError?: (error: Error) => void
  onClose?: () => void
}

export async function bootstrapEmbed(payload: {
  app_id: string
  access_token: string
  key?: string
  session_mode?: string
  parent_origin?: string
}) {
  return request<EmbedBootstrapResult>('/embed/bootstrap', {
    method: 'POST',
    body: jsonBody(payload),
    skipAuthHeader: true,
    skipUnauthorizedRedirect: true
  })
}

export async function listEmbedMessages(sessionId: number, token: string) {
  return request<PageResult<ChatMessageRecord>>(
    `/embed/chat/sessions/${sessionId}/messages${queryString({ page: 1, page_size: 200, embed_token: token })}`,
    {
      method: 'GET',
      skipAuthHeader: true,
      skipUnauthorizedRedirect: true
    }
  )
}

export async function sendEmbedMessageStream(sessionId: number, payload: EmbedStreamPayload, onStep: (event: AgentEvent) => void) {
  let reader: ReadableStreamDefaultReader<Uint8Array> | null = null
  try {
    const response = await fetch(apiPath(`/embed/chat/sessions/${sessionId}/messages/stream`), {
      method: 'POST',
      headers: {
        Accept: 'text/event-stream',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(payload)
    })
    if (!response.ok) {
      throw await streamResponseError(response)
    }
    const stream = response.body
    if (!stream || typeof stream.getReader !== 'function') {
      throw new Error(friendlyErrorMessage('stream response is not readable'))
    }

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
      if (!value) continue
      buffer += decoder.decode(value, { stream: true })
      const parts = buffer.split(/\r?\n\r?\n/)
      buffer = parts.pop() || ''
      for (const part of parts) {
        const event = parseSSEEvent(part)
        if (!event) continue
        if (event.name === 'result') {
          finalResult = event.data as SendChatMessageResult
          await closeStreamReader(reader)
          reader = null
          return finalResult
        }
        if (event.name === 'error') {
          throw new Error(streamEventMessage(event.data))
        }
        onStep(event.data as AgentEvent)
      }
    }

    if (buffer.trim()) {
      const event = parseSSEEvent(buffer)
      if (event?.name === 'result') {
        finalResult = event.data as SendChatMessageResult
        await closeStreamReader(reader)
        reader = null
        return finalResult
      }
      if (event?.name === 'error') {
        throw new Error(streamEventMessage(event.data))
      }
      if (event) onStep(event.data as AgentEvent)
    }
    if (!finalResult) throw new Error(friendlyErrorMessage('stream finished without result'))
    return finalResult
  } catch (error) {
    throw error instanceof Error ? new Error(friendlyErrorMessage(error.message)) : new Error('问数请求失败')
  } finally {
    if (reader) {
      try {
        await reader.cancel()
      } catch {
        // Stream may already be closed.
      }
      try {
        reader.releaseLock()
      } catch {
        // Reader may already be released.
      }
    }
  }
}

export function openEmbedVoiceRealtime(sessionId: number, payload: EmbedVoiceRealtimePayload, handlers: EmbedVoiceRealtimeHandlers = {}) {
  const socket = new WebSocket(embedVoiceRealtimeURL(sessionId, payload))
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
      if (data.event) handlers.onEvent?.(data.event)
    } catch (error) {
      handlers.onError?.(error instanceof Error ? new Error(friendlyErrorMessage(error.message)) : new Error('实时语音事件解析失败'))
    }
  }
  socket.onerror = () => handlers.onError?.(new Error('实时语音连接异常'))
  socket.onclose = () => handlers.onClose?.()

  return {
    socket,
    sendAudio(chunk: Blob | ArrayBuffer | ArrayBufferView) {
      if (socket.readyState === WebSocket.OPEN) socket.send(chunk)
    },
    stop() {
      if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'stop' }))
    },
    abort() {
      if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) socket.close()
    }
  }
}

function embedVoiceRealtimeURL(sessionId: number, payload: EmbedVoiceRealtimePayload) {
  const selectedDatasourceIds = payload.selected_datasource_ids?.filter(Boolean).join(',')
  const search = queryString({
    embed_token: payload.embed_token,
    language: payload.language,
    auto_execute: payload.auto_execute,
    max_rows: payload.max_rows,
    datasource_id: payload.datasource_id,
    selected_datasource_ids: selectedDatasourceIds,
    voice: payload.voice,
    format: payload.format
  })
  return websocketPath(`/embed/chat/sessions/${sessionId}/voice/realtime${search}`)
}

function streamEventMessage(data: unknown) {
  if (data && typeof data === 'object' && 'message' in data) {
    return friendlyErrorMessage(String((data as { message?: string }).message || 'stream failed'))
  }
  return friendlyErrorMessage('stream failed')
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
    // Plain text error body.
  }
  return new ApiError(friendlyErrorMessage(message, response.status), response.status, code, requestId)
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
    if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart())
  }
  if (!dataLines.length) return null
  return { name, data: JSON.parse(dataLines.join('\n')) }
}

async function closeStreamReader(reader: ReadableStreamDefaultReader<Uint8Array>) {
  try {
    await reader.cancel()
  } catch {
    // Stream may already be closed.
  }
  try {
    reader.releaseLock()
  } catch {
    // Reader may already be released.
  }
}

function apiPath(path: string) {
  const base = API_BASE.replace(/\/$/, '')
  const normalized = path.startsWith('/') ? path : `/${path}`
  return `${base}${normalized}`
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
