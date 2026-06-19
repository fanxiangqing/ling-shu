import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { chatApi } from '@/api/resources'
import type {
  ChatMessage,
  ChatMessageRecord,
  ChatSessionRecord,
  DataSourceOption,
  PageResult,
  SendChatMessageResult
} from '@/types/domain'
import { DEFAULT_PAGE_SIZE, currentChatTitle, emptyPage } from '@/utils/format'
import {
  assistantResultText,
  failedChatResult,
  parseAgentResultMessage,
  pendingChatResult,
  readableMessageContent,
  streamMessageContent
} from '@/utils/chatResult'
import { notify } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'

export const useChatStore = defineStore('chat', () => {
  const ws = useWorkspaceStore()

  const sessions = ref<PageResult<ChatSessionRecord>>(emptyPage())
  const messages = ref<ChatMessage[]>([])
  const latestResult = ref<SendChatMessageResult | null>(null)
  const sessionLoadingMore = ref(false)
  const chatProjectModalVisible = ref(false)
  const maxRows = ref(200)

  const chatForm = reactive({
    project_id: 0
  })

  const visibleSessions = computed(() => sessions.value.items)
  const sessionsHasMore = computed(() => sessions.value.items.length < sessions.value.total)
  const selectedSession = computed(() => sessions.value.items.find((item) => item.id === ws.context.sessionId))

  const chatDatasources = computed<DataSourceOption[]>(() => {
    const project = useProjectStore()
    return project.projectDatasources.items.map((item) => ({
      id: item.id,
      projectId: ws.context.projectId,
      name: item.name,
      type: item.db_type,
      dialect: item.db_type,
      role: 'available',
      tableCount: 0,
      syncedAt: item.last_sync_at || item.last_sync_status || '未同步',
      health: item.status === 'active' ? 'healthy' : 'warning'
    }))
  })

  function sessionProjectName(session: ChatSessionRecord) {
    const project = useProjectStore()
    return project.projects.items.find((item) => item.id === session.project_id)?.name || `项目 #${session.project_id}`
  }

  async function refreshSessions(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId) {
      sessions.value = emptyPage()
      ws.context.sessionId = 0
      return
    }
    ws.pageState.sessions = 1
    const result = await ws.run('刷新会话', () => chatApi.listSessions(ws.context.tenantId, ws.pageParams('sessions')), options)
    if (!result) return
    sessions.value = result as PageResult<ChatSessionRecord>
    ws.syncPage('sessions', sessions.value)
    const exists = sessions.value.items.some((item) => item.id === ws.context.sessionId)
    if (!exists) ws.context.sessionId = 0
  }

  async function loadMoreSessions() {
    if (!ws.context.tenantId || sessionLoadingMore.value || !sessionsHasMore.value) return
    sessionLoadingMore.value = true
    const nextPage = ws.pageState.sessions + 1
    try {
      const result = await ws.run(
        '加载更多会话',
        () => chatApi.listSessions(ws.context.tenantId, { page: nextPage, page_size: DEFAULT_PAGE_SIZE }),
        { silent: true }
      )
      if (!result) return
      const page = result as PageResult<ChatSessionRecord>
      const seen = new Set(sessions.value.items.map((item) => item.id))
      sessions.value = {
        ...page,
        items: [...sessions.value.items, ...page.items.filter((item) => !seen.has(item.id))]
      }
      ws.syncPage('sessions', sessions.value)
    } finally {
      sessionLoadingMore.value = false
    }
  }

  function handleSessionListScroll(event: Event) {
    const target = event.currentTarget as HTMLElement | null
    if (!target) return
    const distanceToBottom = target.scrollHeight - target.scrollTop - target.clientHeight
    if (distanceToBottom <= 48) {
      void loadMoreSessions()
    }
  }

  async function createSession() {
    if (!ws.ensureTenant()) return
    if (!chatForm.project_id) return notify.warning('创建会话前请选择项目')
    const result = await ws.run('创建会话', () =>
      chatApi.createSession(ws.context.tenantId, {
        project_id: chatForm.project_id,
        user_id: ws.context.userId,
        title: currentChatTitle()
      })
    )
    if (!result) return
    chatProjectModalVisible.value = false
    await refreshSessions({ silent: true })
    await enterSession(result as ChatSessionRecord)
  }

  function openNewChatModal() {
    const project = useProjectStore()
    chatForm.project_id = ws.context.projectId || project.projects.items[0]?.id || 0
    chatProjectModalVisible.value = true
  }

  async function enterSession(session: ChatSessionRecord) {
    const project = useProjectStore()
    ws.context.sessionId = session.id
    ws.context.projectId = session.project_id
    chatForm.project_id = session.project_id
    latestResult.value = null
    await project.refreshProjectDatasources({ silent: true })
    await loadMessages()
    useUiStore().activeModule = 'chat'
  }

  async function loadMessages() {
    if (!selectedSession.value) {
      messages.value = []
      return
    }
    const result = await ws.run(
      '加载会话消息',
      () =>
        chatApi.listMessages(ws.context.sessionId, ws.context.tenantId, selectedSession.value?.project_id || ws.context.projectId, {
          page: 1,
          page_size: 200
        }),
      { silent: true }
    )
    if (!result) {
      messages.value = []
      return
    }
    messages.value = (result as PageResult<ChatMessageRecord>).items.map((item) => {
      const parsed = parseAgentResultMessage(item)
      return {
        id: String(item.id),
        role: item.role === 'user' ? 'user' : 'assistant',
        content: readableMessageContent(item, parsed),
        createdAt: item.created_at || new Date().toISOString(),
        result: parsed || undefined
      }
    })
  }

  async function ask(question: string) {
    const project = useProjectStore()
    if (!selectedSession.value) return notify.warning('请先创建或选择一个会话')
    if (!ws.context.projectId) return notify.warning('会话缺少项目，请重新选择')
    if (!project.projectDatasources.items.length) return notify.warning('当前项目还没有绑定数据源')

    const now = Date.now()
    const pendingId = `assistant-pending-${now}`
    messages.value.push({ id: `user-${now}`, role: 'user', content: question, createdAt: new Date().toISOString() })
    messages.value.push({
      id: pendingId,
      role: 'assistant',
      content: '正在理解问题、选择数据源并生成查询计划。',
      createdAt: new Date().toISOString(),
      pending: true,
      result: pendingChatResult(question, [])
    })
    const payload = {
      tenant_id: ws.context.tenantId,
      project_id: ws.context.projectId,
      user_id: ws.context.userId,
      content: question,
      selected_datasource_ids: project.projectDatasources.items.map((item) => item.id),
      auto_execute: true,
      max_rows: maxRows.value
    }
    let result: SendChatMessageResult
    ws.loading = true
    try {
      result = await chatApi.sendChatMessageStream(ws.context.sessionId, payload, (event) => {
        const pending = messages.value.find((item) => item.id === pendingId)
        if (!pending) return
        const steps = [...(pending.result?.agent.steps || []), event]
        pending.result = pendingChatResult(question, steps)
        pending.content = streamMessageContent(event, pending.content)
      })
    } catch (error) {
      const text = error instanceof Error ? error.message : '问数请求失败'
      const pending = messages.value.find((item) => item.id === pendingId)
      const steps = pending?.result?.agent.steps || []
      const failedMessage: ChatMessage = {
        id: `${pendingId}-failed`,
        role: 'assistant',
        content: `这次问数没有完成：${text}`,
        createdAt: new Date().toISOString(),
        result: failedChatResult(question, text, steps)
      }
      const pendingIndex = messages.value.findIndex((item) => item.id === pendingId)
      if (pendingIndex >= 0) {
        messages.value.splice(pendingIndex, 1, failedMessage)
      } else {
        messages.value.push(failedMessage)
      }
      notify.error(text)
      return
    } finally {
      ws.loading = false
    }
    latestResult.value = result as SendChatMessageResult
    const assistantMessage: ChatMessage = {
      id: `assistant-${Date.now()}`,
      role: 'assistant',
      content: assistantResultText(latestResult.value),
      createdAt: new Date().toISOString(),
      result: latestResult.value
    }
    const pendingIndex = messages.value.findIndex((item) => item.id === pendingId)
    if (pendingIndex >= 0) {
      messages.value.splice(pendingIndex, 1, assistantMessage)
    } else {
      messages.value.push(assistantMessage)
    }
  }

  return {
    sessions,
    messages,
    latestResult,
    sessionLoadingMore,
    chatProjectModalVisible,
    maxRows,
    chatForm,
    visibleSessions,
    sessionsHasMore,
    selectedSession,
    chatDatasources,
    sessionProjectName,
    refreshSessions,
    loadMoreSessions,
    handleSessionListScroll,
    createSession,
    openNewChatModal,
    enterSession,
    loadMessages,
    ask
  }
})
