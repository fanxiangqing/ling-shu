import { reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { auditApi } from '@/api/resources'
import type { PageResult } from '@/types/domain'
import { emptyPage, textColumns } from '@/utils/format'
import { notify } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'

export const auditEventTypeOptions = [
  { label: '全部操作', value: '' },
  { label: '发送消息', value: 'chat.message' },
  { label: 'SQL 审核', value: 'sql.review' },
  { label: 'SQL 执行', value: 'query.execute' },
  { label: '元数据编辑', value: 'metadata.comment.update' }
]

export const auditResourceTypeOptions = [
  { label: '全部资源', value: '' },
  { label: '会话', value: 'chat_session' },
  { label: '查询记录', value: 'query_execution' },
  { label: 'SQL 审核', value: 'sql_review' },
  { label: '元数据表', value: 'metadata_table' },
  { label: '元数据字段', value: 'metadata_column' }
]

export const queryStatusOptions = [
  { label: '全部状态', value: '' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '等待中', value: 'pending' }
]

export const auditLogColumns = textColumns(['id', 'event_type', 'resource_type', 'resource_id', 'user_id', 'project_id', 'request_id', 'created_at'])
export const auditQueryColumns = textColumns(['id', 'question', 'status', 'datasource_id', 'row_count', 'duration_ms', 'created_at'])

export const useAuditStore = defineStore('audit', () => {
  const ws = useWorkspaceStore()

  const auditLogs = ref<PageResult<Record<string, unknown>>>(emptyPage())
  const auditQueryExecutions = ref<PageResult<Record<string, unknown>>>(emptyPage())
  const auditTimeRange = ref<[number, number] | null>(null)

  const auditFilters = reactive({
    user_id: null as number | null,
    event_type: null as string | null,
    resource_type: null as string | null,
    query_status: null as string | null
  })

  function clear() {
    auditLogs.value = emptyPage()
    auditQueryExecutions.value = emptyPage()
  }

  function auditTimeParams() {
    if (!auditTimeRange.value) return {}
    const [start, end] = auditTimeRange.value
    return {
      start_time: new Date(start).toISOString(),
      end_time: new Date(end).toISOString()
    }
  }

  async function refreshAudit(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId) {
      auditLogs.value = emptyPage()
      auditQueryExecutions.value = emptyPage()
      return
    }
    const timeParams = auditTimeParams()
    const logResult = await ws.run(
      '刷新操作日志',
      () =>
        auditApi.logs({
          tenant_id: ws.context.tenantId,
          project_id: ws.context.projectId || undefined,
          user_id: auditFilters.user_id || undefined,
          event_type: auditFilters.event_type || undefined,
          resource_type: auditFilters.resource_type || undefined,
          ...timeParams,
          ...ws.pageParams('auditLogs')
        }),
      { silent: true }
    )
    if (logResult) {
      auditLogs.value = logResult as PageResult<Record<string, unknown>>
      ws.syncPage('auditLogs', auditLogs.value)
    }

    if (!ws.context.projectId) {
      auditQueryExecutions.value = emptyPage()
    } else {
      const queryResult = await ws.run(
        '刷新查询记录',
        () =>
          auditApi.queryExecutions({
            tenant_id: ws.context.tenantId,
            project_id: ws.context.projectId,
            user_id: auditFilters.user_id || undefined,
            status: auditFilters.query_status || undefined,
            ...timeParams,
            ...ws.pageParams('auditQueries')
          }),
        { silent: true }
      )
      if (queryResult) {
        auditQueryExecutions.value = queryResult as PageResult<Record<string, unknown>>
        ws.syncPage('auditQueries', auditQueryExecutions.value)
      }
    }

    if (!options.silent) notify.success('审计记录已刷新')
  }

  async function applyAuditFilters() {
    ws.resetPage('auditLogs')
    ws.resetPage('auditQueries')
    await refreshAudit()
  }

  return {
    auditLogs,
    auditQueryExecutions,
    auditTimeRange,
    auditFilters,
    clear,
    refreshAudit,
    applyAuditFilters
  }
})
