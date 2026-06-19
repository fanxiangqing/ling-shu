<script setup lang="ts">
import { NButton, NIcon, NSelect } from 'naive-ui'
import { ChevronLeft, ChevronRight, LogOut } from '@lucide/vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useAuditStore } from '@/stores/audit'
import { useMemberStore } from '@/stores/member'
import type { AuditSubKey, KnowledgeSubKey, MemberSubKey, ModuleKey } from '@/stores/types'
import type { LoginResult } from '@/types/domain'
import { auditSubmenus, knowledgeSubmenus, memberSubmenus, modules } from '@/components/workspace/navigation'

defineProps<{
  login: LoginResult
}>()

const emit = defineEmits<{
  logout: []
}>()

const workspace = useWorkspaceStore()
const ui = useUiStore()
const audit = useAuditStore()
const member = useMemberStore()

function resetMemberPaging() {
  workspace.resetPage('tenantMembers')
  workspace.resetPage('projectMembers')
  void member.refreshMembers({ silent: true })
}

const { context, tenantOptions, selectedTenant } = storeToRefs(workspace)
const { activeModule, activeMemberSub, activeKnowledgeSub, activeAuditSub, sidebarCollapsed } = storeToRefs(ui)

function selectModule(key: ModuleKey) {
  ui.activeModule = key
  if (key === 'members') {
    ui.activeMemberSub = 'invite'
    resetMemberPaging()
  }
  if (key === 'knowledge') ui.activeKnowledgeSub = 'terms'
  if (key === 'audit') void audit.refreshAudit({ silent: true })
}

function selectMemberSub(key: MemberSubKey) {
  ui.activeModule = 'members'
  if (ui.activeMemberSub !== key) resetMemberPaging()
  ui.activeMemberSub = key
}

function selectKnowledgeSub(key: KnowledgeSubKey) {
  ui.activeModule = 'knowledge'
  ui.activeKnowledgeSub = key
}

function selectAuditSub(key: AuditSubKey) {
  ui.activeModule = 'audit'
  ui.activeAuditSub = key
  void audit.refreshAudit({ silent: true })
}
</script>

<template>
  <aside class="workspace-sidebar">
    <div class="workspace-brand">
      <div class="workspace-mark">灵</div>
      <div v-if="!sidebarCollapsed">
        <div class="brand-name">Ling-Shu</div>
        <div class="brand-sub">自然语言问数平台</div>
      </div>
      <NButton
        class="sidebar-toggle"
        quaternary
        circle
        size="small"
        :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'"
        :aria-label="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'"
        @click="ui.toggleSidebar()"
      >
        <template #icon>
          <NIcon :component="sidebarCollapsed ? ChevronRight : ChevronLeft" />
        </template>
      </NButton>
    </div>

    <section v-if="!sidebarCollapsed" class="workspace-switcher">
      <div class="switcher-label">工作空间</div>
      <NSelect
        :value="context.tenantId || null"
        :options="tenantOptions"
        size="small"
        filterable
        placeholder="选择组织"
        @update:value="workspace.handleTenantChange"
      />
      <p>
        当前组织：
        <strong>{{ selectedTenant?.name || '未选择' }}</strong>
      </p>
    </section>

    <nav class="module-nav">
      <div v-for="item in modules" :key="item.key" class="module-nav-group">
        <button
          class="module-button"
          :class="{ active: activeModule === item.key }"
          type="button"
          :title="item.label"
          :aria-label="item.label"
          @click="selectModule(item.key)"
        >
          <NIcon :component="item.icon" />
          <span v-if="!sidebarCollapsed">
            <strong>{{ item.label }}</strong>
            <em>{{ item.hint }}</em>
          </span>
        </button>

        <div v-if="!sidebarCollapsed && item.key === 'members' && activeModule === 'members'" class="module-subnav">
          <button
            v-for="child in memberSubmenus"
            :key="child.key"
            type="button"
            :class="{ active: activeMemberSub === child.key }"
            @click="selectMemberSub(child.key)"
          >
            <strong>{{ child.label }}</strong>
            <em>{{ child.hint }}</em>
          </button>
        </div>

        <div v-if="!sidebarCollapsed && item.key === 'knowledge' && activeModule === 'knowledge'" class="module-subnav">
          <button
            v-for="child in knowledgeSubmenus"
            :key="child.key"
            type="button"
            :class="{ active: activeKnowledgeSub === child.key }"
            @click="selectKnowledgeSub(child.key)"
          >
            <strong>{{ child.label }}</strong>
            <em>{{ child.hint }}</em>
          </button>
        </div>

        <div v-if="!sidebarCollapsed && item.key === 'audit' && activeModule === 'audit'" class="module-subnav">
          <button
            v-for="child in auditSubmenus"
            :key="child.key"
            type="button"
            :class="{ active: activeAuditSub === child.key }"
            @click="selectAuditSub(child.key)"
          >
            <strong>{{ child.label }}</strong>
            <em>{{ child.hint }}</em>
          </button>
        </div>
      </div>
    </nav>

    <div class="sidebar-footer">
      <div v-if="!sidebarCollapsed">
        <strong>{{ login.user.display_name || login.user.username }}</strong>
        <span>账号 #{{ login.user.id }}</span>
      </div>
      <NButton
        quaternary
        size="small"
        :circle="sidebarCollapsed"
        title="退出登录"
        aria-label="退出登录"
        @click="emit('logout')"
      >
        <template #icon>
          <NIcon :component="LogOut" />
        </template>
        <span v-if="!sidebarCollapsed">退出</span>
      </NButton>
    </div>
  </aside>
</template>
