<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon, NPopconfirm, NTag } from 'naive-ui'
import { Code2, Database, Trash2 } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useProjectStore } from '@/stores/project'
import type { ProjectRecord } from '@/types/domain'

const props = defineProps<{
  item: ProjectRecord
}>()

const emit = defineEmits<{
  select: [id: number]
  embed: [id: number]
}>()

const workspace = useWorkspaceStore()
const projectStore = useProjectStore()
const { context } = storeToRefs(workspace)
const { projectDatasources } = storeToRefs(projectStore)

const isSelected = computed(() => props.item.id === context.value.projectId)
const datasourceLabel = computed(() =>
  isSelected.value ? `已绑定 ${projectDatasources.value.total} 个数据源` : '点击查看项目数据源'
)
</script>

<template>
  <article class="project-card" :class="{ selected: isSelected }" @click="emit('select', item.id)">
    <div class="project-card-top">
      <div class="project-card-head">
        <div class="project-icon">{{ item.name.slice(0, 1) }}</div>
        <div class="project-card-title">
          <h3>{{ item.name }}</h3>
          <p>项目 #{{ item.id }}</p>
        </div>
      </div>
      <NTag size="small" round :type="isSelected ? 'success' : 'default'" :bordered="false">
        {{ isSelected ? '当前项目' : (item.status || 'active') }}
      </NTag>
    </div>

    <p class="project-description">{{ item.description || '暂无项目说明' }}</p>

    <div class="project-card-meta">
      <NIcon :component="Database" />
      <span>{{ datasourceLabel }}</span>
    </div>

    <div class="project-card-footer">
      <div class="project-provider-tags">
        <NTag size="small" round :type="projectStore.projectProviderTagType(item.id, 'llm')" :bordered="false">
          {{ projectStore.projectProviderLabel(item.id, 'llm') }}
        </NTag>
        <NTag size="small" round :type="projectStore.projectProviderTagType(item.id, 'asr')" :bordered="false">
          {{ projectStore.projectProviderLabel(item.id, 'asr') }}
        </NTag>
        <NTag size="small" round :type="projectStore.projectProviderTagType(item.id, 'tts')" :bordered="false">
          {{ projectStore.projectProviderLabel(item.id, 'tts') }}
        </NTag>
      </div>
      <div class="card-actions" @click.stop>
        <NButton size="small" secondary @click="emit('embed', item.id)">
          <template #icon>
            <NIcon :component="Code2" />
          </template>
          内嵌
        </NButton>
        <NPopconfirm @positive-click="projectStore.deleteProject(item)">
          <template #trigger>
            <NButton size="small" type="error" quaternary>
              <template #icon>
                <NIcon :component="Trash2" />
              </template>
              删除
            </NButton>
          </template>
          删除后会移除这个项目的会话、成员绑定、业务知识和 AI 配置，确定删除吗？
        </NPopconfirm>
      </div>
    </div>
  </article>
</template>
