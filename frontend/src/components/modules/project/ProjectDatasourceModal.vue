<script setup lang="ts">
import { NButton, NIcon, NModal, NPagination, NTag } from 'naive-ui'
import { Database } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useDatasourceStore } from '@/stores/datasource'
import { datasourceSyncLabel, datasourceVersion, pageSize, showPagination } from '@/utils/format'

const workspace = useWorkspaceStore()
const ui = useUiStore()
const projectStore = useProjectStore()
const datasourceStore = useDatasourceStore()

const { context, pageState } = storeToRefs(workspace)
const { projectDatasources, selectedProject, projectDatasourceModalVisible } = storeToRefs(projectStore)

function goManageDatasource() {
  projectDatasourceModalVisible.value = false
  ui.activeModule = 'datasource'
}
</script>

<template>
  <NModal
    v-model:show="projectDatasourceModalVisible"
    preset="card"
    class="project-datasource-modal"
    :title="`${selectedProject?.name || '项目'} · 数据源`"
  >
    <template #header-extra>
      <NTag size="small">{{ projectDatasources.total }} 个数据源</NTag>
    </template>

    <div class="project-datasource-modal-body">
      <p class="surface-note">对话问数只会在当前项目绑定的数据源范围内进行。</p>

      <div v-if="projectDatasources.items.length" class="datasource-card-grid compact project-datasource-modal-grid">
        <article
          v-for="source in projectDatasources.items"
          :key="source.id"
          class="datasource-card"
          :class="{ selected: source.id === context.datasourceId }"
          @click="datasourceStore.selectDatasource(source)"
        >
          <div class="datasource-card-head">
            <div class="datasource-icon">{{ source.db_type.slice(0, 2).toUpperCase() }}</div>
            <div>
              <h3>{{ source.name }}</h3>
              <p>{{ source.db_type }} · #{{ source.id }}</p>
            </div>
          </div>
          <div class="datasource-card-meta">
            <span>状态：{{ source.status || 'active' }}</span>
            <span>同步：{{ datasourceSyncLabel(source) }}</span>
            <span>版本：{{ datasourceVersion(source) || '未识别' }}</span>
          </div>
        </article>
      </div>

      <div v-else class="empty-state compact">
        <NIcon :component="Database" />
        <h2>当前项目还没有数据源</h2>
        <p>先在数据源管理里创建数据源，再创建项目并选择它。</p>
        <NButton type="primary" @click="goManageDatasource">去数据源管理</NButton>
      </div>

      <div v-if="showPagination(projectDatasources)" class="pager-row compact">
        <NPagination
          :page="pageState.projectDatasources"
          :page-size="pageSize(projectDatasources)"
          :item-count="projectDatasources.total"
          @update:page="(page) => workspace.changePage('projectDatasources', page, projectStore.refreshProjectDatasources)"
        />
      </div>
    </div>
  </NModal>
</template>
