import { computed, reactive, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { knowledgeApi, ragApi } from '@/api/resources'
import type {
  KBFewShotRecord,
  KBMetricRecord,
  KBTermRecord,
  PageResult,
  RAGFewShotItem,
  RAGKnowledgeItem,
  RAGRebuildResult,
  RAGSearchResult
} from '@/types/domain'
import { emptyPage } from '@/utils/format'
import { notify } from '@/composables/useNotify'
import { useWorkspaceStore } from '@/stores/workspace'
import { useUiStore } from '@/stores/ui'

export const knowledgeStatusOptions = [
  { label: '全部知识', value: 'all' },
  { label: '只看启用', value: 'enabled' },
  { label: '只看停用', value: 'disabled' }
]

type RAGResultItem = {
  title: string
  body: string
  meta?: string
  code?: string
}

type RAGResultSection = {
  key: 'terms' | 'metrics' | 'fewShots'
  label: string
  items: RAGResultItem[]
}

export const useKnowledgeStore = defineStore('knowledge', () => {
  const ws = useWorkspaceStore()

  const terms = ref<PageResult<KBTermRecord>>(emptyPage())
  const metrics = ref<PageResult<KBMetricRecord>>(emptyPage())
  const fewShots = ref<PageResult<KBFewShotRecord>>(emptyPage())

  const knowledgeStatusFilter = ref<'all' | 'enabled' | 'disabled'>('all')
  const termModalVisible = ref(false)
  const metricModalVisible = ref(false)
  const fewShotModalVisible = ref(false)

  const termForm = reactive({ term: 'GMV', definition: '成交金额，通常等于已支付订单金额总和。', aliases: '销售额,成交额' })
  const metricForm = reactive({ name: '销售额', description: '已支付订单金额', formula: 'sum(pay_amount)', default_time_column: 'created_at' })
  const fewShotForm = reactive({
    question: '今天销售额是多少？',
    sql: 'select sum(pay_amount) as sales_amount from orders where date(created_at) = current_date',
    explanation: '按当天订单创建时间过滤并汇总支付金额。'
  })
  const ragForm = reactive({ question: 'GMV 怎么计算？', limit: 8 })
  const ragSearchResult = ref<RAGSearchResult | null>(null)
  const ragRebuildResult = ref<RAGRebuildResult | null>(null)
  const ragLastQuestion = ref('')

  function filterKnowledgeItems<T extends { enabled: boolean }>(items: T[]) {
    if (knowledgeStatusFilter.value === 'enabled') return items.filter((item) => item.enabled)
    if (knowledgeStatusFilter.value === 'disabled') return items.filter((item) => !item.enabled)
    return items
  }

  const visibleTerms = computed(() => filterKnowledgeItems(terms.value.items))
  const visibleMetrics = computed(() => filterKnowledgeItems(metrics.value.items))
  const visibleFewShots = computed(() => filterKnowledgeItems(fewShots.value.items))
  const knowledgeTotal = computed(() => terms.value.total + metrics.value.total + fewShots.value.total)
  const enabledKnowledgeTotal = computed(
    () =>
      terms.value.items.filter((item) => item.enabled).length +
      metrics.value.items.filter((item) => item.enabled).length +
      fewShots.value.items.filter((item) => item.enabled).length
  )
  const ragVectorHits = computed(() => ragSearchResult.value?.hits ?? [])
  const ragHitCount = computed(() => ragVectorHits.value.length)
  const ragContextCount = computed(() => {
    const result = ragSearchResult.value
    if (!result) return 0
    return result.business_terms.length + result.metrics.length + result.few_shots.length
  })
  const ragResultSections = computed<RAGResultSection[]>(() => {
    const result = ragSearchResult.value
    if (!result) return []
    const sections: RAGResultSection[] = [
      {
        key: 'terms',
        label: '业务术语',
        items: result.business_terms.map((item) => ({
          title: item.name || '未命名术语',
          body: item.description || '暂无描述'
        }))
      },
      {
        key: 'metrics',
        label: '指标口径',
        items: result.metrics.map((item) => ({
          title: item.name || '未命名指标',
          body: item.description || item.expression || '暂无描述',
          meta: item.expression ? `口径：${item.expression}` : undefined
        }))
      },
      {
        key: 'fewShots',
        label: '示例问法',
        items: result.few_shots.map((item) => ({
          title: item.question || '未命名示例',
          body: item.datasource_id ? `数据源 #${item.datasource_id}` : '项目通用示例',
          code: item.sql
        }))
      }
    ]
    return sections.filter((section) => section.items.length > 0)
  })

  function knowledgeEnabledParam() {
    if (knowledgeStatusFilter.value === 'enabled') return true
    if (knowledgeStatusFilter.value === 'disabled') return false
    return null
  }

  function clearItems() {
    terms.value = emptyPage()
    metrics.value = emptyPage()
    fewShots.value = emptyPage()
    clearRAGFeedback()
  }

  function clearRAGFeedback() {
    ragSearchResult.value = null
    ragRebuildResult.value = null
    ragLastQuestion.value = ''
  }

  function normalizeRAGSearchResult(result: RAGSearchResult): RAGSearchResult {
    return {
      business_terms: normalizeRAGKnowledgeItems(result.business_terms),
      metrics: normalizeRAGKnowledgeItems(result.metrics),
      few_shots: normalizeRAGFewShotItems(result.few_shots),
      hits: Array.isArray(result.hits) ? result.hits : []
    }
  }

  function normalizeRAGKnowledgeItems(items?: RAGKnowledgeItem[]) {
    return Array.isArray(items) ? items : []
  }

  function normalizeRAGFewShotItems(items?: RAGFewShotItem[]) {
    return Array.isArray(items) ? items : []
  }

  function ragContextTotal(result: RAGSearchResult) {
    return result.business_terms.length + result.metrics.length + result.few_shots.length
  }

  async function refreshKnowledge(options: { silent?: boolean } = {}) {
    if (!ws.ensureProject()) return
    const enabled = knowledgeEnabledParam()
    const [termResult, metricResult, fewShotResult] = await Promise.all([
      knowledgeApi.listTerms(ws.context.projectId, ws.context.tenantId, enabled, ws.pageParams('terms')).catch((error) => error),
      knowledgeApi.listMetrics(ws.context.projectId, ws.context.tenantId, ws.context.datasourceId || undefined, enabled, ws.pageParams('metrics')).catch((error) => error),
      knowledgeApi.listFewShots(ws.context.projectId, ws.context.tenantId, ws.context.datasourceId || undefined, enabled, ws.pageParams('fewShots')).catch((error) => error)
    ])
    if (!(termResult instanceof Error)) {
      terms.value = termResult
      ws.syncPage('terms', terms.value)
    }
    if (!(metricResult instanceof Error)) {
      metrics.value = metricResult
      ws.syncPage('metrics', metrics.value)
    }
    if (!(fewShotResult instanceof Error)) {
      fewShots.value = fewShotResult
      ws.syncPage('fewShots', fewShots.value)
    }
    if (!options.silent) notify.success('业务知识已刷新')
  }

  async function createTerm() {
    if (!ws.ensureProject()) return
    const result = await ws.run('创建术语', () =>
      knowledgeApi.createTerm(ws.context.projectId, {
        tenant_id: ws.context.tenantId,
        term: termForm.term,
        definition: termForm.definition,
        aliases: termForm.aliases.split(',').map((item) => item.trim()).filter(Boolean),
        enabled: true
      })
    )
    if (!result) return
    termModalVisible.value = false
    await refreshKnowledge({ silent: true })
  }

  async function createMetric() {
    if (!ws.ensureProject()) return
    const result = await ws.run('创建指标', () =>
      knowledgeApi.createMetric(ws.context.projectId, {
        tenant_id: ws.context.tenantId,
        datasource_id: ws.context.datasourceId || undefined,
        name: metricForm.name,
        description: metricForm.description,
        formula: metricForm.formula,
        default_time_column: metricForm.default_time_column,
        enabled: true
      })
    )
    if (!result) return
    metricModalVisible.value = false
    await refreshKnowledge({ silent: true })
  }

  async function createFewShot() {
    if (!ws.ensureProject()) return
    const result = await ws.run('创建示例问法', () =>
      knowledgeApi.createFewShot(ws.context.projectId, {
        tenant_id: ws.context.tenantId,
        datasource_id: ws.context.datasourceId || undefined,
        question: fewShotForm.question,
        sql: fewShotForm.sql,
        explanation: fewShotForm.explanation,
        enabled: true
      })
    )
    if (!result) return
    fewShotModalVisible.value = false
    await refreshKnowledge({ silent: true })
  }

  async function toggleTerm(term: KBTermRecord) {
    if (!ws.ensureProject()) return
    await ws.run(term.enabled ? '停用术语' : '启用术语', () =>
      knowledgeApi.updateTermEnabled(ws.context.projectId, term.id, {
        tenant_id: ws.context.tenantId,
        enabled: !term.enabled
      })
    )
    await refreshKnowledge({ silent: true })
  }

  async function deleteTerm(term: KBTermRecord) {
    if (!ws.ensureProject()) return
    await ws.run('删除术语', () => knowledgeApi.deleteTerm(ws.context.projectId, term.id, ws.context.tenantId))
    await refreshKnowledge({ silent: true })
  }

  async function toggleMetric(metric: KBMetricRecord) {
    if (!ws.ensureProject()) return
    await ws.run(metric.enabled ? '停用指标' : '启用指标', () =>
      knowledgeApi.updateMetricEnabled(ws.context.projectId, metric.id, {
        tenant_id: ws.context.tenantId,
        enabled: !metric.enabled
      })
    )
    await refreshKnowledge({ silent: true })
  }

  async function deleteMetric(metric: KBMetricRecord) {
    if (!ws.ensureProject()) return
    await ws.run('删除指标', () => knowledgeApi.deleteMetric(ws.context.projectId, metric.id, ws.context.tenantId))
    await refreshKnowledge({ silent: true })
  }

  async function toggleFewShot(fewShot: KBFewShotRecord) {
    if (!ws.ensureProject()) return
    await ws.run(fewShot.enabled ? '停用示例' : '启用示例', () =>
      knowledgeApi.updateFewShotEnabled(ws.context.projectId, fewShot.id, {
        tenant_id: ws.context.tenantId,
        enabled: !fewShot.enabled
      })
    )
    await refreshKnowledge({ silent: true })
  }

  async function deleteFewShot(fewShot: KBFewShotRecord) {
    if (!ws.ensureProject()) return
    await ws.run('删除示例', () => knowledgeApi.deleteFewShot(ws.context.projectId, fewShot.id, ws.context.tenantId))
    await refreshKnowledge({ silent: true })
  }

  async function rebuildRAG() {
    if (!ws.ensureProject()) return
    const result = await ws.run(
      '重建业务知识索引',
      () =>
        ragApi.rebuild(ws.context.projectId, {
          tenant_id: ws.context.tenantId,
          datasource_id: ws.context.datasourceId || undefined,
          limit: ragForm.limit
        }),
      { successMessage: false }
    )
    if (!result) return
    const rebuildResult = result as RAGRebuildResult
    ragRebuildResult.value = rebuildResult
    ragSearchResult.value = null
    ragLastQuestion.value = ''
    notify.info(`已写入 ${rebuildResult.vector_count || 0} 条向量，覆盖 ${rebuildResult.chunk_count || 0} 个知识切片`)
  }

  async function searchRAG() {
    if (!ws.ensureProject()) return
    const result = await ws.run(
      '检索业务知识',
      () =>
        ragApi.search(ws.context.projectId, {
          tenant_id: ws.context.tenantId,
          datasource_id: ws.context.datasourceId || undefined,
          question: ragForm.question,
          limit: ragForm.limit
        }),
      { successMessage: false }
    )
    if (!result) return
    const searchResult = normalizeRAGSearchResult(result as RAGSearchResult)
    ragSearchResult.value = searchResult
    ragLastQuestion.value = ragForm.question.trim()
    notify.info(`进入上下文 ${ragContextTotal(searchResult)} 条，向量命中 ${searchResult.hits?.length || 0} 条`)
  }

  watch(knowledgeStatusFilter, () => {
    ws.resetPage('terms')
    ws.resetPage('metrics')
    ws.resetPage('fewShots')
    if (useUiStore().activeModule === 'knowledge' && ws.context.projectId) {
      void refreshKnowledge({ silent: true })
    }
  })

  watch(
    () => [ws.context.projectId, ws.context.datasourceId],
    () => clearRAGFeedback()
  )

  return {
    terms,
    metrics,
    fewShots,
    knowledgeStatusFilter,
    termModalVisible,
    metricModalVisible,
    fewShotModalVisible,
    termForm,
    metricForm,
    fewShotForm,
    ragForm,
    ragSearchResult,
    ragRebuildResult,
    ragLastQuestion,
    ragVectorHits,
    ragHitCount,
    ragContextCount,
    ragResultSections,
    visibleTerms,
    visibleMetrics,
    visibleFewShots,
    knowledgeTotal,
    enabledKnowledgeTotal,
    clear: clearItems,
    clearItems,
    refreshKnowledge,
    createTerm,
    createMetric,
    createFewShot,
    toggleTerm,
    deleteTerm,
    toggleMetric,
    deleteMetric,
    toggleFewShot,
    deleteFewShot,
    rebuildRAG,
    searchRAG
  }
})
