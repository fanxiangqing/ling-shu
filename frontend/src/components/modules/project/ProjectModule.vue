<script setup lang="ts">
import { NButton, NIcon, NInput, NPagination, NSpace, NTag } from 'naive-ui'
import { Building2, Search } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { pageSize, showPagination } from '@/utils/format'
import ProjectCard from '@/components/modules/project/ProjectCard.vue'
import ProjectDatasourceModal from '@/components/modules/project/ProjectDatasourceModal.vue'
import ProjectEmbedModal from '@/components/modules/project/ProjectEmbedModal.vue'
import ProjectFormModal from '@/components/modules/project/ProjectFormModal.vue'

const workspace = useWorkspaceStore()
const ui = useUiStore()
const projectStore = useProjectStore()

const { pageState } = storeToRefs(workspace)
const { projects, projectSearch, projectModalVisible, filteredProjects } = storeToRefs(projectStore)

async function openProjectDatasources(id: number) {
  await workspace.handleProjectChange(id)
  projectStore.projectDatasourceModalVisible = true
}
</script>

<template>
  <section class="module-page project-page">
    <div class="config-banner">
      <div>
        <span>项目空间</span>
        <h2>创建项目时，选择这个项目可以使用的数据源</h2>
        <p>项目代表一个业务分析空间。数据源先在“数据源管理”里接入和同步，项目这里只负责选择可用范围。</p>
      </div>
      <NSpace>
        <NButton type="primary" @click="projectModalVisible = true">创建项目</NButton>
        <NButton secondary @click="ui.activeModule = 'datasource'">去管理数据源</NButton>
        <NButton type="primary" secondary @click="ui.activeModule = 'chat'">去创建会话</NButton>
      </NSpace>
    </div>

    <section class="surface project-list-surface">
      <div class="surface-head">
        <div>
          <h2>项目列表</h2>
          <p class="surface-note">每个项目都是一个独立问数空间，包含数据源范围、会话、成员和 AI 配置。</p>
        </div>
        <NTag size="small">{{ filteredProjects.length }} / {{ projects.total }} 个</NTag>
      </div>
      <div class="project-toolbar">
        <NInput v-model:value="projectSearch" clearable placeholder="搜索项目名称或说明">
          <template #prefix>
            <NIcon :component="Search" />
          </template>
        </NInput>
        <NButton type="primary" secondary @click="() => projectStore.refreshProjects()">刷新</NButton>
      </div>
      <div v-if="filteredProjects.length" class="project-card-grid">
        <ProjectCard
          v-for="project in filteredProjects"
          :key="project.id"
          :item="project"
          @select="openProjectDatasources"
          @embed="projectStore.openEmbedModal"
        />
      </div>
      <div v-else class="empty-state compact">
        <NIcon :component="Building2" />
        <h2>还没有项目</h2>
        <p>点击“创建项目”，选择数据源并确认 LLM/ASR/TTS 配置。</p>
        <NButton type="primary" @click="projectModalVisible = true">创建项目</NButton>
      </div>
      <div v-if="showPagination(projects)" class="pager-row">
        <NPagination
          :page="pageState.projects"
          :page-size="pageSize(projects)"
          :item-count="projects.total"
          @update:page="(page) => workspace.changePage('projects', page, projectStore.refreshProjects)"
        />
      </div>
    </section>

    <ProjectFormModal />

    <ProjectDatasourceModal />

    <ProjectEmbedModal />
  </section>
</template>
