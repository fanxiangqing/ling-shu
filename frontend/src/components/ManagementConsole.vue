<script setup lang="ts">
import { onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { storeToRefs } from 'pinia'
import { clearToken } from '@/api/client'
import { setMessageApi } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import type { LoginResult } from '@/types/domain'
import WorkspaceSidebar from '@/components/workspace/WorkspaceSidebar.vue'
import WorkspaceTopbar from '@/components/workspace/WorkspaceTopbar.vue'
import ProjectModule from '@/components/modules/project/ProjectModule.vue'
import DatasourceModule from '@/components/modules/datasource/DatasourceModule.vue'
import ChatModule from '@/components/modules/chat/ChatModule.vue'
import MembersModule from '@/components/modules/members/MembersModule.vue'
import KnowledgeModule from '@/components/modules/knowledge/KnowledgeModule.vue'
import AuditModule from '@/components/modules/audit/AuditModule.vue'

const props = defineProps<{
  login: LoginResult
}>()

const emit = defineEmits<{
  logout: []
}>()

const message = useMessage()
setMessageApi(message)

const workspace = useWorkspaceStore()
const ui = useUiStore()

workspace.setUserId(props.login.user.id)

const { activeModule, sidebarCollapsed } = storeToRefs(ui)

function logout() {
  clearToken()
  emit('logout')
}

onMounted(async () => {
  await workspace.initializeWorkspace()
})
</script>

<template>
  <div class="workspace-shell" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <WorkspaceSidebar :login="login" @logout="logout" />

    <main class="workspace-main">
      <WorkspaceTopbar />

      <ProjectModule v-if="activeModule === 'project'" />
      <DatasourceModule v-else-if="activeModule === 'datasource'" />
      <ChatModule v-else-if="activeModule === 'chat'" />
      <MembersModule v-else-if="activeModule === 'members'" />
      <KnowledgeModule v-else-if="activeModule === 'knowledge'" />
      <AuditModule v-else-if="activeModule === 'audit'" />
    </main>
  </div>
</template>
