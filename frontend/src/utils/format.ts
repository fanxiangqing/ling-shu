import dayjs from 'dayjs'
import type {
  DataSourceRecord,
  KBTermRecord,
  MetadataForeignKeyRecord,
  MetadataIndexRecord,
  PageResult
} from '@/types/domain'

export const DEFAULT_PAGE_SIZE = 10

const DATE_COLUMN_KEYS = new Set([
  'created_at',
  'updated_at',
  'last_sync_at',
  'started_at',
  'finished_at',
  'executed_at'
])

export function formatDateTime(value: unknown, fallback = '') {
  if (value === null || value === undefined || value === '') return fallback
  const parsed = dayjs(value as string | number | Date)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : String(value)
}

export function formatDate(value: unknown, fallback = '') {
  if (value === null || value === undefined || value === '') return fallback
  const parsed = dayjs(value as string | number | Date)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD') : String(value)
}

export function emptyPage<T>(): PageResult<T> {
  return { items: [], total: 0, page: 1, page_size: DEFAULT_PAGE_SIZE }
}

export function pageSize<T>(result: PageResult<T>) {
  return result.page_size || DEFAULT_PAGE_SIZE
}

export function showPagination<T>(result: PageResult<T>) {
  return result.total > pageSize(result)
}

export const columnTitleMap: Record<string, string> = {
  id: 'ID',
  name: '名称',
  code: '编码',
  status: '状态',
  tenant_id: '组织',
  project_id: '项目',
  username: '用户名',
  display_name: '姓名',
  user_id: '用户',
  title: '会话',
  db_type: '数据库',
  last_sync_status: '同步状态',
  schema_name: '库名',
  table_name: '表名',
  table_type: '类型',
  comment_text: '说明',
  term: '术语',
  definition: '定义',
  enabled: '启用',
  formula: '口径',
  datasource_id: '数据源',
  question: '问题',
  sql_text: '示例查询',
  event_type: '操作',
  resource_type: '资源',
  resource_id: '资源 ID',
  request_id: '请求 ID',
  ip: 'IP',
  row_count: '行数',
  duration_ms: '耗时(ms)',
  final_sql: '执行 SQL',
  error_message: '错误',
  created_at: '创建时间'
}

const DEFAULT_COLUMN_MIN_WIDTH = 150

const COLUMN_MIN_WIDTH: Record<string, number> = {
  id: 80,
  status: 110,
  tenant_id: 90,
  project_id: 90,
  user_id: 90,
  resource_id: 110,
  datasource_id: 110,
  row_count: 90,
  duration_ms: 110,
  enabled: 90,
  event_type: 160,
  resource_type: 150,
  request_id: 250,
  created_at: 200,
  question: 280,
  sql_text: 280,
  final_sql: 280,
  error_message: 240
}

export function columnMinWidth(key: string) {
  return COLUMN_MIN_WIDTH[key] ?? DEFAULT_COLUMN_MIN_WIDTH
}

export function textColumns(keys: string[]) {
  return keys.map((key) => ({
    key,
    title: columnTitleMap[key] || key,
    width: columnMinWidth(key),
    ellipsis: { tooltip: true },
    render(row: Record<string, unknown>) {
      const value = row[key]
      if (value === null || value === undefined) return ''
      if (DATE_COLUMN_KEYS.has(key)) return formatDateTime(value)
      if (typeof value === 'object') return JSON.stringify(value)
      return String(value)
    }
  }))
}

export function tableScrollX(columns: Array<{ width?: number }>) {
  return columns.reduce((sum, column) => sum + (column.width ?? DEFAULT_COLUMN_MIN_WIDTH), 0)
}

export function recordText(row: object, key: string, fallback = '') {
  const value = (row as Record<string, unknown>)[key]
  if (value === null || value === undefined || value === '') return fallback
  return String(value)
}

export function memberDisplayName(row: object) {
  return recordText(row, 'display_name') || recordText(row, 'username') || `成员 #${recordText(row, 'user_id', recordText(row, 'id', '-'))}`
}

export function memberAccountName(row: object) {
  return recordText(row, 'username') || `用户 #${recordText(row, 'user_id', '-')}`
}

export function memberStatus(row: object) {
  return recordText(row, 'status', 'active')
}

export function memberStatusLabel(row: object) {
  switch (memberStatus(row)) {
    case 'active':
      return '启用'
    case 'inactive':
    case 'disabled':
    case 'paused':
      return '停用'
    default:
      return memberStatus(row)
  }
}

export function memberStatusTagType(row: object): 'success' | 'warning' | 'default' {
  return memberStatus(row) === 'active' ? 'success' : 'warning'
}

export function termAliases(term: KBTermRecord) {
  if (!term.aliases_json) return '无别名'
  try {
    const aliases = JSON.parse(term.aliases_json)
    if (Array.isArray(aliases) && aliases.length) return aliases.join('、')
  } catch {
    return term.aliases_json
  }
  return '无别名'
}

export function indexColumns(index: MetadataIndexRecord) {
  if (!index.columns_json) return '未记录字段'
  try {
    const columns = JSON.parse(index.columns_json) as unknown
    if (Array.isArray(columns)) return columns.map((item) => String(item)).join(', ') || '未记录字段'
  } catch {
    return index.columns_json
  }
  return index.columns_json
}

export function foreignKeyTarget(fk: MetadataForeignKeyRecord) {
  const schema = fk.referenced_schema ? `${fk.referenced_schema}.` : ''
  return `${schema}${fk.referenced_table}.${fk.referenced_column}`
}

export function currentChatTitle() {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`
}

export function datasourceSyncLabel(datasource: DataSourceRecord) {
  if (datasource.last_sync_status === 'success') return '已同步'
  if (datasource.last_sync_status === 'failed') return '同步失败'
  if (datasource.last_sync_status) return datasource.last_sync_status
  return '未同步'
}

export function datasourceSyncTagType(datasource: DataSourceRecord): 'success' | 'error' | 'warning' {
  if (datasource.last_sync_status === 'success') return 'success'
  if (datasource.last_sync_status === 'failed') return 'error'
  return 'warning'
}

export function datasourceVersion(datasource: DataSourceRecord) {
  const raw = datasource.config_json || ''
  if (!raw) return ''
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const version = parsed.version
    return typeof version === 'string' ? version.trim() : ''
  } catch {
    return ''
  }
}

export function generateProjectCode(name: string) {
  const normalized = name
    .trim()
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 28)
  return `${normalized || 'project'}-${Date.now().toString(36).slice(-5)}`
}
