<script setup lang="ts">
import { NButton, NIcon, NInput, NPagination, NPopconfirm, NSelect, NSpace, NTag } from 'naive-ui'
import { Database, Search } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useDatasourceStore, dbTypeOptions } from '@/stores/datasource'
import { datasourceSyncLabel, datasourceSyncTagType, datasourceVersion, formatDateTime, pageSize, showPagination } from '@/utils/format'
import DatasourceFormModal from '@/components/modules/datasource/DatasourceFormModal.vue'
import MetadataPreviewModal from '@/components/modules/datasource/MetadataPreviewModal.vue'

const workspace = useWorkspaceStore()
const datasourceStore = useDatasourceStore()

const { context, pageState } = storeToRefs(workspace)
const { datasources, datasourceSearch, datasourceTypeFilter, datasourceModalVisible, filteredDatasources } =
  storeToRefs(datasourceStore)
</script>

<template>
  <section class="module-page datasource-page">
    <div class="config-banner">
      <div>
        <span>数据源管理</span>
        <h2>数据源列表以块展示，连接、同步和查看元数据都在这里完成</h2>
        <p>数据源属于组织。创建项目时再从这些数据源里选择可用范围，业务用户进入对话后无需理解连接细节。</p>
      </div>
      <NSpace>
        <NButton type="primary" @click="datasourceModalVisible = true">添加数据源</NButton>
        <NButton secondary @click="() => datasourceStore.refreshTenantDatasources()">刷新数据源</NButton>
      </NSpace>
    </div>

    <section class="surface datasource-list-surface">
      <div class="surface-head">
        <div>
          <h2>数据源列表</h2>
        </div>
        <NTag size="small">{{ filteredDatasources.length }} / {{ datasources.total }} 个</NTag>
      </div>
      <div class="datasource-toolbar">
        <NInput v-model:value="datasourceSearch" clearable placeholder="搜索数据源名称或类型">
          <template #prefix>
            <NIcon :component="Search" />
          </template>
        </NInput>
        <NSelect
          v-model:value="datasourceTypeFilter"
          :options="dbTypeOptions"
          clearable
          placeholder="全部类型"
        />
        <NButton type="primary" secondary @click="() => datasourceStore.refreshTenantDatasources()">刷新</NButton>
      </div>
      <div v-if="filteredDatasources.length" class="datasource-card-grid">
        <article
          v-for="source in filteredDatasources"
          :key="source.id"
          class="datasource-card"
          :class="{ selected: source.id === context.datasourceId }"
          @click="datasourceStore.viewSelectedDatasourceMetadata(source)"
        >
          <div class="datasource-card-head">
            <div class="datasource-icon">{{ source.db_type.slice(0, 2).toUpperCase() }}</div>
            <div>
              <h3>{{ source.name }}</h3>
              <p>{{ source.db_type }} · 数据源 #{{ source.id }}</p>
            </div>
          </div>
          <div class="datasource-card-meta">
            <span>组织 #{{ source.tenant_id }}</span>
            <span>最近同步：{{ formatDateTime(source.last_sync_at, '暂无') }}</span>
            <span>版本：{{ datasourceVersion(source) || '未识别' }}</span>
          </div>
          <div class="datasource-card-footer">
            <div class="datasource-tags">
              <NTag size="small" :type="source.status === 'active' ? 'success' : 'default'">
                {{ source.status || 'active' }}
              </NTag>
              <NTag size="small" :type="datasourceSyncTagType(source)">
                {{ datasourceSyncLabel(source) }}
              </NTag>
            </div>
            <NSpace size="small">
              <NButton size="small" secondary @click.stop="datasourceStore.testSelectedDatasource(source)">测试</NButton>
              <NButton size="small" secondary @click.stop="datasourceStore.syncSelectedDatasource(source)">同步</NButton>
              <NButton size="small" type="primary" secondary @click.stop="datasourceStore.viewSelectedDatasourceMetadata(source)">
                元数据
              </NButton>
              <div @click.stop>
                <NPopconfirm @positive-click="datasourceStore.deleteDatasource(source)">
                  <template #trigger>
                    <NButton size="small" type="error" secondary>删除</NButton>
                  </template>
                  删除后会移除这个数据源的项目绑定和已同步元数据，确定删除吗？
                </NPopconfirm>
              </div>
            </NSpace>
          </div>
        </article>
      </div>
      <div v-else class="empty-state compact">
        <NIcon :component="Database" />
        <h2>还没有数据源</h2>
        <p>点击“添加数据源”创建连接，添加成功后会以块的方式展示在这里。</p>
        <NButton type="primary" @click="datasourceModalVisible = true">添加数据源</NButton>
      </div>
      <div v-if="showPagination(datasources)" class="pager-row">
        <NPagination
          :page="pageState.datasources"
          :page-size="pageSize(datasources)"
          :item-count="datasources.total"
          @update:page="(page) => workspace.changePage('datasources', page, datasourceStore.refreshTenantDatasources)"
        />
      </div>
    </section>

    <MetadataPreviewModal />

    <DatasourceFormModal />
  </section>
</template>
