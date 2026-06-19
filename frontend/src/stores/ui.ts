import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import type {
  AuditSubKey,
  KnowledgeSubKey,
  MemberSubKey,
  ModuleKey,
  SavedNavState
} from '@/stores/types'

const NAV_STATE_KEY = 'ling_shu_management_nav_state'

const moduleKeys: ModuleKey[] = ['project', 'datasource', 'chat', 'members', 'knowledge', 'audit']
const memberSubKeys: MemberSubKey[] = ['invite', 'projectAccess', 'directory']
const knowledgeSubKeys: KnowledgeSubKey[] = ['terms', 'metrics', 'fewShots', 'rag']
const auditSubKeys: AuditSubKey[] = ['operationLogs', 'queryExecutions']

function readSavedNavState(): SavedNavState {
  try {
    const raw = localStorage.getItem(NAV_STATE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as SavedNavState
    return {
      activeModule: moduleKeys.includes(parsed.activeModule as ModuleKey) ? parsed.activeModule : undefined,
      activeMemberSub: memberSubKeys.includes(parsed.activeMemberSub as MemberSubKey) ? parsed.activeMemberSub : undefined,
      activeKnowledgeSub: knowledgeSubKeys.includes(parsed.activeKnowledgeSub as KnowledgeSubKey) ? parsed.activeKnowledgeSub : undefined,
      activeAuditSub: auditSubKeys.includes(parsed.activeAuditSub as AuditSubKey) ? parsed.activeAuditSub : undefined
    }
  } catch {
    return {}
  }
}

export const useUiStore = defineStore('ui', () => {
  const savedNavState = readSavedNavState()

  const activeModule = ref<ModuleKey>(savedNavState.activeModule || 'chat')
  const activeMemberSub = ref<MemberSubKey>(savedNavState.activeMemberSub || 'invite')
  const activeKnowledgeSub = ref<KnowledgeSubKey>(savedNavState.activeKnowledgeSub || 'terms')
  const activeAuditSub = ref<AuditSubKey>(savedNavState.activeAuditSub || 'operationLogs')
  const sidebarCollapsed = ref(false)

  function saveNavState() {
    localStorage.setItem(
      NAV_STATE_KEY,
      JSON.stringify({
        activeModule: activeModule.value,
        activeMemberSub: activeMemberSub.value,
        activeKnowledgeSub: activeKnowledgeSub.value,
        activeAuditSub: activeAuditSub.value
      })
    )
  }

  watch([activeModule, activeMemberSub, activeKnowledgeSub, activeAuditSub], saveNavState)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  return {
    activeModule,
    activeMemberSub,
    activeKnowledgeSub,
    activeAuditSub,
    sidebarCollapsed,
    toggleSidebar
  }
})
