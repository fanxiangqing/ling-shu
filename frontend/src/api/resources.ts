import { jsonBody, queryString, request, setToken } from '@/api/client'
import {
  openVoiceChatRealtime,
  sendChatMessageStream as streamChatMessage,
  sendVoiceChatStream as streamVoiceChat
} from '@/api/chat'
import type {
  AgentEvent,
  AskPayload,
  ChatMessageRecord,
  ChatSessionRecord,
  DataSourceRecord,
  KBMetricRecord,
  KBTermRecord,
  KBFewShotRecord,
  LoginResult,
  MetadataColumnRecord,
  MetadataTableRecord,
  PageResult,
  PermissionRecord,
  ProjectRecord,
  ProviderSummary,
  QueryExecutionResult,
  RAGRebuildResult,
  RAGSearchResult,
  ReviewResult,
  RoleBindingRecord,
  RoleRecord,
  SendChatMessageResult,
  TenantRecord,
  UserRecord
} from '@/types/domain'

export type PageParams = {
  page?: number
  page_size?: number
}

export const authApi = {
  createUser(payload: {
    username: string
    password: string
    display_name?: string
    email?: string
    mobile?: string
    tenant_name?: string
    tenant_code?: string
    project_name?: string
    project_code?: string
  }) {
    return request<UserRecord>('/auth/users', { method: 'POST', body: jsonBody(payload) })
  },
  async login(payload: { username: string; password: string }) {
    const result = await request<LoginResult>('/auth/login', {
      method: 'POST',
      body: jsonBody(payload)
    })
    setToken(result.access_token)
    return result
  },
  listUsers(params: PageParams = {}) {
    return request<PageResult<UserRecord>>(`/auth/users${queryString(params)}`)
  },
  listTenantMembers(tenantId: number, params: PageParams = {}) {
    return request<PageResult<Record<string, unknown>>>(`/tenants/${tenantId}/members${queryString(params)}`)
  },
  addTenantMember(tenantId: number, payload: { user_id: number }) {
    return request(`/tenants/${tenantId}/members`, { method: 'POST', body: jsonBody(payload) })
  },
  createTenantUser(tenantId: number, payload: {
    username: string
    password: string
    display_name?: string
    email?: string
    mobile?: string
    role_code?: string
  }) {
    return request<UserRecord>(`/tenants/${tenantId}/users`, { method: 'POST', body: jsonBody(payload) })
  },
  addProjectMember(projectId: number, payload: { tenant_id: number; user_id: number }) {
    return request(`/projects/${projectId}/members`, { method: 'POST', body: jsonBody(payload) })
  },
  listProjectMembers(projectId: number, tenantId: number, params: PageParams = {}) {
    return request<PageResult<Record<string, unknown>>>(
      `/projects/${projectId}/members${queryString({ tenant_id: tenantId, ...params })}`
    )
  }
}

export const tenantApi = {
  list(params: PageParams = {}) {
    return request<PageResult<TenantRecord>>(`/tenants${queryString(params)}`)
  },
  create(payload: { name: string; code: string }) {
    return request<TenantRecord>('/tenants', { method: 'POST', body: jsonBody(payload) })
  }
}

export const projectApi = {
  list(tenantId?: number, params: PageParams = {}) {
    return request<PageResult<ProjectRecord>>(`/projects${queryString({ tenant_id: tenantId, ...params })}`)
  },
  create(payload: { tenant_id: number; name: string; code: string; description?: string; datasource_ids: number[] }) {
    return request<ProjectRecord>('/projects', { method: 'POST', body: jsonBody(payload) })
  },
  delete(id: number, tenantId: number) {
    return request<{ status: string }>(`/projects/${id}${queryString({ tenant_id: tenantId })}`, { method: 'DELETE' })
  },
  listSessions(projectId: number, tenantId: number, userId?: number, params: PageParams = {}) {
    return request<PageResult<Record<string, unknown>>>(
      `/projects/${projectId}/chat/sessions${queryString({ tenant_id: tenantId, user_id: userId, ...params })}`
    )
  },
  createSession(projectId: number, payload: { tenant_id: number; user_id: number; title?: string }) {
    return request<Record<string, unknown>>(`/projects/${projectId}/chat/sessions`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  }
}

export const datasourceApi = {
  listTenant(tenantId: number, params: PageParams = {}) {
    return request<PageResult<DataSourceRecord>>(`/tenants/${tenantId}/datasources${queryString(params)}`)
  },
  createForTenant(tenantId: number, payload: {
    name: string
    db_type: string
    dsn: string
    config_json?: string
  }) {
    return request<DataSourceRecord>(`/tenants/${tenantId}/datasources`, {
      method: 'POST',
      body: jsonBody({ tenant_id: tenantId, ...payload })
    })
  },
  listProject(projectId: number, tenantId: number, params: PageParams = {}) {
    return request<PageResult<DataSourceRecord>>(
      `/projects/${projectId}/datasources${queryString({ tenant_id: tenantId, ...params })}`
    )
  },
  list(projectId: number, tenantId: number, params: PageParams = {}) {
    return request<PageResult<DataSourceRecord>>(
      `/projects/${projectId}/datasources${queryString({ tenant_id: tenantId, ...params })}`
    )
  },
  create(projectId: number, payload: {
    tenant_id: number
    name: string
    db_type: string
    dsn: string
    config_json?: string
  }) {
    return request<DataSourceRecord>(`/projects/${projectId}/datasources`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  test(id: number) {
    return request<{ status: string; version?: string }>(`/datasources/${id}/test`, { method: 'POST' })
  },
  testConnection(payload: { tenant_id: number; db_type: string; dsn: string; config_json?: string }) {
    return request<{ status: string; version?: string }>('/datasources/test-connection', {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  delete(id: number, tenantId: number) {
    return request<{ status: string }>(`/datasources/${id}${queryString({ tenant_id: tenantId })}`, { method: 'DELETE' })
  },
  sync(id: number, payload: { trigger_type?: string; user_id?: number } = {}) {
    return request<Record<string, unknown>>(`/datasources/${id}/sync`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  metadataTables(id: number, withColumns = true, params: PageParams = {}) {
    return request<PageResult<MetadataTableRecord>>(
      `/datasources/${id}/metadata/tables${queryString({ with_columns: withColumns, ...params })}`
    )
  },
  metadataTableDetail(id: number, tableId: number) {
    return request<MetadataTableRecord>(`/datasources/${id}/metadata/tables/${tableId}`)
  },
  updateTableComment(id: number, tableId: number, payload: { comment: string; user_id?: number }) {
    return request<MetadataTableRecord>(`/datasources/${id}/metadata/tables/${tableId}/comment`, {
      method: 'PATCH',
      body: jsonBody(payload)
    })
  },
  updateColumnComment(id: number, columnId: number, payload: { comment: string; user_id?: number }) {
    return request<MetadataColumnRecord>(`/datasources/${id}/metadata/columns/${columnId}/comment`, {
      method: 'PATCH',
      body: jsonBody(payload)
    })
  }
}

export const chatApi = {
  listSessions(tenantId: number, params: { project_id?: number; user_id?: number; status?: string } & PageParams = {}) {
    return request<PageResult<ChatSessionRecord>>(`/tenants/${tenantId}/chat/sessions${queryString(params)}`)
  },
  createSession(tenantId: number, payload: { project_id: number; user_id: number; title?: string }) {
    return request<ChatSessionRecord>(`/tenants/${tenantId}/chat/sessions`, {
      method: 'POST',
      body: jsonBody({ tenant_id: tenantId, ...payload })
    })
  },
  listMessages(sessionId: number, tenantId: number, projectId: number, params: PageParams = {}) {
    return request<PageResult<ChatMessageRecord>>(
      `/chat/sessions/${sessionId}/messages${queryString({ tenant_id: tenantId, project_id: projectId, ...params })}`
    )
  },
  sendMessage(sessionId: number, payload: AskPayload) {
    return request<SendChatMessageResult>(`/chat/sessions/${sessionId}/messages`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  sendChatMessageStream(sessionId: number, payload: AskPayload, onStep: (event: AgentEvent) => void) {
    return streamChatMessage(sessionId, payload, onStep)
  },
  voice(sessionId: number, payload: Record<string, unknown>) {
    return request(`/chat/sessions/${sessionId}/voice`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  voiceStream(sessionId: number, payload: Record<string, unknown>, onEvent: Parameters<typeof streamVoiceChat>[2]) {
    return streamVoiceChat(sessionId, payload, onEvent)
  },
  voiceRealtime(sessionId: number, payload: Parameters<typeof openVoiceChatRealtime>[1], handlers?: Parameters<typeof openVoiceChatRealtime>[2]) {
    return openVoiceChatRealtime(sessionId, payload, handlers)
  }
}

export const knowledgeApi = {
  listTerms(projectId: number, tenantId: number, enabled?: boolean | null, params: PageParams = {}) {
    return request<PageResult<KBTermRecord>>(
      `/projects/${projectId}/kb/terms${queryString({ tenant_id: tenantId, enabled, ...params })}`
    )
  },
  createTerm(projectId: number, payload: {
    tenant_id: number
    term: string
    aliases?: string[]
    definition: string
    enabled?: boolean
  }) {
    return request<KBTermRecord>(`/projects/${projectId}/kb/terms`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  updateTermEnabled(projectId: number, id: number, payload: { tenant_id: number; enabled: boolean }) {
    return request<Record<string, unknown>>(`/projects/${projectId}/kb/terms/${id}/enabled`, {
      method: 'PATCH',
      body: jsonBody(payload)
    })
  },
  deleteTerm(projectId: number, id: number, tenantId: number) {
    return request<Record<string, unknown>>(`/projects/${projectId}/kb/terms/${id}${queryString({ tenant_id: tenantId })}`, {
      method: 'DELETE'
    })
  },
  listMetrics(projectId: number, tenantId: number, datasourceId?: number, enabled?: boolean | null, params: PageParams = {}) {
    return request<PageResult<KBMetricRecord>>(
      `/projects/${projectId}/kb/metrics${queryString({ tenant_id: tenantId, datasource_id: datasourceId, enabled, ...params })}`
    )
  },
  createMetric(projectId: number, payload: {
    tenant_id: number
    name: string
    description: string
    formula: string
    datasource_id?: number
    default_time_column?: string
    enabled?: boolean
  }) {
    return request<KBMetricRecord>(`/projects/${projectId}/kb/metrics`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  updateMetricEnabled(projectId: number, id: number, payload: { tenant_id: number; enabled: boolean }) {
    return request<Record<string, unknown>>(`/projects/${projectId}/kb/metrics/${id}/enabled`, {
      method: 'PATCH',
      body: jsonBody(payload)
    })
  },
  deleteMetric(projectId: number, id: number, tenantId: number) {
    return request<Record<string, unknown>>(`/projects/${projectId}/kb/metrics/${id}${queryString({ tenant_id: tenantId })}`, {
      method: 'DELETE'
    })
  },
  listFewShots(projectId: number, tenantId: number, datasourceId?: number, enabled?: boolean | null, params: PageParams = {}) {
    return request<PageResult<KBFewShotRecord>>(
      `/projects/${projectId}/kb/fewshots${queryString({ tenant_id: tenantId, datasource_id: datasourceId, enabled, ...params })}`
    )
  },
  createFewShot(projectId: number, payload: {
    tenant_id: number
    datasource_id?: number
    question: string
    sql: string
    explanation?: string
    enabled?: boolean
  }) {
    return request<KBFewShotRecord>(`/projects/${projectId}/kb/fewshots`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  updateFewShotEnabled(projectId: number, id: number, payload: { tenant_id: number; enabled: boolean }) {
    return request<Record<string, unknown>>(`/projects/${projectId}/kb/fewshots/${id}/enabled`, {
      method: 'PATCH',
      body: jsonBody(payload)
    })
  },
  deleteFewShot(projectId: number, id: number, tenantId: number) {
    return request<Record<string, unknown>>(`/projects/${projectId}/kb/fewshots/${id}${queryString({ tenant_id: tenantId })}`, {
      method: 'DELETE'
    })
  }
}

export const ragApi = {
  rebuild(projectId: number, payload: { tenant_id: number; datasource_id?: number; limit?: number }) {
    return request<RAGRebuildResult>(`/projects/${projectId}/rag/rebuild`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  },
  search(projectId: number, payload: { tenant_id: number; datasource_id?: number; question: string; limit?: number }) {
    return request<RAGSearchResult>(`/projects/${projectId}/rag/search`, {
      method: 'POST',
      body: jsonBody(payload)
    })
  }
}

export const providerApi = {
  summary(tenantId?: number, projectId?: number) {
    return request<ProviderSummary>(`/providers${queryString({ tenant_id: tenantId, project_id: projectId })}`)
  },
  getLLM(projectId: number, tenantId: number) {
    return request<Record<string, unknown>>(`/projects/${projectId}/llm-config${queryString({ tenant_id: tenantId })}`)
  },
  upsertLLM(projectId: number, payload: Record<string, unknown>) {
    return request<Record<string, unknown>>(`/projects/${projectId}/llm-config`, {
      method: 'PUT',
      body: jsonBody(payload)
    })
  },
  getASR(projectId: number, tenantId: number) {
    return request<Record<string, unknown>>(`/projects/${projectId}/asr-config${queryString({ tenant_id: tenantId })}`)
  },
  upsertASR(projectId: number, payload: Record<string, unknown>) {
    return request<Record<string, unknown>>(`/projects/${projectId}/asr-config`, {
      method: 'PUT',
      body: jsonBody(payload)
    })
  },
  getTTS(projectId: number, tenantId: number) {
    return request<Record<string, unknown>>(`/projects/${projectId}/tts-config${queryString({ tenant_id: tenantId })}`)
  },
  upsertTTS(projectId: number, payload: Record<string, unknown>) {
    return request<Record<string, unknown>>(`/projects/${projectId}/tts-config`, {
      method: 'PUT',
      body: jsonBody(payload)
    })
  }
}

export const queryApi = {
  review(payload: { tenant_id: number; project_id: number; datasource_id?: number; user_id?: number; sql: string; max_rows?: number }) {
    return request<ReviewResult>('/query/review', { method: 'POST', body: jsonBody(payload) })
  },
  execute(payload: {
    tenant_id: number
    project_id: number
    datasource_id: number
    session_id?: number
    user_id?: number
    question?: string
    sql: string
    max_rows?: number
  }) {
    return request<QueryExecutionResult>('/query/execute', { method: 'POST', body: jsonBody(payload) })
  },
  history(params: { tenant_id: number; project_id: number; user_id?: number; datasource_id?: number; status?: string } & PageParams) {
    return request<PageResult<Record<string, unknown>>>(`/query/history${queryString(params)}`)
  }
}

export const permissionApi = {
  roles() {
    return request<RoleRecord[]>('/permissions/roles')
  },
  permissions() {
    return request<PermissionRecord[]>('/permissions')
  },
  bindRole(payload: { user_id: number; role_code: string; tenant_id?: number; project_id?: number; created_by?: number }) {
    return request<RoleBindingRecord>('/permissions/role-bindings', { method: 'POST', body: jsonBody(payload) })
  },
  roleBindings(params: { user_id?: number; tenant_id?: number; project_id?: number } & PageParams = {}) {
    return request<PageResult<RoleBindingRecord>>(`/permissions/role-bindings${queryString(params)}`)
  },
  check(payload: { user_id?: number; tenant_id?: number; project_id?: number; code?: string; resource?: string; action?: string }) {
    return request<Record<string, unknown>>('/permissions/check', { method: 'POST', body: jsonBody(payload) })
  }
}

export const auditApi = {
  logs(params: {
    tenant_id?: number
    project_id?: number
    user_id?: number
    event_type?: string
    resource_type?: string
    start_time?: string
    end_time?: string
  } & PageParams = {}) {
    return request<PageResult<Record<string, unknown>>>(`/audit/logs${queryString(params)}`)
  },
  queryExecutions(params: {
    tenant_id?: number
    project_id?: number
    user_id?: number
    datasource_id?: number
    status?: string
    start_time?: string
    end_time?: string
  } & PageParams = {}) {
    return request<PageResult<Record<string, unknown>>>(`/audit/query-executions${queryString(params)}`)
  }
}
