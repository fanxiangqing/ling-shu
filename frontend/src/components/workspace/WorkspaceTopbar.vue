<script setup lang="ts">
import { computed } from 'vue'
import { NIcon, NSelect } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { modules } from '@/components/workspace/navigation'

const workspace = useWorkspaceStore()
const ui = useUiStore()
const project = useProjectStore()

const { context } = storeToRefs(workspace)
const { activeModule } = storeToRefs(ui)
const { projectOptions, projectSelectable } = storeToRefs(project)

const activeModuleMeta = computed(() => modules.find((item) => item.key === activeModule.value) || modules[0])
</script>

<template>
  <header class="workspace-topbar">
    <div>
      <div class="module-kicker">
        <NIcon :component="activeModuleMeta.icon" />
        {{ activeModuleMeta.hint }}
      </div>
      <h1>{{ activeModuleMeta.label }}</h1>
    </div>
    <div v-if="activeModule !== 'project'" class="topbar-project-picker">
      <span>当前项目</span>
      <NSelect
        :value="context.projectId || null"
        :options="projectOptions"
        :disabled="!projectSelectable"
        filterable
        clearable
        placeholder="选择项目"
        @update:value="workspace.handleProjectChange"
      />
    </div>
  </header>
</template>
