import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { tenantApi } from '@/api/resources'
import type { PageParams } from '@/api/resources'
import type { PageResult, TenantRecord } from '@/types/domain'
import { DEFAULT_PAGE_SIZE, emptyPage } from '@/utils/format'
import { fetchAllPages } from '@/utils/pagination'
import { notify } from '@/composables/useNotify'
import type { PageKey, RefreshFn } from '@/stores/types'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useDatasourceStore } from '@/stores/datasource'
import { useChatStore } from '@/stores/chat'
import { useMemberStore } from '@/stores/member'
import { useKnowledgeStore } from '@/stores/knowledge'
import { useAuditStore } from '@/stores/audit'

type RunOptions = {
  silent?: boolean
  successMessage?: string | false
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const loading = ref(false)
  const context = reactive({
    tenantId: 0,
    projectId: 0,
    datasourceId: 0,
    sessionId: 0,
    userId: 0
  })

  const pageState = reactive<Record<PageKey, number>>({
    tenants: 1,
    projects: 1,
    datasources: 1,
    projectDatasources: 1,
    metadataTables: 1,
    users: 1,
    tenantMembers: 1,
    projectMembers: 1,
    sessions: 1,
    terms: 1,
    metrics: 1,
    fewShots: 1,
    auditLogs: 1,
    auditQueries: 1
  })

  const tenants = ref<PageResult<TenantRecord>>(emptyPage())
  const tenantOptionItems = ref<TenantRecord[]>([])

  const tenantOptionRecords = computed(() => tenantOptionItems.value.length ? tenantOptionItems.value : tenants.value.items)
  const tenantOptions = computed(() => tenantOptionRecords.value.map((item) => ({ label: `${item.name} / ${item.code}`, value: item.id })))
  const selectedTenant = computed(() => tenantOptionRecords.value.find((item) => item.id === context.tenantId))
  const workspaceReady = computed(() => Boolean(context.tenantId && context.projectId))

  function setUserId(userId: number) {
    context.userId = userId
  }

  function pageParams(key: PageKey): PageParams {
    return { page: pageState[key], page_size: DEFAULT_PAGE_SIZE }
  }

  function syncPage<T>(key: PageKey, result: PageResult<T>) {
    pageState[key] = result.page || pageState[key] || 1
  }

  function resetPage(key: PageKey) {
    pageState[key] = 1
  }

  async function changePage(key: PageKey, page: number, refresh: RefreshFn) {
    if (!page || page === pageState[key]) return
    pageState[key] = page
    await refresh({ silent: true })
  }

  async function run(action: string, task: () => Promise<unknown>, options: RunOptions = {}) {
    loading.value = true
    try {
      const result = await task()
      if (!options.silent && options.successMessage !== false) notify.success(options.successMessage || `${action}成功`)
      return result
    } catch (error) {
      const text = error instanceof Error ? error.message : `${action}失败`
      if (!options.silent) notify.error(text)
      return null
    } finally {
      loading.value = false
    }
  }

  function ensureTenant() {
    if (context.tenantId > 0) return true
    notify.warning('请先选择组织')
    useUiStore().activeModule = 'project'
    return false
  }

  function ensureProject() {
    if (!ensureTenant()) return false
    if (context.projectId > 0) return true
    notify.warning('请先创建或选择项目')
    return false
  }

  async function refreshTenants(options: { silent?: boolean } = {}) {
    const result = await run('刷新组织', () => tenantApi.list(pageParams('tenants')), options)
    if (!result) return
    tenants.value = result as PageResult<TenantRecord>
    syncPage('tenants', tenants.value)
    await refreshTenantOptions()
    const choices = tenantOptionRecords.value
    if (!context.tenantId) context.tenantId = choices[0]?.id || 0
  }

  async function refreshTenantOptions() {
    const result = await run(
      '刷新组织选项',
      () => fetchAllPages<TenantRecord>((params) => tenantApi.list(params)),
      { silent: true, successMessage: false }
    )
    if (result) tenantOptionItems.value = result as TenantRecord[]
  }

  async function handleTenantChange(value: number | null) {
    const project = useProjectStore()
    const datasource = useDatasourceStore()
    const chat = useChatStore()
    const member = useMemberStore()
    const knowledge = useKnowledgeStore()
    const audit = useAuditStore()

    context.tenantId = Number(value || 0)
    context.projectId = 0
    context.datasourceId = 0
    context.sessionId = 0
    chat.chatForm.project_id = 0
    chat.messages = []
    chat.latestResult = null
    datasource.resetMetadataPreview()
    knowledge.clear()
    audit.clear()
    await project.refreshProjects({ silent: true })
    await datasource.refreshTenantDatasources({ silent: true })
    await project.refreshProjectDatasources({ silent: true })
    await chat.refreshSessions({ silent: true })
    await member.refreshMembers({ silent: true })
  }

  async function handleProjectChange(value: number | null) {
    const ui = useUiStore()
    const project = useProjectStore()
    const datasource = useDatasourceStore()
    const chat = useChatStore()
    const member = useMemberStore()
    const knowledge = useKnowledgeStore()
    const audit = useAuditStore()

    context.projectId = Number(value || 0)
    chat.chatForm.project_id = context.projectId
    context.datasourceId = 0
    context.sessionId = 0
    resetPage('projectDatasources')
    resetPage('projectMembers')
    resetPage('sessions')
    resetPage('terms')
    resetPage('metrics')
    resetPage('fewShots')
    resetPage('auditLogs')
    resetPage('auditQueries')
    chat.messages = []
    chat.latestResult = null
    datasource.resetMetadataPreview()
    knowledge.clearItems()
    await project.refreshProjectDatasources({ silent: true })
    await member.refreshMembers({ silent: true })
    if (ui.activeModule === 'knowledge' && context.projectId) {
      await knowledge.refreshKnowledge({ silent: true })
    }
    if (ui.activeModule === 'audit') {
      await audit.refreshAudit({ silent: true })
    }
  }

  async function handleDatasourceChange(value: number | null) {
    const ui = useUiStore()
    const datasource = useDatasourceStore()
    const knowledge = useKnowledgeStore()
    const audit = useAuditStore()

    context.datasourceId = Number(value || 0)
    resetPage('metadataTables')
    resetPage('metrics')
    resetPage('fewShots')
    resetPage('auditQueries')
    datasource.resetMetadataPreview()
    if (ui.activeModule === 'knowledge' && context.projectId) {
      await knowledge.refreshKnowledge({ silent: true })
    }
    if (ui.activeModule === 'audit') {
      await audit.refreshAudit({ silent: true })
    }
  }

  async function initializeWorkspace() {
    const ui = useUiStore()
    const project = useProjectStore()
    const datasource = useDatasourceStore()
    const chat = useChatStore()
    const member = useMemberStore()
    const audit = useAuditStore()

    await refreshTenants({ silent: true })
    await project.refreshProjects({ silent: true })
    await Promise.all([
      member.refreshUsers({ silent: true }),
      datasource.refreshTenantDatasources({ silent: true }),
      chat.refreshSessions({ silent: true }),
      member.refreshMembers({ silent: true })
    ])
    await project.refreshProjectDatasources({ silent: true })
    if (ui.activeModule === 'audit') {
      await audit.refreshAudit({ silent: true })
    }
  }

  return {
    loading,
    context,
    pageState,
    tenants,
    tenantOptionItems,
    tenantOptions,
    selectedTenant,
    workspaceReady,
    setUserId,
    pageParams,
    syncPage,
    resetPage,
    changePage,
    run,
    ensureTenant,
    ensureProject,
    refreshTenants,
    refreshTenantOptions,
    handleTenantChange,
    handleProjectChange,
    handleDatasourceChange,
    initializeWorkspace
  }
})
