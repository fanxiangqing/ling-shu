import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { datasourceApi } from '@/api/resources'
import type {
  DataSourceRecord,
  MetadataColumnRecord,
  MetadataTableRecord,
  PageResult
} from '@/types/domain'
import { emptyPage } from '@/utils/format'
import { fetchAllPages } from '@/utils/pagination'
import { notify } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'
import { useProjectStore } from '@/stores/project'
import { useKnowledgeStore } from '@/stores/knowledge'

export const dbTypeOptions = [
  { label: 'MySQL', value: 'mysql' },
  { label: 'PostgreSQL', value: 'postgresql' },
  { label: 'Oracle', value: 'oracle' },
  { label: 'SQL Server', value: 'sqlserver' },
  { label: 'KingbaseES', value: 'kingbase' },
  { label: 'ClickHouse', value: 'clickhouse' },
  { label: 'Doris', value: 'doris' },
  { label: '达梦 DM8', value: 'dm8' }
]

export const useDatasourceStore = defineStore('datasource', () => {
  const ws = useWorkspaceStore()

  const datasources = ref<PageResult<DataSourceRecord>>(emptyPage())
  const datasourceOptionItems = ref<DataSourceRecord[]>([])
  const datasourceOptionTenantId = ref(0)
  const metadataTables = ref<PageResult<MetadataTableRecord>>(emptyPage())
  const selectedMetadataTable = ref<MetadataTableRecord | null>(null)
  const tableCommentDraft = ref('')
  const columnCommentDrafts = ref<Record<number, string>>({})

  const datasourceSearch = ref('')
  const datasourceTypeFilter = ref<string | null>(null)
  const datasourceModalVisible = ref(false)
  const metadataPreviewVisible = ref(false)

  const datasourceForm = reactive({
    name: 'local-mysql',
    db_type: 'mysql',
    host: '127.0.0.1',
    port: 3306,
    username: 'root',
    password: 'root',
    database: 'ling_shu',
    config_json: ''
  })

  const datasourceOptionRecords = computed(() => datasourceOptionItems.value.length ? datasourceOptionItems.value : datasources.value.items)
  const datasourceOptions = computed(() => datasourceOptionRecords.value.map((item) => ({ label: `${item.name} (${item.db_type})`, value: item.id })))
  const selectedDatasource = computed(() => datasourceOptionRecords.value.find((item) => item.id === ws.context.datasourceId))
  const filteredDatasources = computed(() => {
    const keyword = datasourceSearch.value.trim().toLowerCase()
    const type = datasourceTypeFilter.value
    return datasources.value.items.filter((item) => {
      const matchesKeyword = !keyword || `${item.name} ${item.db_type}`.toLowerCase().includes(keyword)
      const matchesType = !type || item.db_type === type
      return matchesKeyword && matchesType
    })
  })

  function datasourceDefaultPort(dbType: string) {
    const ports: Record<string, number> = {
      mysql: 3306,
      postgresql: 5432,
      oracle: 1521,
      sqlserver: 1433,
      kingbase: 54321,
      dm8: 5236,
      clickhouse: 9000,
      doris: 9030
    }
    return ports[dbType] || 3306
  }

  function buildDatasourceDsn() {
    const host = datasourceForm.host.trim()
    const port = datasourceForm.port
    const user = datasourceForm.username.trim()
    const password = datasourceForm.password
    const database = datasourceForm.database.trim()

    if (datasourceForm.db_type === 'postgresql' || datasourceForm.db_type === 'kingbase') {
      return `host=${host} port=${port} user=${user} password=${password} dbname=${database} sslmode=disable`
    }
    if (datasourceForm.db_type === 'sqlserver') {
      return `sqlserver://${encodeURIComponent(user)}:${encodeURIComponent(password)}@${host}:${port}?database=${encodeURIComponent(database)}`
    }
    if (datasourceForm.db_type === 'oracle') {
      return `oracle://${encodeURIComponent(user)}:${encodeURIComponent(password)}@${host}:${port}/${encodeURIComponent(database)}`
    }
    if (datasourceForm.db_type === 'clickhouse') {
      return `clickhouse://${host}:${port}?username=${encodeURIComponent(user)}&password=${encodeURIComponent(password)}&database=${encodeURIComponent(database)}`
    }
    if (datasourceForm.db_type === 'dm8') {
      return `dm://${encodeURIComponent(user)}:${encodeURIComponent(password)}@${host}:${port}?schema=${encodeURIComponent(database)}&escapeProcess=true`
    }
    return `${user}:${password}@tcp(${host}:${port})/${database}?charset=utf8mb4&parseTime=true&loc=Local`
  }

  function buildDatasourceConfigJson() {
    const raw = datasourceForm.config_json.trim()
    let config: Record<string, unknown> = {}
    if (raw) {
      try {
        const parsed = JSON.parse(raw)
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          config = parsed as Record<string, unknown>
        }
      } catch {
        return raw
      }
    }
    return Object.keys(config).length ? JSON.stringify(config) : ''
  }

  function validateDatasourceForm() {
    if (!ws.ensureTenant()) return false
    if (!datasourceForm.name.trim()) {
      notify.warning('请输入数据源名称')
      return false
    }
    if (!datasourceForm.host.trim()) {
      notify.warning('请输入 Host')
      return false
    }
    if (!datasourceForm.port) {
      notify.warning('请输入 Port')
      return false
    }
    if (!datasourceForm.username.trim()) {
      notify.warning('请输入用户名')
      return false
    }
    if (!datasourceForm.database.trim()) {
      notify.warning('请输入数据库')
      return false
    }
    return true
  }

  async function refreshTenantDatasources(options: { silent?: boolean } = {}) {
    if (!ws.context.tenantId) {
      datasources.value = emptyPage()
      datasourceOptionItems.value = []
      datasourceOptionTenantId.value = 0
      ws.context.datasourceId = 0
      return
    }
    const result = await ws.run('刷新数据源', () => datasourceApi.listTenant(ws.context.tenantId, ws.pageParams('datasources')), options)
    if (!result) return
    datasources.value = result as PageResult<DataSourceRecord>
    ws.syncPage('datasources', datasources.value)
    await refreshDatasourceOptions()
  }

  async function refreshDatasourceOptions() {
    if (!ws.context.tenantId) {
      datasourceOptionItems.value = []
      datasourceOptionTenantId.value = 0
      return
    }
    if (datasourceOptionTenantId.value !== ws.context.tenantId) {
      datasourceOptionItems.value = []
      datasourceOptionTenantId.value = ws.context.tenantId
    }
    const result = await ws.run(
      '刷新数据源选项',
      () => fetchAllPages<DataSourceRecord>((params) => datasourceApi.listTenant(ws.context.tenantId, params)),
      { silent: true, successMessage: false }
    )
    if (result) datasourceOptionItems.value = result as DataSourceRecord[]
  }

  async function createDatasource() {
    if (!validateDatasourceForm()) return
    const result = await ws.run('添加数据源', () =>
      datasourceApi.createForTenant(ws.context.tenantId, {
        name: datasourceForm.name,
        db_type: datasourceForm.db_type,
        dsn: buildDatasourceDsn(),
        config_json: buildDatasourceConfigJson()
      })
    )
    if (!result) return
    datasourceModalVisible.value = false
    await refreshTenantDatasources()
  }

  async function testDatasourceForm() {
    if (!validateDatasourceForm()) return
    const result = await ws.run(
      '测试连接',
      () => datasourceApi.testConnection({
        tenant_id: ws.context.tenantId,
        db_type: datasourceForm.db_type,
        dsn: buildDatasourceDsn(),
        config_json: buildDatasourceConfigJson()
      }),
      { silent: true }
    )
    const version =
      result && typeof result === 'object' && 'version' in result ? String((result as { version?: string }).version || '') : ''
    if (version) {
      notify.success(`测试连接成功，识别到数据库版本：${version}`)
      return
    }
    if (result) notify.success('测试连接成功')
  }

  async function testDatasource() {
    if (!ws.context.datasourceId) return notify.warning('请先选择数据源')
    const result = await ws.run('测试连接', () => datasourceApi.test(ws.context.datasourceId), { silent: true })
    const version =
      result && typeof result === 'object' && 'version' in result ? String((result as { version?: string }).version || '') : ''
    if (version) {
      notify.success(`测试连接成功，识别到数据库版本：${version}`)
      await refreshTenantDatasources({ silent: true })
      await useProjectStore().refreshProjectDatasources({ silent: true })
      return
    }
    if (result) notify.success('测试连接成功')
  }

  async function syncDatasource() {
    if (!ws.context.datasourceId) return notify.warning('请先选择数据源')
    await ws.run('同步元数据', () => datasourceApi.sync(ws.context.datasourceId, { trigger_type: 'manual', user_id: ws.context.userId }))
    await refreshTenantDatasources()
    await useProjectStore().refreshProjectDatasources({ silent: true })
    if (metadataPreviewVisible.value) {
      await loadMetadata({ silent: true })
    }
  }

  async function loadMetadata(options: { silent?: boolean } = {}) {
    if (!ws.context.datasourceId) return notify.warning('请先选择数据源')
    const previousTableId = selectedMetadataTable.value?.id
    const result = await ws.run('加载元数据', () => datasourceApi.metadataTables(ws.context.datasourceId, false, ws.pageParams('metadataTables')), options)
    if (result) {
      metadataTables.value = result as PageResult<MetadataTableRecord>
      ws.syncPage('metadataTables', metadataTables.value)
      metadataPreviewVisible.value = true
      const nextTable =
        metadataTables.value.items.find((table) => table.id === previousTableId) ||
        metadataTables.value.items[0]
      if (nextTable) {
        await selectMetadataTable(nextTable)
        return
      }
      selectedMetadataTable.value = null
      tableCommentDraft.value = ''
      columnCommentDrafts.value = {}
    }
  }

  async function selectMetadataTable(table: MetadataTableRecord) {
    if (!ws.context.datasourceId) return notify.warning('请先选择数据源')
    const result = await ws.run('加载表详情', () => datasourceApi.metadataTableDetail(ws.context.datasourceId, table.id), { silent: true })
    if (!result) return
    const detail = result as MetadataTableRecord
    selectedMetadataTable.value = detail
    tableCommentDraft.value = detail.comment_text || ''
    columnCommentDrafts.value = Object.fromEntries((detail.columns || []).map((column) => [column.id, column.comment_text || '']))
  }

  async function saveMetadataTableComment() {
    if (!ws.context.datasourceId || !selectedMetadataTable.value) return
    const result = await ws.run(
      '保存表备注',
      () =>
        datasourceApi.updateTableComment(ws.context.datasourceId, selectedMetadataTable.value!.id, {
          comment: tableCommentDraft.value,
          user_id: ws.context.userId
        }),
      { silent: true }
    )
    if (!result) return
    const updated = result as MetadataTableRecord
    selectedMetadataTable.value = {
      ...selectedMetadataTable.value,
      comment_text: updated.comment_text || ''
    }
    metadataTables.value.items = metadataTables.value.items.map((item) =>
      item.id === updated.id ? { ...item, comment_text: updated.comment_text || '' } : item
    )
    notify.success('表备注已保存')
  }

  async function saveMetadataColumnComment(column: MetadataColumnRecord) {
    if (!ws.context.datasourceId || !selectedMetadataTable.value) return
    const result = await ws.run(
      '保存字段备注',
      () =>
        datasourceApi.updateColumnComment(ws.context.datasourceId, column.id, {
          comment: columnCommentDrafts.value[column.id] || '',
          user_id: ws.context.userId
        }),
      { silent: true }
    )
    if (!result) return
    const updated = result as MetadataColumnRecord
    selectedMetadataTable.value = {
      ...selectedMetadataTable.value,
      columns: (selectedMetadataTable.value.columns || []).map((item) =>
        item.id === updated.id ? { ...item, comment_text: updated.comment_text || '' } : item
      )
    }
    columnCommentDrafts.value[updated.id] = updated.comment_text || ''
    notify.success('字段备注已保存')
  }

  function resetMetadataPreview() {
    ws.resetPage('metadataTables')
    metadataTables.value = emptyPage()
    selectedMetadataTable.value = null
    tableCommentDraft.value = ''
    columnCommentDrafts.value = {}
    metadataPreviewVisible.value = false
  }

  function handleMetadataModalVisibleChange(value: boolean) {
    if (value) {
      metadataPreviewVisible.value = true
      return
    }
    resetMetadataPreview()
  }

  function selectDatasource(datasource: DataSourceRecord) {
    ws.context.datasourceId = datasource.id
    ws.resetPage('metadataTables')
    resetMetadataPreview()
  }

  async function testSelectedDatasource(datasource: DataSourceRecord) {
    selectDatasource(datasource)
    await testDatasource()
  }

  async function syncSelectedDatasource(datasource: DataSourceRecord) {
    selectDatasource(datasource)
    await syncDatasource()
  }

  async function viewSelectedDatasourceMetadata(datasource: DataSourceRecord) {
    selectDatasource(datasource)
    await loadMetadata()
  }

  async function deleteDatasource(source: DataSourceRecord) {
    const result = await ws.run('删除数据源', () => datasourceApi.delete(source.id, ws.context.tenantId))
    if (!result) return
    if (ws.context.datasourceId === source.id) {
      ws.context.datasourceId = 0
      resetMetadataPreview()
    }
    await refreshTenantDatasources({ silent: true })
    await useProjectStore().refreshProjectDatasources({ silent: true })
    if (useUiStore().activeModule === 'knowledge' && ws.context.projectId) {
      await useKnowledgeStore().refreshKnowledge({ silent: true })
    }
  }

  function handleDatasourceTypeChange(value: string | null) {
    datasourceForm.db_type = value || 'mysql'
    datasourceForm.port = datasourceDefaultPort(datasourceForm.db_type)
  }

  return {
    datasources,
    datasourceOptionItems,
    metadataTables,
    selectedMetadataTable,
    tableCommentDraft,
    columnCommentDrafts,
    datasourceSearch,
    datasourceTypeFilter,
    datasourceModalVisible,
    metadataPreviewVisible,
    datasourceForm,
    datasourceOptions,
    selectedDatasource,
    filteredDatasources,
    refreshTenantDatasources,
    refreshDatasourceOptions,
    createDatasource,
    testDatasourceForm,
    testDatasource,
    syncDatasource,
    loadMetadata,
    selectMetadataTable,
    saveMetadataTableComment,
    saveMetadataColumnComment,
    resetMetadataPreview,
    handleMetadataModalVisibleChange,
    selectDatasource,
    testSelectedDatasource,
    syncSelectedDatasource,
    viewSelectedDatasourceMetadata,
    deleteDatasource,
    handleDatasourceTypeChange
  }
})
