export type Role = 'user' | 'assistant'

export interface ProjectOption {
  id: number
  tenantId: number
  name: string
  description: string
  status: 'active' | 'paused'
}

export interface DataSourceOption {
  id: number
  projectId: number
  name: string
  type: string
  dialect: string
  role: string
  tableCount: number
  syncedAt: string
  health: 'healthy' | 'warning' | 'offline'
}

export interface ChatMessage {
  id: string
  role: Role
  content: string
  createdAt: string
  pending?: boolean
  result?: SendChatMessageResult
}

export interface ReviewResult {
  passed: boolean
  risk_level: string
  normalized_sql: string
  blocked_reason?: string
  warnings?: string[]
  limit?: number
}

export interface AgentResult {
  question: string
  intent?: 'chat' | 'query' | 'multi_query' | 'clarify'
  sql: string
  sql_tasks?: AgentSQLTask[]
  answer?: string
  explanation: string
  datasource_id?: number
  datasource_ids?: number[]
  dialect?: string
  requires_multi_datasource?: boolean
  need_clarification?: boolean
  review: ReviewResult
  steps?: AgentEvent[]
}

export interface AgentSQLTask {
  datasource_id: number
  datasource_name?: string
  dialect?: string
  purpose?: string
  sql: string
  review: ReviewResult
}

export interface AgentEvent {
  type: string
  step: number
  name?: string
  content?: string
  sql?: string
  review?: ReviewResult
  occurred_at?: string
}

export interface ChartSuggestion {
  type: string
  title?: string
  x_field?: string
  y_fields?: string[]
  name_field?: string
  value_field?: string
  reason?: string
}

export interface QueryExecution {
  id: number
  status: string
  datasource_id?: number
  final_sql?: string
  row_count?: number
  duration_ms?: number
  chart_type?: string
  error_message?: string
}

export interface QueryExecutionResult {
  execution: QueryExecution
  review: ReviewResult
  chart?: ChartSuggestion
  answer?: string
  speech_summary?: string
  error?: string
  columns?: string[]
  rows?: Record<string, unknown>[]
}

export interface SendChatMessageResult {
  agent: AgentResult
  execution?: QueryExecutionResult
  executions?: QueryExecutionResult[]
  loops?: number
  max_loops?: number
}

export interface TranscribeStreamEvent {
  task_id?: string
  raw_request_id?: string
  event?: string
  status?: string
  status_code?: number
  text?: string
  result_url?: string
  done?: boolean
}

export interface SynthesizeStreamEvent {
  audio_base64_chunk?: string
  audio_url?: string
  content_type?: string
  task_id?: string
  event?: string
  status?: string
  status_code?: number
  done?: boolean
}

export interface VoiceChatResult {
  transcript?: {
    task_id?: string
    status?: string
    text?: string
    result_url?: string
    raw_request_id?: string
  }
  chat?: SendChatMessageResult
  speech?: {
    audio_url?: string
    audio_base64?: string
    content_type?: string
    raw_request_id?: string
  }
  speech_text?: string
}

export interface VoiceChatStreamEvent {
  stage: 'status' | 'asr' | 'chat' | 'tts'
  message?: string
  transcript?: TranscribeStreamEvent
  agent?: AgentEvent
  speech?: SynthesizeStreamEvent
  done?: boolean
}

export interface AskPayload {
  tenant_id: number
  project_id: number
  user_id: number
  content: string
  datasource_id?: number
  selected_datasource_ids?: number[]
  auto_execute: boolean
  max_rows: number
}

export interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
  request_id?: string
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface BaseRecord {
  id: number
  created_at?: string
  updated_at?: string
}

export interface UserRecord extends BaseRecord {
  username: string
  email?: string
  mobile?: string
  display_name?: string
  status?: string
}

export interface MemberRecord extends BaseRecord {
  tenant_id: number
  project_id?: number
  user_id: number
  username: string
  email?: string
  mobile?: string
  display_name?: string
  status?: string
}

export interface LoginResult {
  access_token: string
  token_type: string
  expires_at: string
  user: UserRecord
}

export interface TenantRecord extends BaseRecord {
  name: string
  code: string
  status?: string
}

export interface ProjectRecord extends BaseRecord {
  tenant_id: number
  name: string
  code: string
  description?: string
  status?: string
  created_by?: number
}

export interface DataSourceRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  name: string
  db_type: string
  status?: string
  last_sync_status?: string
  last_sync_at?: string
  config_json?: string
}

export interface ChatSessionRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  user_id: number
  title: string
  status?: string
}

export interface MetadataColumnRecord {
  id: number
  table_id?: number
  column_name: string
  ordinal_position?: number
  data_type: string
  column_type?: string
  nullable: boolean
  is_primary_key: boolean
  is_foreign_key?: boolean
  default_value?: string
  comment_text?: string
  original_comment_text?: string
  business_comment_text?: string
}

export interface MetadataIndexRecord {
  id: number
  table_id?: number
  index_name: string
  index_type?: string
  unique_index: boolean
  columns_json?: string
}

export interface MetadataForeignKeyRecord {
  id: number
  table_id?: number
  constraint_name: string
  column_name: string
  referenced_schema?: string
  referenced_table: string
  referenced_column: string
}

export interface MetadataTableRecord {
  id: number
  schema_name: string
  table_name: string
  table_type: string
  comment_text?: string
  original_comment_text?: string
  business_comment_text?: string
  row_count?: number
  columns?: MetadataColumnRecord[]
  indexes?: MetadataIndexRecord[]
  foreign_keys?: MetadataForeignKeyRecord[]
}

export interface ChatMessageRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  session_id: number
  user_id?: number
  role: string
  content: string
  content_type: string
}

export interface KBTermRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  term: string
  definition: string
  aliases_json?: string
  enabled: boolean
}

export interface KBMetricRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  datasource_id?: number
  name: string
  description: string
  formula: string
  default_time_column?: string
  enabled: boolean
}

export interface KBFewShotRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  datasource_id?: number
  question: string
  sql_text: string
  explanation?: string
  enabled: boolean
}

export interface RAGKnowledgeItem {
  name: string
  description: string
  expression?: string
}

export interface RAGFewShotItem {
  question: string
  sql: string
  datasource_id?: number
}

export interface RAGHit {
  id: number
  score: number
  tenant_id: number
  project_id: number
  datasource_id?: number
  kb_type: string
  ref_id: number
  chunk_text: string
}

export interface RAGSearchResult {
  business_terms: RAGKnowledgeItem[]
  metrics: RAGKnowledgeItem[]
  few_shots: RAGFewShotItem[]
  hits?: RAGHit[]
}

export interface RAGRebuildResult {
  collection: string
  chunk_count: number
  vector_count: number
  embedding_model?: string
}

export interface ProviderInfo {
  provider: string
  configured: boolean
  model: string
  source?: string
}

export interface ProviderSummary {
  llm: ProviderInfo
  asr: ProviderInfo
  tts: ProviderInfo
}

export interface EmbedAppRecord extends BaseRecord {
  tenant_id: number
  project_id: number
  app_id: string
  name: string
  allowed_origins_json?: string
  session_policy: 'user' | 'context' | 'new'
  launcher_title: string
  welcome_message?: string
  status?: 'active' | 'disabled' | string
}

export interface EmbedAppCreateResult {
  app: EmbedAppRecord
  app_secret: string
}

export interface EmbedAppSecretResult {
  app_id: string
  app_secret: string
}

export interface EmbedTokenResult {
  access_token: string
  token_type: string
  expires_at: string
}

export interface EmbedDatasourceRecord {
  id: number
  name: string
  db_type: string
  status?: string
  synced_at?: string
}

export interface EmbedBootstrapResult {
  app: {
    app_id: string
    name: string
    launcher_title: string
    welcome_message?: string
    session_policy: string
    allowed_origins?: string[]
  }
  tenant_id: number
  project_id: number
  user_id: number
  session_id: number
  session_key: string
  datasources: EmbedDatasourceRecord[]
  capabilities: {
    asr: boolean
    tts: boolean
    realtime_voice: boolean
  }
  identity: {
    external_user_id: string
    external_user_name?: string
  }
}

export interface RoleRecord {
  code: string
  name: string
  scope_type?: string
  description?: string
}

export interface PermissionRecord {
  code: string
  resource: string
  action: string
  description?: string
}

export interface RoleBindingRecord extends BaseRecord {
  user_id: number
  role_code: string
  tenant_id?: number
  project_id?: number
}
