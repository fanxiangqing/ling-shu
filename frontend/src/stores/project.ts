import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { projectApi, providerApi, datasourceApi } from '@/api/resources'
import type { DataSourceRecord, PageResult, ProjectRecord } from '@/types/domain'
import { emptyPage, generateProjectCode } from '@/utils/format'
import { fetchAllPages } from '@/utils/pagination'
import { notify } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'
import { useChatStore } from '@/stores/chat'
import { useDatasourceStore } from '@/stores/datasource'
import { useMemberStore } from '@/stores/member'
import { useKnowledgeStore } from '@/stores/knowledge'
import type { ProjectProviderModes, ProviderConfigMode } from '@/stores/types'

type ProviderConfigRecord = { id?: number; enabled?: boolean }

export const useProjectStore = defineStore('project', () => {
  const ws = useWorkspaceStore()

  const projects = ref<PageResult<ProjectRecord>>(emptyPage())
  const projectDatasources = ref<PageResult<DataSourceRecord>>(emptyPage())
  const projectOptionItems = ref<ProjectRecord[]>([])
  const projectDatasourceOptionItems = ref<DataSourceRecord[]>([])
  const projectOptionTenantId = ref(0)
  const projectDatasourceOptionScope = ref('')
  const projectProviderModes = ref<Record<number, ProjectProviderModes>>({})

  const projectSearch = ref('')
  const projectModalVisible = ref(false)
  const projectDatasourceModalVisible = ref(false)

  const projectForm = reactive({
    name: '电商经营项目',
    description: '订单、商品、用户与渠道',
    datasource_ids: [] as number[],
    llm_mode: 'global' as ProviderConfigMode,
    llm_model: 'qwen-plus',
    llm_api_base: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    llm_api_key: '',
    asr_mode: 'global' as ProviderConfigMode,
    asr_model: 'nls-realtime-asr',
    asr_access_key_id: '',
    asr_access_key_secret: '',
    asr_app_key: '',
    tts_mode: 'global' as ProviderConfigMode,
    tts_model: 'nls-tts',
    tts_voice: 'aixia',
    tts_access_key_id: '',
    tts_access_key_secret: '',
    tts_app_key: ''
  })

  const projectOptionRecords = computed(() => projectOptionItems.value.length ? projectOptionItems.value : projects.value.items)
  const projectDatasourceOptionRecords = computed(() =>
    projectDatasourceOptionItems.value.length ? projectDatasourceOptionItems.value : projectDatasources.value.items
  )
  const projectOptions = computed(() => projectOptionRecords.value.map((item) => ({ label: item.name, value: item.id })))
  const projectDatasourceOptions = computed(() =>
    projectDatasourceOptionRecords.value.map((item) => ({ label: `${item.name} (${item.db_type})`, value: item.id }))
  )
  const selectedProject = computed(() => projectOptionRecords.value.find((item) => item.id === ws.context.projectId))
  const projectSelectable = computed(() => Boolean(ws.context.tenantId && projectOptionRecords.value.length))
  const filteredProjects = computed(() => {
    const keyword = projectSearch.value.trim().toLowerCase()
    if (!keyword) return projects.value.items
    return projects.value.items.filter((item) => `${item.name} ${item.code} ${item.description || ''}`.toLowerCase().includes(keyword))
  })

  async function refreshProjects(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId) {
      projects.value = emptyPage()
      projectOptionItems.value = []
      projectOptionTenantId.value = 0
      ws.context.projectId = 0
      return
    }
    const result = await ws.run('刷新项目', () => projectApi.list(ws.context.tenantId, ws.pageParams('projects')), options)
    if (!result) return
    projects.value = result as PageResult<ProjectRecord>
    ws.syncPage('projects', projects.value)
    await refreshProjectOptions()
    if (!ws.context.projectId) ws.context.projectId = projectOptionRecords.value[0]?.id || 0
    const chat = useChatStore()
    if (!chat.chatForm.project_id) chat.chatForm.project_id = ws.context.projectId
    await refreshProjectProviderModes()
  }

  async function refreshProjectOptions() {
    if (!ws.context.tenantId) {
      projectOptionItems.value = []
      projectOptionTenantId.value = 0
      return
    }
    if (projectOptionTenantId.value !== ws.context.tenantId) {
      projectOptionItems.value = []
      projectOptionTenantId.value = ws.context.tenantId
    }
    const result = await ws.run(
      '刷新项目选项',
      () => fetchAllPages<ProjectRecord>((params) => projectApi.list(ws.context.tenantId, params)),
      { silent: true, successMessage: false }
    )
    if (result) projectOptionItems.value = result as ProjectRecord[]
  }

  async function refreshProjectDatasources(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId || !ws.context.projectId) {
      projectDatasources.value = emptyPage()
      projectDatasourceOptionItems.value = []
      projectDatasourceOptionScope.value = ''
      return
    }
    const result = await ws.run(
      '刷新项目数据源',
      () => datasourceApi.listProject(ws.context.projectId, ws.context.tenantId, ws.pageParams('projectDatasources')),
      options
    )
    if (!result) return
    projectDatasources.value = result as PageResult<DataSourceRecord>
    ws.syncPage('projectDatasources', projectDatasources.value)
    await refreshProjectDatasourceOptions()
  }

  async function refreshProjectDatasourceOptions() {
    if (!ws.context.tenantId || !ws.context.projectId) {
      projectDatasourceOptionItems.value = []
      projectDatasourceOptionScope.value = ''
      return
    }
    const scope = `${ws.context.tenantId}:${ws.context.projectId}`
    if (projectDatasourceOptionScope.value !== scope) {
      projectDatasourceOptionItems.value = []
      projectDatasourceOptionScope.value = scope
    }
    const result = await ws.run(
      '刷新项目数据源选项',
      () => fetchAllPages<DataSourceRecord>((params) => datasourceApi.listProject(ws.context.projectId, ws.context.tenantId, params)),
      { silent: true, successMessage: false }
    )
    if (result) projectDatasourceOptionItems.value = result as DataSourceRecord[]
  }

  function providerConfigMode(result: PromiseSettledResult<Record<string, unknown>>): ProviderConfigMode {
    if (result.status !== 'fulfilled') return 'global'
    const config = result.value as ProviderConfigRecord
    if (!config || !config.id) return 'global'
    return config.enabled === false ? 'disabled' : 'custom'
  }

  async function refreshProjectProviderModes() {
    if (!ws.context.tenantId || !projects.value.items.length) return
    const entries = await Promise.all(projects.value.items.map(async (project) => {
      const [llm, asr, tts] = await Promise.allSettled([
        providerApi.getLLM(project.id, ws.context.tenantId),
        providerApi.getASR(project.id, ws.context.tenantId),
        providerApi.getTTS(project.id, ws.context.tenantId)
      ])
      return [
        project.id,
        {
          llm: providerConfigMode(llm),
          asr: providerConfigMode(asr),
          tts: providerConfigMode(tts)
        }
      ] as const
    }))
    projectProviderModes.value = {
      ...projectProviderModes.value,
      ...Object.fromEntries(entries)
    }
  }

  function projectProviderLabel(projectID: number, provider: keyof ProjectProviderModes) {
    const mode = projectProviderModes.value[projectID]?.[provider] || 'global'
    const name = provider.toUpperCase()
    if (mode === 'custom') return `${name} 项目`
    if (mode === 'disabled') return `${name} 关闭`
    return `${name} Global`
  }

  function projectProviderTagType(projectID: number, provider: keyof ProjectProviderModes): 'success' | 'info' | 'warning' {
    const mode = projectProviderModes.value[projectID]?.[provider] || 'global'
    if (mode === 'custom') return 'info'
    if (mode === 'disabled') return 'warning'
    return 'success'
  }

  function validateProjectProviderForm() {
    if (projectForm.llm_mode === 'custom' && !projectForm.llm_api_key.trim()) {
      notify.warning('项目自定义 LLM 配置需要填写 API Key')
      return false
    }
    if (
      projectForm.asr_mode === 'custom' &&
      (!projectForm.asr_access_key_id.trim() || !projectForm.asr_access_key_secret.trim() || !projectForm.asr_app_key.trim())
    ) {
      notify.warning('项目自定义 ASR 配置需要填写 AccessKey 和 AppKey')
      return false
    }
    if (
      projectForm.tts_mode === 'custom' &&
      (!projectForm.tts_access_key_id.trim() || !projectForm.tts_access_key_secret.trim() || !projectForm.tts_app_key.trim())
    ) {
      notify.warning('项目自定义 TTS 配置需要填写 AccessKey 和 AppKey')
      return false
    }
    return true
  }

  async function configureProjectProviders(projectID: number) {
    if (projectForm.llm_mode === 'custom') {
      await providerApi.upsertLLM(projectID, {
        tenant_id: ws.context.tenantId,
        provider: 'aliyun',
        model: projectForm.llm_model,
        api_base: projectForm.llm_api_base,
        api_key: projectForm.llm_api_key,
        enabled: true
      })
    }

    if (projectForm.asr_mode === 'custom') {
      await providerApi.upsertASR(projectID, {
        tenant_id: ws.context.tenantId,
        provider: 'aliyun',
        model: projectForm.asr_model,
        access_key_id: projectForm.asr_access_key_id,
        access_key_secret: projectForm.asr_access_key_secret,
        app_key: projectForm.asr_app_key,
        format: 'pcm',
        sample_rate: 16000,
        enabled: true
      })
    } else if (projectForm.asr_mode === 'disabled') {
      await providerApi.upsertASR(projectID, {
        tenant_id: ws.context.tenantId,
        provider: 'aliyun',
        enabled: false
      })
    }

    if (projectForm.tts_mode === 'custom') {
      await providerApi.upsertTTS(projectID, {
        tenant_id: ws.context.tenantId,
        provider: 'aliyun',
        model: projectForm.tts_model,
        voice: projectForm.tts_voice,
        access_key_id: projectForm.tts_access_key_id,
        access_key_secret: projectForm.tts_access_key_secret,
        app_key: projectForm.tts_app_key,
        format: 'mp3',
        sample_rate: 16000,
        enabled: true
      })
    } else if (projectForm.tts_mode === 'disabled') {
      await providerApi.upsertTTS(projectID, {
        tenant_id: ws.context.tenantId,
        provider: 'aliyun',
        enabled: false
      })
    }
  }

  function rememberProjectProviderModes(projectID: number) {
    projectProviderModes.value[projectID] = {
      llm: projectForm.llm_mode,
      asr: projectForm.asr_mode,
      tts: projectForm.tts_mode
    }
  }

  async function createProject() {
    if (!ws.ensureTenant()) return
    if (!projectForm.name.trim()) return notify.warning('请输入项目名称')
    if (!projectForm.datasource_ids.length) return notify.warning('创建项目时必须选择至少一个数据源')
    if (!validateProjectProviderForm()) return
    const projectCode = generateProjectCode(projectForm.name)
    const result = await ws.run('创建项目', async () => {
      const project = await projectApi.create({
        tenant_id: ws.context.tenantId,
        name: projectForm.name,
        code: projectCode,
        description: projectForm.description,
        datasource_ids: projectForm.datasource_ids
      })
      await configureProjectProviders(project.id)
      return project
    })
    if (!result) return
    projectModalVisible.value = false
    await refreshProjects()
    const chat = useChatStore()
    const member = useMemberStore()
    const id = Number((result as ProjectRecord | null)?.id || 0)
    if (id > 0) {
      ws.context.projectId = id
      chat.chatForm.project_id = id
      rememberProjectProviderModes(id)
    }
    await refreshProjectDatasources()
    await member.refreshMembers({ silent: true })
  }

  async function deleteProject(project: ProjectRecord) {
    const result = await ws.run('删除项目', () => projectApi.delete(project.id, ws.context.tenantId))
    if (!result) return
    const chat = useChatStore()
    const datasource = useDatasourceStore()
    const knowledge = useKnowledgeStore()
    const member = useMemberStore()
    if (ws.context.projectId === project.id) {
      ws.context.projectId = 0
      chat.chatForm.project_id = 0
      ws.context.datasourceId = 0
      ws.context.sessionId = 0
      chat.messages = []
      chat.latestResult = null
      datasource.resetMetadataPreview()
      knowledge.clearItems()
    }
    await refreshProjects({ silent: true })
    await refreshProjectDatasources({ silent: true })
    await chat.refreshSessions({ silent: true })
    await member.refreshMembers({ silent: true })
  }

  return {
    projects,
    projectDatasources,
    projectOptionItems,
    projectDatasourceOptionItems,
    projectProviderModes,
    projectSearch,
    projectModalVisible,
    projectDatasourceModalVisible,
    projectForm,
    projectOptions,
    projectDatasourceOptions,
    selectedProject,
    projectSelectable,
    filteredProjects,
    refreshProjects,
    refreshProjectOptions,
    refreshProjectDatasources,
    refreshProjectDatasourceOptions,
    refreshProjectProviderModes,
    rememberProjectProviderModes,
    projectProviderLabel,
    projectProviderTagType,
    validateProjectProviderForm,
    configureProjectProviders,
    createProject,
    deleteProject
  }
})
