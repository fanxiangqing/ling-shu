<script setup lang="ts">
import { NButton, NDataTable, NDatePicker, NPagination, NSelect, NTag } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useMemberStore } from '@/stores/member'
import {
  useAuditStore,
  auditEventTypeOptions,
  auditLogColumns,
  auditQueryColumns,
  auditResourceTypeOptions,
  queryStatusOptions
} from '@/stores/audit'
import { pageSize, showPagination, tableScrollX } from '@/utils/format'

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
        :columns="auditLogColumns"
        :data="auditLogs.items"
        :bordered="false"
        size="small"
        flex-height
        :scroll-x="tableScrollX(auditLogColumns)"
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
        :columns="auditQueryColumns"
        :data="auditQueryExecutions.items"
        :bordered="false"
        size="small"
        flex-height
        :scroll-x="tableScrollX(auditQueryColumns)"
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
  </section>
</template>
