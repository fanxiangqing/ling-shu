<script setup lang="ts">
import { computed, h, ref } from 'vue'
import { Eye, Link2 } from '@lucide/vue'
import { NButton, NDataTable, NDatePicker, NDrawer, NDrawerContent, NIcon, NPagination, NSelect, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useMemberStore } from '@/stores/member'
import {
  useAuditStore,
  auditEventTypeOptions,
  auditResourceTypeOptions,
  queryStatusOptions
} from '@/stores/audit'
import { formatDateTime, pageSize, showPagination } from '@/utils/format'

const workspace = useWorkspaceStore()
const ui = useUiStore()
const projectStore = useProjectStore()
const memberStore = useMemberStore()
const auditStore = useAuditStore()

const { context, pageState } = storeToRefs(workspace)
const { activeAuditSub } = storeToRefs(ui)
const { projectOptions } = storeToRefs(projectStore)
const { userOptions } = storeToRefs(memberStore)
const { auditLogs, auditQueryExecutions, auditTimeRange, auditFilters } = storeToRefs(auditStore)

type AuditRow = Record<string, unknown>

type AuditDetail = {
  title: string
  kind: 'log' | 'query'
  row: AuditRow
  payload: AuditRow
}

type AuditTagType = 'default' | 'primary' | 'success' | 'info' | 'warning' | 'error'

const auditDetailVisible = ref(false)
const selectedAuditDetail = ref<AuditDetail | null>(null)

const eventTypeLabelMap = new Map(auditEventTypeOptions.map((item) => [item.value, item.label]))
const resourceTypeLabelMap = new Map(auditResourceTypeOptions.map((item) => [item.value, item.label]))
const queryStatusLabelMap = new Map(queryStatusOptions.map((item) => [item.value, item.label]))

const auditLogTableColumns = computed<DataTableColumns<AuditRow>>(() => [
  {
    key: 'id',
    title: 'ID',
    width: 76,
    render: (row) => textValue(row.id)
  },
  {
    key: 'source',
    title: '来源',
    width: 128,
    render: renderSource
  },
  {
    key: 'event_type',
    title: '操作',
    width: 150,
    render: (row) => renderTextTag(optionLabel(eventTypeLabelMap, row.event_type), 'default')
  },
  {
    key: 'resource',
    title: '资源',
    width: 168,
    render: renderResource
  },
  {
    key: 'integration',
    title: '集成方',
    width: 270,
    render: renderIntegration
  },
  {
    key: 'user_id',
    title: '用户',
    width: 100,
    render: (row) => userLabel(row)
  },
  {
    key: 'created_at',
    title: '创建时间',
    width: 190,
    render: (row) => formatDateTime(row.created_at)
  },
  {
    key: 'actions',
    title: '详情',
    width: 96,
    fixed: 'right',
    render: (row) => renderDetailAction(row, 'log')
  }
])

const auditQueryTableColumns = computed<DataTableColumns<AuditRow>>(() => [
  {
    key: 'id',
    title: 'ID',
    width: 76,
    render: (row) => textValue(row.id)
  },
  {
    key: 'source',
    title: '来源',
    width: 128,
    render: renderSource
  },
  {
    key: 'question',
    title: '问题',
    width: 320,
    render: (row) => h('div', { class: 'audit-question-cell' }, textValue(row.question, '-'))
  },
  {
    key: 'status',
    title: '状态',
    width: 108,
    render: (row) => renderStatus(row)
  },
  {
    key: 'integration',
    title: '集成方',
    width: 270,
    render: renderIntegration
  },
  {
    key: 'metrics',
    title: '执行',
    width: 170,
    render: renderQueryMetrics
  },
  {
    key: 'created_at',
    title: '创建时间',
    width: 190,
    render: (row) => formatDateTime(row.created_at)
  },
  {
    key: 'actions',
    title: '详情',
    width: 96,
    fixed: 'right',
    render: (row) => renderDetailAction(row, 'query')
  }
])

const detailBaseFields = computed(() => {
  const detail = selectedAuditDetail.value
  if (!detail) return []
  const row = detail.row
  const fields =
    detail.kind === 'log'
      ? [
          ['ID', row.id],
          ['来源', sourceLabel(row)],
          ['操作', optionLabel(eventTypeLabelMap, row.event_type)],
          ['资源', resourceLabel(row)],
          ['用户', userLabelText(row)],
          ['项目', row.project_id],
          ['请求 ID', row.request_id],
          ['IP', row.ip],
          ['创建时间', formatDateTime(row.created_at)]
        ]
      : [
          ['ID', row.id],
          ['来源', sourceLabel(row)],
          ['问题', row.question],
          ['状态', optionLabel(queryStatusLabelMap, row.status)],
          ['数据源', hasValue(row.datasource_id) ? `#${row.datasource_id}` : '未指定'],
          ['用户', userLabelText(row)],
          ['行数', row.row_count],
          ['耗时', hasValue(row.duration_ms) ? `${row.duration_ms} ms` : '暂无'],
          ['创建时间', formatDateTime(row.created_at)]
        ]
  return fields.map(([label, value]) => ({ label: String(label), value: textValue(value, '暂无') }))
})

const detailEmbedFields = computed(() => {
  const detail = selectedAuditDetail.value
  if (!detail) return []
  const meta = embedMeta(detail.row)
  return [
    ['App ID', meta.appId],
    ['三方用户 ID', meta.externalUserId],
    ['三方用户名称', meta.externalUserName],
    ['Session Key', meta.sessionKey]
  ].map(([label, value]) => ({ label, value: value || '暂无' }))
})

const detailPayloadFields = computed(() => {
  const payload = selectedAuditDetail.value?.payload || {}
  return Object.entries(payload).map(([key, value]) => ({
    label: key,
    value: payloadValue(value)
  }))
})

function payloadFromRow(row: AuditRow) {
  const directPayload = recordValue(row, 'audit_payload')
  if (isRecord(directPayload)) return directPayload
  const payloadJSON = recordValue(row, 'payload_json')
  if (isRecord(payloadJSON)) return payloadJSON
  if (typeof payloadJSON === 'string' && payloadJSON.trim()) {
    try {
      const parsed = JSON.parse(payloadJSON)
      return isRecord(parsed) ? parsed : {}
    } catch {
      return {}
    }
  }
  return {}
}

function embedMeta(row: AuditRow) {
  const payload = payloadFromRow(row)
  return {
    appId: firstValue(row, payload, ['app_id', 'embed_app_id']),
    externalUserId: firstValue(row, payload, ['external_user_id']),
    externalUserName: firstValue(row, payload, ['external_user_name']),
    sessionKey: firstValue(row, payload, ['session_key'])
  }
}

function sourceValue(row: AuditRow) {
  const payload = payloadFromRow(row)
  return firstValue(row, payload, ['source'])
}

function sourceLabel(row: AuditRow) {
  const source = sourceValue(row)
  if (source === 'embed' || embedMeta(row).appId) return '三方内嵌'
  if (!source) return '控制台'
  return source
}

function sourceTagType(row: AuditRow): AuditTagType {
  const source = sourceValue(row)
  if (source === 'embed' || embedMeta(row).appId) return 'success'
  if (!source) return 'default'
  return 'info'
}

function renderSource(row: AuditRow) {
  return h('div', { class: 'audit-source-cell' }, [
    h(
      NTag,
      { size: 'small', type: sourceTagType(row), round: true },
      {
        default: () => [
          sourceValue(row) === 'embed'
            ? h(NIcon, { size: 13, class: 'audit-source-icon' }, { default: () => h(Link2) })
            : null,
          sourceLabel(row)
        ]
      }
    ),
    sourceValue(row) && sourceValue(row) !== 'embed' ? h('span', { class: 'audit-source-raw' }, sourceValue(row)) : null
  ])
}

function renderIntegration(row: AuditRow) {
  const meta = embedMeta(row)
  if (!meta.appId && sourceValue(row) !== 'embed') {
    return h('span', { class: 'audit-muted' }, '控制台会话')
  }
  return h('div', { class: 'audit-integration-cell' }, [
    h('div', { class: 'audit-integration-app' }, meta.appId || '三方内嵌应用'),
    h('div', { class: 'audit-integration-user' }, meta.externalUserName || meta.externalUserId || '未传三方用户'),
    meta.sessionKey ? h('div', { class: 'audit-integration-key' }, meta.sessionKey) : null
  ])
}

function renderResource(row: AuditRow) {
  return h('div', { class: 'audit-resource-cell' }, [
    h('span', resourceLabel(row)),
    row.resource_id ? h('strong', `#${textValue(row.resource_id)}`) : null
  ])
}

function renderStatus(row: AuditRow) {
  const status = textValue(row.status)
  const type: AuditTagType = status === 'success' ? 'success' : status === 'failed' ? 'error' : 'warning'
  return renderTextTag(optionLabel(queryStatusLabelMap, row.status), type)
}

function renderQueryMetrics(row: AuditRow) {
  const rowCount = textValue(row.row_count, '0')
  const duration = hasValue(row.duration_ms) ? `${textValue(row.duration_ms)} ms` : '暂无耗时'
  return h('div', { class: 'audit-metric-cell' }, [
    h('span', `数据源 #${textValue(row.datasource_id, '-')}`),
    h('strong', `${rowCount} 行 / ${duration}`)
  ])
}

function renderTextTag(label: string, type: AuditTagType) {
  return h(NTag, { size: 'small', type, round: true }, { default: () => label || '未知' })
}

function auditTableScrollX(columns: DataTableColumns<AuditRow>) {
  return columns.reduce((sum, column) => {
    const width = typeof column.width === 'number' ? column.width : 150
    return sum + width
  }, 0)
}

function renderDetailAction(row: AuditRow, kind: AuditDetail['kind']) {
  return h(
    NButton,
    {
      size: 'small',
      secondary: true,
      onClick: () => openAuditDetail(row, kind)
    },
    {
      icon: () => h(NIcon, null, { default: () => h(Eye) }),
      default: () => '查看'
    }
  )
}

function openAuditDetail(row: AuditRow, kind: AuditDetail['kind']) {
  selectedAuditDetail.value = {
    title: kind === 'log' ? `操作日志 #${textValue(row.id)}` : `查询记录 #${textValue(row.id)}`,
    kind,
    row,
    payload: payloadFromRow(row)
  }
  auditDetailVisible.value = true
}

function optionLabel(map: Map<string, string>, value: unknown) {
  const text = textValue(value)
  return map.get(text) || text || '未知'
}

function resourceLabel(row: AuditRow) {
  return optionLabel(resourceTypeLabelMap, row.resource_type)
}

function userLabel(row: AuditRow) {
  return h('span', { class: 'audit-user-cell' }, userLabelText(row))
}

function userLabelText(row: AuditRow) {
  return row.user_id ? `#${textValue(row.user_id)}` : '系统'
}

function firstValue(row: AuditRow, payload: AuditRow, keys: string[]) {
  for (const key of keys) {
    const rowValue = textValue(row[key])
    if (rowValue) return rowValue
    const payloadValue = textValue(payload[key])
    if (payloadValue) return payloadValue
  }
  return ''
}

function recordValue(row: AuditRow, key: string) {
  return row[key]
}

function textValue(value: unknown, fallback = '') {
  if (value === null || value === undefined || value === '') return fallback
  return String(value)
}

function hasValue(value: unknown) {
  return value !== null && value !== undefined && value !== ''
}

function payloadValue(value: unknown) {
  if (value === null || value === undefined || value === '') return '暂无'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  return String(value)
}

function isRecord(value: unknown): value is AuditRow {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}
</script>

<template>
  <section class="module-page audit-page">
    <div class="config-banner">
      <div>
        <span>行为追踪</span>
        <h2>查看操作日志和问数执行记录</h2>
        <p>按组织、项目、成员、操作类型和时间范围排查问数链路，定位 SQL 执行、元数据维护和会话行为。</p>
      </div>
      <NButton type="primary" secondary @click="() => auditStore.refreshAudit()">刷新审计</NButton>
    </div>

    <section class="surface audit-filter-surface">
      <div class="surface-head">
        <div>
          <h2>{{ activeAuditSub === 'operationLogs' ? '操作日志筛选' : '查询记录筛选' }}</h2>
          <p class="surface-note">
            {{
              activeAuditSub === 'operationLogs'
                ? '查看成员在项目、数据源、会话和元数据里的关键操作。'
                : '查看自然语言问数的执行状态、返回行数和耗时。'
            }}
          </p>
        </div>
        <NTag size="small">
          {{ activeAuditSub === 'operationLogs' ? `${auditLogs.total} 条操作` : `${auditQueryExecutions.total} 条查询` }}
        </NTag>
      </div>
      <div class="audit-filter-grid">
        <div>
          <span>项目</span>
          <NSelect
            :value="context.projectId || null"
            :options="projectOptions"
            clearable
            filterable
            placeholder="全部项目"
            @update:value="workspace.handleProjectChange"
          />
        </div>
        <div>
          <span>成员</span>
          <NSelect v-model:value="auditFilters.user_id" :options="userOptions" clearable filterable placeholder="全部成员" />
        </div>
        <div v-if="activeAuditSub === 'operationLogs'">
          <span>操作类型</span>
          <NSelect v-model:value="auditFilters.event_type" :options="auditEventTypeOptions" clearable />
        </div>
        <div v-if="activeAuditSub === 'operationLogs'">
          <span>资源类型</span>
          <NSelect v-model:value="auditFilters.resource_type" :options="auditResourceTypeOptions" clearable />
        </div>
        <div v-if="activeAuditSub === 'queryExecutions'">
          <span>查询状态</span>
          <NSelect v-model:value="auditFilters.query_status" :options="queryStatusOptions" clearable />
        </div>
        <div>
          <span>时间范围</span>
          <NDatePicker v-model:value="auditTimeRange" type="datetimerange" clearable />
        </div>
        <NButton type="primary" @click="auditStore.applyAuditFilters">应用筛选</NButton>
      </div>
    </section>

    <section v-if="activeAuditSub === 'operationLogs'" class="surface audit-table-surface">
      <div class="surface-head">
        <div>
          <h2>操作日志</h2>
          <p class="surface-note">记录会话、SQL、元数据备注等关键行为。</p>
        </div>
        <NTag size="small">{{ auditLogs.total }} 条</NTag>
      </div>
      <NDataTable
        class="audit-data-table"
        :columns="auditLogTableColumns"
        :data="auditLogs.items"
        :bordered="false"
        size="small"
        flex-height
        :scroll-x="auditTableScrollX(auditLogTableColumns)"
      />
      <div v-if="showPagination(auditLogs)" class="pager-row compact">
        <NPagination
          :page="pageState.auditLogs"
          :page-size="pageSize(auditLogs)"
          :item-count="auditLogs.total"
          @update:page="(page) => workspace.changePage('auditLogs', page, auditStore.refreshAudit)"
        />
      </div>
    </section>

    <section v-else class="surface audit-table-surface">
      <div class="surface-head">
        <div>
          <h2>查询记录</h2>
          <p class="surface-note">查看问数执行状态、返回行数和耗时。</p>
        </div>
        <NTag size="small">{{ auditQueryExecutions.total }} 条</NTag>
      </div>
      <div v-if="!context.projectId" class="knowledge-empty-mini">请选择项目后查看查询记录</div>
      <NDataTable
        v-else
        class="audit-data-table"
        :columns="auditQueryTableColumns"
        :data="auditQueryExecutions.items"
        :bordered="false"
        size="small"
        flex-height
        :scroll-x="auditTableScrollX(auditQueryTableColumns)"
      />
      <div v-if="context.projectId && showPagination(auditQueryExecutions)" class="pager-row compact">
        <NPagination
          :page="pageState.auditQueries"
          :page-size="pageSize(auditQueryExecutions)"
          :item-count="auditQueryExecutions.total"
          @update:page="(page) => workspace.changePage('auditQueries', page, auditStore.refreshAudit)"
        />
      </div>
    </section>

    <NDrawer v-model:show="auditDetailVisible" placement="right" :width="'min(720px, calc(100vw - 28px))'">
      <NDrawerContent :title="selectedAuditDetail?.title || '审计详情'" closable>
        <div v-if="selectedAuditDetail" class="audit-detail-panel">
          <section class="audit-detail-block">
            <div class="audit-detail-title">
              <span>基础信息</span>
              <NTag size="small" :type="sourceTagType(selectedAuditDetail.row)" round>
                {{ sourceLabel(selectedAuditDetail.row) }}
              </NTag>
            </div>
            <div class="audit-detail-grid">
              <div v-for="field in detailBaseFields" :key="field.label" class="audit-detail-field">
                <span>{{ field.label }}</span>
                <strong>{{ field.value }}</strong>
              </div>
            </div>
          </section>

          <section class="audit-detail-block">
            <div class="audit-detail-title">
              <span>集成方上下文</span>
              <NTag size="small" round>{{ sourceValue(selectedAuditDetail.row) || 'console' }}</NTag>
            </div>
            <div class="audit-detail-grid compact">
              <div v-for="field in detailEmbedFields" :key="field.label" class="audit-detail-field">
                <span>{{ field.label }}</span>
                <strong>{{ field.value }}</strong>
              </div>
            </div>
          </section>

          <section class="audit-detail-block">
            <div class="audit-detail-title">
              <span>Payload</span>
              <NTag size="small" round>{{ detailPayloadFields.length }} 项</NTag>
            </div>
            <div v-if="detailPayloadFields.length" class="audit-payload-list">
              <div v-for="field in detailPayloadFields" :key="field.label" class="audit-payload-row">
                <span>{{ field.label }}</span>
                <pre>{{ field.value }}</pre>
              </div>
            </div>
            <div v-else class="knowledge-empty-mini detail">暂无 payload</div>
          </section>
        </div>
      </NDrawerContent>
    </NDrawer>
  </section>
</template>
