import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { projectApi, providerApi, datasourceApi, embedApi } from '@/api/resources'
import type {
  DataSourceRecord,
  EmbedAppCreateResult,
  EmbedAppRecord,
  EmbedAppSecretResult,
  EmbedTokenResult,
  PageResult,
  ProjectRecord
} from '@/types/domain'
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
  const embedApps = ref<PageResult<EmbedAppRecord>>(emptyPage())
  const lastCreatedEmbed = ref<EmbedAppCreateResult | null>(null)
  const revealedEmbedSecret = ref<EmbedAppSecretResult | null>(null)

  const projectSearch = ref('')
  const projectModalVisible = ref(false)
  const projectDatasourceModalVisible = ref(false)
  const projectEmbedModalVisible = ref(false)
  const embedProjectId = ref(0)

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

  const embedForm = reactive({
    name: '智能问数机器人',
    allowed_origins: '',
    session_policy: 'context',
    launcher_title: '智能问数',
    welcome_message: '你好，我可以帮你查询当前业务空间的数据。'
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

  async function openEmbedModal(projectID: number) {
    ws.context.projectId = projectID
    embedProjectId.value = projectID
    projectEmbedModalVisible.value = true
    lastCreatedEmbed.value = null
    revealedEmbedSecret.value = null
    await refreshEmbedApps({ silent: true })
  }

  async function refreshEmbedApps(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId || !embedProjectId.value) {
      embedApps.value = emptyPage()
      return
    }
    const result = await ws.run(
      '刷新内嵌应用',
      () => embedApi.listApps(embedProjectId.value, ws.context.tenantId, { page: 1, page_size: 20 }),
      options
    )
    if (result) embedApps.value = result as PageResult<EmbedAppRecord>
  }

  async function createEmbedApp() {
    if (!ws.context.tenantId || !embedProjectId.value) return notify.warning('请选择项目后再创建内嵌应用')
    const origins = embedForm.allowed_origins
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean)
    const result = await ws.run('创建内嵌应用', () =>
      embedApi.createApp(embedProjectId.value, {
        tenant_id: ws.context.tenantId,
        name: embedForm.name,
        allowed_origins: origins,
        session_policy: embedForm.session_policy,
        launcher_title: embedForm.launcher_title,
        welcome_message: embedForm.welcome_message
      })
    )
    if (!result) return
    lastCreatedEmbed.value = result as EmbedAppCreateResult
    revealedEmbedSecret.value = {
      app_id: lastCreatedEmbed.value.app.app_id,
      app_secret: lastCreatedEmbed.value.app_secret
    }
    await refreshEmbedApps({ silent: true })
  }

  function cachedEmbedSecret(app: EmbedAppRecord) {
    if (revealedEmbedSecret.value?.app_id === app.app_id) return revealedEmbedSecret.value.app_secret
    if (lastCreatedEmbed.value?.app.app_id === app.app_id) return lastCreatedEmbed.value.app_secret
    return ''
  }

  async function revealEmbedSecret(app: EmbedAppRecord, options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId || !embedProjectId.value) return
    const result = await ws.run(
      '查看 App Secret',
      () => embedApi.revealAppSecret(embedProjectId.value, app.id, ws.context.tenantId),
      { silent: options.silent ?? true, successMessage: false }
    )
    if (!result) return null
    revealedEmbedSecret.value = result as EmbedAppSecretResult
    return revealedEmbedSecret.value
  }

  async function resolveEmbedSecret(app: EmbedAppRecord, options: { silent?: boolean } = {}) {
    const cached = cachedEmbedSecret(app)
    if (cached) return cached
    const revealed = await revealEmbedSecret(app, options)
    return revealed?.app_secret || ''
  }

  async function createEmbedIntegrationTest(app: EmbedAppRecord, input: {
    external_user_id: string
    external_user_name?: string
    ttl_seconds?: number
  }) {
    if (!ws.context.tenantId || !embedProjectId.value) return null
    const appSecret = await resolveEmbedSecret(app, { silent: false })
    if (!appSecret) return null
    const result = await ws.run(
      '签发测试 Token',
      () => embedApi.createToken({
        app_id: app.app_id,
        app_secret: appSecret,
        external_user_id: input.external_user_id,
        external_user_name: input.external_user_name,
        ttl_seconds: input.ttl_seconds || 3600
      }),
      { successMessage: false }
    )
    return result as EmbedTokenResult | null
  }

  async function updateEmbedAppStatus(app: EmbedAppRecord, status: 'active' | 'disabled') {
    if (!ws.context.tenantId || !embedProjectId.value) return
    const result = await ws.run(
      status === 'active' ? '启用内嵌应用' : '停用内嵌应用',
      () => embedApi.updateAppStatus(embedProjectId.value, app.id, ws.context.tenantId, status)
    )
    if (!result) return
    if (revealedEmbedSecret.value?.app_id === app.app_id && status !== 'active') {
      revealedEmbedSecret.value = null
    }
    await refreshEmbedApps({ silent: true })
  }

  async function deleteEmbedApp(app: EmbedAppRecord) {
    if (!ws.context.tenantId || !embedProjectId.value) return
    const result = await ws.run('删除内嵌应用', () => embedApi.deleteApp(embedProjectId.value, app.id, ws.context.tenantId))
    if (!result) return
    if (revealedEmbedSecret.value?.app_id === app.app_id) revealedEmbedSecret.value = null
    if (lastCreatedEmbed.value?.app.app_id === app.app_id) lastCreatedEmbed.value = null
    await refreshEmbedApps({ silent: true })
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
    projectEmbedModalVisible,
    embedProjectId,
    embedApps,
    lastCreatedEmbed,
    revealedEmbedSecret,
    projectForm,
    embedForm,
    projectOptions,
    projectDatasourceOptions,
    projectOptionRecords,
    selectedProject,
    projectSelectable,
    filteredProjects,
    refreshProjects,
    refreshProjectOptions,
    refreshProjectDatasources,
    refreshProjectDatasourceOptions,
    refreshProjectProviderModes,
    openEmbedModal,
    refreshEmbedApps,
    createEmbedApp,
    revealEmbedSecret,
    resolveEmbedSecret,
    createEmbedIntegrationTest,
    updateEmbedAppStatus,
    deleteEmbedApp,
    rememberProjectProviderModes,
    projectProviderLabel,
    projectProviderTagType,
    validateProjectProviderForm,
    configureProjectProviders,
    createProject,
    deleteProject
  }
})
