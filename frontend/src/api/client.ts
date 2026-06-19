import axios, { AxiosHeaders, type AxiosRequestConfig } from 'axios'
import type { ApiEnvelope, PageResult } from '@/types/domain'

export const API_BASE = import.meta.env.VITE_API_BASE_URL || '/api/v1'
const TOKEN_KEY = 'ling_shu_access_token'
export const LOGIN_KEY = 'ling_shu_login'
export const UNAUTHORIZED_EVENT = 'ling-shu:unauthorized'
const UNAUTHORIZED_CODE = 40100

type RequestOptions = Omit<AxiosRequestConfig, 'baseURL' | 'data' | 'url'> & {
  body?: unknown
  skipUnauthorizedRedirect?: boolean
}

type ApiRequestConfig = AxiosRequestConfig & {
  skipUnauthorizedRedirect?: boolean
}

export class ApiError extends Error {
  status: number
  code?: number
  requestId?: string

  constructor(message: string, status: number, code?: number, requestId?: string) {
    super(message)
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

export function friendlyErrorMessage(message: string, status = 0) {
  const text = String(message || '').trim()
  if (!text) return '请求处理失败，请稍后重试'
  if (/[\u4e00-\u9fff]/.test(text)) return text
  const lower = text.toLowerCase()
  const exact: Record<string, string> = {
    'invalid input': '请求参数不完整或不合法，请检查后再试',
    'invalid request body': '请求内容格式不正确，请检查后再试',
    'invalid request query': '请求参数不完整或格式不正确，请检查后再试',
    'request failed': '请求失败，请稍后重试',
    'stream failed': '流式请求失败，请稍后重试',
    'stream response is not readable': '服务没有返回可读取的流式响应，请稍后重试',
    'stream finished without result': '流式请求已结束，但没有返回最终结果，请稍后重试',
    'service call failed': '服务调用失败，请稍后重试',
    'model service call failed': '模型服务调用失败，请稍后重试',
    'service is not configured': '服务尚未配置，请先完成相关配置',
    'provider is not configured': '服务尚未配置，请先完成相关配置',
    'provider streaming audio is not supported': '当前语音服务不支持流式音频',
    'llm provider is not configured': '大模型服务未配置，请先配置 LLM',
    'model service is not configured': '大模型服务未配置，请先配置 LLM',
    'prompt renderer is not configured': 'Prompt 模板服务未配置，请检查服务端配置',
    'rag provider is not configured': '知识库服务未配置，请先配置 RAG',
    'database is disabled': '元数据库未启用，请检查服务端配置',
    'record not found': '记录不存在或已被删除',
    'internal server error': '服务暂时异常，请稍后重试',
    'auth is not configured': '认证服务未配置，请联系管理员',
    'invalid username or password': '账号或密码不正确，请检查后再试',
    'user is disabled': '账号已停用，请联系管理员',
    'user has no active workspace': '账号没有可用组织，请联系管理员',
    'primary admin cannot be modified': '主管理员不能被停用或删除',
    'missing bearer token': '请先登录后再操作',
    'invalid bearer token': '登录状态已失效，请重新登录',
    'invalid authorization header': '登录凭证格式不正确，请重新登录',
    'authentication is required': '请先登录后再操作',
    'permission checker is not configured': '权限服务未配置，请联系管理员',
    'permission denied': '没有权限执行该操作',
    'invalid permission scope': '权限范围参数不正确',
    'tenant_id is required': '请选择组织后再操作',
    'project_id is required': '请选择项目后再操作',
    'datasource id is required': '请选择数据源后再操作',
    'datasource scope not found': '数据源不存在或没有访问权限',
    'check permission failed': '权限校验失败，请稍后重试',
    'network error': '无法连接服务，请检查后端是否启动'
  }
  if (exact[lower]) return exact[lower]
  if (status === 401) return '登录状态已失效，请重新登录'
  if (status === 403) return '没有权限执行该操作'
  if (status === 404) return '请求的资源不存在或已被删除'
  if (status >= 500) return '服务暂时异常，请稍后重试'
  if (lower.includes('timeout') || lower.includes('deadline exceeded') || lower.includes('context deadline')) {
    return '请求处理时间较长，本次已中断。可以缩小问题范围、减少结果数量，或稍后重试'
  }
  if (lower.includes('too many requests') || lower.includes('rate limit') || lower.includes('throttl')) {
    return '服务当前繁忙，请稍后重试'
  }
  if (lower.includes('unauthorized') || lower.includes('forbidden') || lower.includes('invalid api key') || lower.includes('authentication')) {
    return '服务认证失败，请检查配置或重新登录'
  }
  if (lower.includes('datasource driver not found')) {
    return '暂不支持该数据源类型，请检查数据库类型是否正确'
  }
  if (lower.includes('failed to fetch')) {
    return '请求服务失败，请检查网络或后端服务状态'
  }
  return text
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export function clearAuthState() {
  clearToken()
  localStorage.removeItem(LOGIN_KEY)
}

export const apiClient = axios.create({
  baseURL: API_BASE,
  headers: {
    Accept: 'application/json'
  }
})

apiClient.interceptors.request.use((config) => {
  const headers = AxiosHeaders.from(config.headers)
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  config.headers = headers
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const apiError = toApiError(error)
    if (isUnauthorized(apiError)) {
      handleUnauthorized((axios.isAxiosError(error) ? error.config : undefined) as ApiRequestConfig | undefined)
    }
    return Promise.reject(apiError)
  }
)

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, headers, ...config } = options
  const requestHeaders = normalizeHeaders(headers)
  if (body !== undefined && body !== null && !requestHeaders.has('Content-Type')) {
    requestHeaders.set('Content-Type', 'application/json')
  }

  const response = await apiClient.request<ApiEnvelope<T>>({
    ...config,
    url: path,
    headers: requestHeaders,
    data: body
  })
  const envelope = response.data
  if (!envelope || envelope.code !== 0) {
    const error = new ApiError(
      friendlyErrorMessage(envelope?.message || `request failed: ${response.status}`, response.status),
      response.status,
      envelope?.code,
      envelope?.request_id
    )
    if (isUnauthorized(error)) {
      handleUnauthorized(config)
    }
    throw error
  }
  return envelope.data
}

export function jsonBody(payload: unknown) {
  return JSON.stringify(payload)
}

export function queryString(params: Record<string, string | number | boolean | undefined | null>) {
  const search = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      search.set(key, String(value))
    }
  })
  const content = search.toString()
  return content ? `?${content}` : ''
}

export function emptyPage<T>(): PageResult<T> {
  return { items: [], total: 0, page: 1, page_size: 10 }
}

function toApiError(error: unknown) {
  if (error instanceof ApiError) return error
  if (axios.isAxiosError<ApiEnvelope<unknown>>(error)) {
    const status = error.response?.status || 0
    const body = isEnvelope(error.response?.data) ? error.response?.data : null
    return new ApiError(
      friendlyErrorMessage(body?.message || error.message || `request failed: ${status}`, status),
      status,
      body?.code,
      body?.request_id
    )
  }
  if (error instanceof Error) {
    return new ApiError(friendlyErrorMessage(error.message), 0)
  }
  return new ApiError(friendlyErrorMessage('request failed'), 0)
}

function isEnvelope(value: unknown): value is ApiEnvelope<unknown> {
  return !!value && typeof value === 'object' && 'code' in value
}

function isUnauthorized(error: ApiError) {
  return error.status === 401 || error.code === UNAUTHORIZED_CODE
}

function normalizeHeaders(headers: AxiosRequestConfig['headers']) {
  return AxiosHeaders.from(headers as string | Record<string, string> | AxiosHeaders | undefined)
}

function handleUnauthorized(config?: ApiRequestConfig) {
  if (config?.skipUnauthorizedRedirect) return
  clearAuthState()
  window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT))
}
