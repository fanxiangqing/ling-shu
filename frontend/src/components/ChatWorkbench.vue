<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import {
  NButton,
  NDataTable,
  NIcon,
  NInput,
  NInputNumber,
  NScrollbar,
  NTag,
  NTooltip
} from 'naive-ui'
import { Mic, SendHorizontal, Sparkles } from '@lucide/vue'
import type { AgentEvent, ChartSuggestion, ChatMessage, DataSourceOption, QueryExecutionResult } from '@/types/domain'

const props = defineProps<{
  messages: ChatMessage[]
  datasources: DataSourceOption[]
  sessionId: number
  projectName: string
  sessionTitle: string
  autoExecute: boolean
  maxRows: number
  loading: boolean
  voiceRecording?: boolean
  voiceBusy?: boolean
  voiceEnabled?: boolean
  assistantName?: string
  welcomeMessage?: string
}>()

const emit = defineEmits<{
  ask: [question: string]
  'voice-toggle': []
  'update:maxRows': [value: number]
}>()

const draft = ref('')
const isComposing = ref(false)
const compositionLockUntil = ref(0)
const scrollbarRef = ref<{ scrollTo: (options: { top: number; behavior?: ScrollBehavior }) => void } | null>(null)
const chartColors = ['#0f8f6b', '#2d6cdf', '#f59e0b', '#ef6f6c', '#8b5cf6', '#14b8a6', '#64748b', '#d946ef']
const voiceBars = [32, 54, 42, 72, 48, 86, 58, 68, 38, 76, 44, 62, 34, 52, 46, 80, 56, 70]

const activeSourceLabel = computed(() => {
  if (props.datasources.length > 1) return `项目数据源 ${props.datasources.length} 个`
  if (props.datasources.length === 1) return props.datasources[0].name
  return '未绑定数据源'
})

const activeSourceHint = computed(() => {
  if (props.datasources.length > 1) return props.datasources.map((source) => source.name).join('、')
  return activeSourceLabel.value
})

const assistantLabel = computed(() => props.assistantName || 'Ling-Shu')

const textSendLoading = computed(() => props.loading && !props.voiceRecording && !props.voiceBusy)

function submit(value = draft.value) {
  const question = value.trim()
  if (!question || props.loading) return
  draft.value = ''
  emit('ask', question)
}

function toggleVoice() {
  if (props.loading && !props.voiceRecording && !props.voiceBusy) return
  emit('voice-toggle')
}

function handleEnter(event: KeyboardEvent) {
  if (event.isComposing || isComposing.value || Date.now() < compositionLockUntil.value) return
  event.preventDefault()
  submit()
}

function handleCompositionEnd() {
  isComposing.value = false
  compositionLockUntil.value = Date.now() + 120
}

async function scrollToBottom(behavior: ScrollBehavior = 'smooth') {
  await nextTick()
  scrollbarRef.value?.scrollTo({ top: Number.MAX_SAFE_INTEGER, behavior })
}

watch(
  () => props.sessionId,
  () => scrollToBottom('auto'),
  { flush: 'post' }
)

watch(
  () => props.messages.map((message) => [
    message.id,
    message.content.length,
    message.pending ? 'pending' : 'done',
    message.result?.agent?.steps?.length || 0,
    message.result?.execution?.rows?.length || 0,
    message.result?.executions?.map((item) => item.rows?.length || 0).join(',')
  ].join(':')).join('|'),
  () => scrollToBottom(),
  { flush: 'post' }
)

function executionResult(message: ChatMessage) {
  return message.result?.execution
}

function executionResults(message: ChatMessage) {
  const multi = message.result?.executions?.filter(Boolean) || []
  const single = executionResult(message)
  if (multi.length) {
    if (single && !sameExecution(single, multi[0])) return [single, ...multi]
    return multi
  }
  return single ? [single] : []
}

function sameExecution(left?: QueryExecutionResult, right?: QueryExecutionResult) {
  const leftId = left?.execution?.id
  const rightId = right?.execution?.id
  return Boolean(leftId && rightId && leftId === rightId)
}

function executionTitle(message: ChatMessage, result: QueryExecutionResult, index: number) {
  if (isCombinedExecution(message, result)) return '跨数据源分布图'
  const task = executionTask(message, result, index)
  return task?.purpose || chartTitle(result)
}

function executionTag(message: ChatMessage, result: QueryExecutionResult, index: number) {
  if (isCombinedExecution(message, result)) return chartKind(result)
  const task = executionTask(message, result, index)
  return task?.datasource_name || (task?.datasource_id ? `数据源 #${task.datasource_id}` : chartKind(result))
}

function executionTask(message: ChatMessage, result: QueryExecutionResult, index: number) {
  const offset = isCombinedExecution(message, result) ? -1 : combinedExecutionOffset(message, index)
  return offset >= 0 ? message.result?.agent?.sql_tasks?.[offset] : undefined
}

function isCombinedExecution(message: ChatMessage, result: QueryExecutionResult) {
  return Boolean(hasCombinedExecution(message) && executionResult(message) === result)
}

function hasCombinedExecution(message: ChatMessage) {
  const single = executionResult(message)
  const firstDetail = message.result?.executions?.[0]
  return Boolean(message.result?.executions?.length && single && !sameExecution(single, firstDetail))
}

function combinedExecutionOffset(message: ChatMessage, index: number) {
  return hasCombinedExecution(message) ? index - 1 : index
}

function resultRows(result?: QueryExecutionResult) {
  return result?.rows || []
}

function resultColumns(result?: QueryExecutionResult) {
  if (result?.columns?.length) return result.columns
  return Object.keys(result?.rows?.[0] || {})
}

function chartKind(result?: QueryExecutionResult) {
  const raw = (result?.chart?.type || result?.execution?.chart_type || '').toLowerCase()
  if (['line', 'bar', 'pie', 'funnel', 'radar', 'table'].includes(raw)) return raw
  return inferChartKind(result)
}

function inferChartKind(result?: QueryExecutionResult) {
  const rows = resultRows(result)
  const columns = resultColumns(result)
  const numeric = numericFields(rows, columns)
  if (!rows.length || !numeric.length) return 'table'
  if (firstTimeField(rows, columns)) return 'line'
  if (firstLabelField(rows, columns, numeric) && rows.length <= 8) return 'pie'
  if (numeric.length >= 3 && rows.length === 1) return 'radar'
  return 'bar'
}

function chartTitle(result?: QueryExecutionResult) {
  const kind = chartKind(result)
  if (result?.chart?.title) return result.chart.title
  const names: Record<string, string> = {
    line: '趋势分析',
    bar: '分类对比',
    pie: '占比分析',
    funnel: '转化漏斗',
    radar: '多指标雷达',
    table: '明细结果'
  }
  return names[kind] || '查询结果'
}

function chartDescription(result?: QueryExecutionResult) {
  if (result?.chart?.reason) return result.chart.reason
  const rows = resultRows(result).length
  return rows ? `返回 ${rows} 行数据，已自动选择适合的展示方式。` : '暂无可视化数据。'
}

function chartData(result?: QueryExecutionResult) {
  const rows = resultRows(result)
  const columns = resultColumns(result)
  const numeric = numericFields(rows, columns)
  const labelField = chartLabelField(result?.chart, rows, columns, numeric)
  const valueField = chartValueField(result?.chart, numeric)
  if (!rows.length || !valueField) return []
  const values = rows
    .map((row, index) => ({
      label: formatCell(row[labelField] ?? row[columns[0]] ?? `第 ${index + 1} 项`),
      value: toNumber(row[valueField]) || 0,
      color: chartColors[index % chartColors.length]
    }))
    .filter((item) => Number.isFinite(item.value))
    .slice(0, 12)
  const max = Math.max(...values.map((item) => Math.abs(item.value)), 0)
  const total = values.reduce((sum, item) => sum + Math.abs(item.value), 0)
  return values.map((item) => ({
    ...item,
    width: max > 0 ? Math.max(4, Math.round((Math.abs(item.value) / max) * 100)) : 0,
    percent: total > 0 ? Math.round((Math.abs(item.value) / total) * 1000) / 10 : 0
  }))
}

function chartLabelField(chart: ChartSuggestion | undefined, rows: Record<string, unknown>[], columns: string[], numeric: string[]) {
  return chart?.name_field || chart?.x_field || firstTimeField(rows, columns) || firstLabelField(rows, columns, numeric) || columns[0]
}

function chartValueField(chart: ChartSuggestion | undefined, numeric: string[]) {
  return chart?.value_field || chart?.y_fields?.[0] || numeric[0]
}

function numericFields(rows: Record<string, unknown>[], columns: string[]) {
  return columns.filter((column) => {
    const checked = rows.filter((row) => row[column] !== null && row[column] !== undefined && row[column] !== '')
    return checked.length > 0 && checked.every((row) => Number.isFinite(toNumber(row[column])))
  })
}

function firstTimeField(rows: Record<string, unknown>[], columns: string[]) {
  return columns.find((column) => {
    const name = column.toLowerCase()
    if (name.includes('date') || name.includes('time') || name.includes('day') || name.includes('month') || name.includes('日期') || name.includes('时间')) {
      return true
    }
    return rows.some((row) => typeof row[column] === 'string' && /^\d{4}[-/]\d{1,2}[-/]\d{1,2}|^\d{1,2}[-/]\d{1,2}$/.test(String(row[column])))
  })
}

function firstLabelField(rows: Record<string, unknown>[], columns: string[], numeric: string[]) {
  return columns.find((column) => !numeric.includes(column) && rows.some((row) => row[column] !== null && row[column] !== undefined && row[column] !== ''))
}

function toNumber(value: unknown) {
  if (typeof value === 'number') return value
  if (typeof value === 'string') {
    const parsed = Number(value.replace(/,/g, '').replace('%', '').trim())
    return Number.isFinite(parsed) ? parsed : Number.NaN
  }
  return Number.NaN
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value)
}

function formatCell(value: unknown) {
  if (value === null || value === undefined) return '-'
  if (typeof value === 'number') return formatNumber(value)
  return String(value)
}

function pieGradient(result?: QueryExecutionResult) {
  const data = chartData(result)
  if (!data.length) return '#edf3ef'
  let start = 0
  const parts = data.map((item) => {
    const end = start + item.percent
    const segment = `${item.color} ${start}% ${end}%`
    start = end
    return segment
  })
  return `conic-gradient(${parts.join(', ')})`
}

function linePoints(result?: QueryExecutionResult) {
  const data = chartData(result).slice(0, 16)
  const values = data.map((item) => item.value)
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  const width = 640
  const height = 220
  const left = 32
  const right = 24
  const top = 18
  const bottom = 34
  const plotWidth = width - left - right
  const plotHeight = height - top - bottom
  return data.map((item, index) => ({
    ...item,
    x: left + (data.length === 1 ? plotWidth / 2 : (index / (data.length - 1)) * plotWidth),
    y: top + ((max - item.value) / span) * plotHeight
  }))
}

function linePath(result?: QueryExecutionResult) {
  return linePoints(result).map((point) => `${point.x},${point.y}`).join(' ')
}

function radarPoints(result?: QueryExecutionResult) {
  const rows = resultRows(result)
  const columns = resultColumns(result)
  const numeric = numericFields(rows, columns)
  const source = rows.length === 1 && numeric.length >= 3
    ? numeric.slice(0, 8).map((field, index) => ({
        label: field,
        value: toNumber(rows[0][field]) || 0,
        color: chartColors[index % chartColors.length]
      }))
    : chartData(result).slice(0, 8)
  const max = Math.max(...source.map((item) => Math.abs(item.value)), 1)
  const center = 120
  const radius = 84
  return source.map((item, index) => {
    const angle = (Math.PI * 2 * index) / source.length - Math.PI / 2
    const scale = Math.abs(item.value) / max
    const axisX = center + Math.cos(angle) * radius
    const axisY = center + Math.sin(angle) * radius
    return {
      ...item,
      axisX,
      axisY,
      x: center + Math.cos(angle) * radius * scale,
      y: center + Math.sin(angle) * radius * scale
    }
  })
}

function radarPolygon(result?: QueryExecutionResult) {
  return radarPoints(result).map((point) => `${point.x},${point.y}`).join(' ')
}

function tableColumns(result?: QueryExecutionResult) {
  return resultColumns(result).slice(0, 8).map((column) => ({
    title: column,
    key: column,
    ellipsis: { tooltip: true },
    render: (row: Record<string, unknown>) => formatCell(row[column])
  }))
}

function tableRows(result?: QueryExecutionResult) {
  return resultRows(result).slice(0, 8).map((row, index) => ({ ...row, rowKey: index }))
}

function messageSteps(message: ChatMessage) {
  const steps = message.result?.agent?.steps || []
  if (!steps.length) {
    return message.pending
      ? [{ type: 'thought', step: 1, name: '准备执行', content: '正在理解你的问题，并准备项目上下文。' }]
      : []
  }
  const out: AgentEvent[] = []
  const deltaSteps = new Set<number>()
  for (const item of steps) {
    if (item.type === 'final') continue
    if (item.type === 'llm_delta') {
      if (deltaSteps.has(item.step)) continue
      deltaSteps.add(item.step)
      out.push({ ...item, content: item.name === 'llm.plan' ? '模型正在判断任务类型。' : '模型正在生成查询计划。' })
      continue
    }
    out.push(item)
  }
  return out
}

function activeStep(message: ChatMessage) {
  const steps = messageSteps(message)
  return steps[steps.length - 1]
}

function stepStatusText(message: ChatMessage) {
  if (message.pending) return '运行中'
  const steps = messageSteps(message)
  if (steps.some((step) => step.type === 'error')) return '失败'
  return '已完成'
}

function stepStatusClass(message: ChatMessage) {
  if (message.pending) return 'running'
  if (messageSteps(message).some((step) => step.type === 'error')) return 'warning'
  return 'done'
}

function stepContent(step?: AgentEvent) {
  if (!step) return '准备执行'
  return compactText(step.content || step.sql || stepName(step), 110)
}

function stepDetailContent(step: AgentEvent) {
  return compactText(step.content || '', 180)
}

function compactText(value: string, limit: number) {
  const text = value.replace(/\s+/g, ' ').trim()
  if (text.length <= limit) return text
  return `${text.slice(0, limit)}...`
}

function stepName(step?: AgentEvent) {
  if (!step) return '准备执行'
  const names: Record<string, string> = {
    thought: '思考',
    action: '动作',
    observation: '观察',
    llm_delta: '模型生成',
    error: '错误'
  }
  return step.name || names[step.type] || step.type
}

function stepTypeText(step: AgentEvent) {
  const names: Record<string, string> = {
    thought: '思考',
    action: '动作',
    observation: '观察',
    llm_delta: '生成',
    error: '异常'
  }
  return names[step.type] || step.type
}

function hasSQL(message: ChatMessage) {
  return Boolean(message.result?.agent?.sql || message.result?.agent?.sql_tasks?.length)
}
</script>

<template>
  <main class="chat-workbench">
    <header class="workbench-head">
      <div>
        <div class="eyebrow">{{ projectName }}</div>
        <h1>{{ sessionTitle || '自然语言问数' }}</h1>
      </div>
      <div class="head-tools">
        <NTooltip trigger="hover">
          <template #trigger>
            <NTag round type="success">{{ activeSourceLabel }}</NTag>
          </template>
          {{ activeSourceHint }}
        </NTooltip>
        <NTag round type="success">自动执行已开启</NTag>
        <div class="limit-control" aria-label="结果行数上限">
          <span>结果上限</span>
          <NInputNumber
            class="rows-input"
            :value="maxRows"
            :min="20"
            :max="1000"
            :step="20"
            size="small"
            @update:value="emit('update:maxRows', Number($event || 200))"
          />
        </div>
      </div>
    </header>

    <NScrollbar ref="scrollbarRef" class="message-scroll">
      <div v-if="!messages.length" class="chat-welcome">
        <div class="bot-badge compact">
          <NIcon :component="Sparkles" />
        </div>
        <h2>你好，我是 {{ assistantLabel }}</h2>
        <p>{{ welcomeMessage || `当前项目是 ${projectName}。你可以直接提问业务指标、趋势、排名或明细。` }}</p>
      </div>
      <div v-else class="message-stack">
        <article
          v-for="message in messages"
          :key="message.id"
          class="message"
          :class="[message.role, { pending: message.pending }]"
        >
          <div v-if="message.role === 'assistant'" class="message-label">
            <NIcon :component="Sparkles" />
            {{ assistantLabel }}
            <NTag v-if="message.pending" size="small" round>运行中</NTag>
          </div>
          <p>{{ message.content }}</p>

          <section v-if="message.role === 'assistant' && messageSteps(message).length" class="agent-step-card">
            <header class="step-head">
              <div class="step-current">
                <span class="step-pulse" :class="stepStatusClass(message)" />
                <div>
                  <strong>{{ stepName(activeStep(message)) }}</strong>
                  <p>{{ stepContent(activeStep(message)) }}</p>
                </div>
              </div>
              <NTag size="small" round>{{ stepStatusText(message) }}</NTag>
            </header>
            <div class="step-rail" :aria-label="`共 ${messageSteps(message).length} 个运行步骤`">
              <span
                v-for="(step, index) in messageSteps(message)"
                :key="`${step.step}-${step.type}-${step.name}-${index}`"
                class="step-dot"
                :class="[step.type, { active: index === messageSteps(message).length - 1 }]"
                :title="`${step.step}. ${stepName(step)}`"
              />
            </div>
            <details v-if="messageSteps(message).length > 1" class="step-details">
              <summary>查看 {{ messageSteps(message).length }} 个步骤明细</summary>
              <ol>
                <li v-for="(step, index) in messageSteps(message)" :key="`${step.step}-${step.type}-${step.name}-detail-${index}`" :class="step.type">
                  <span class="step-index">{{ step.step }}</span>
                  <div>
                    <strong>{{ stepName(step) }}</strong>
                    <NTag size="small" round>{{ stepTypeText(step) }}</NTag>
                    <p v-if="step.content">{{ stepDetailContent(step) }}</p>
                    <code v-if="step.sql">{{ step.sql }}</code>
                  </div>
                </li>
              </ol>
            </details>
          </section>

          <template v-if="message.role === 'assistant' && executionResults(message).length">
            <section
              v-for="(result, resultIndex) in executionResults(message)"
              :key="`${message.id}-result-${resultIndex}`"
              v-show="resultRows(result).length || result.error"
              class="message-result-card"
            >
              <header class="visual-head">
                <div>
                  <strong>{{ executionTitle(message, result, resultIndex) }}</strong>
                  <span>{{ chartDescription(result) }}</span>
                </div>
                <NTag size="small" round>{{ executionTag(message, result, resultIndex) }}</NTag>
              </header>

              <div
                v-if="resultRows(result).length && chartKind(result) === 'pie'"
                class="chart-view pie-view"
              >
                <div class="pie-donut" :style="{ background: pieGradient(result) }">
                  <span>{{ resultRows(result).length }} 项</span>
                </div>
                <div class="chart-legend">
                  <div v-for="item in chartData(result)" :key="item.label" class="legend-row">
                    <i :style="{ background: item.color }" />
                    <span>{{ item.label }}</span>
                    <strong>{{ item.percent }}%</strong>
                  </div>
                </div>
              </div>

              <div
                v-else-if="resultRows(result).length && chartKind(result) === 'line'"
                class="chart-view line-view"
              >
                <svg viewBox="0 0 640 220" role="img" aria-label="趋势图">
                  <line x1="32" y1="186" x2="616" y2="186" />
                  <polyline :points="linePath(result)" />
                  <circle
                    v-for="point in linePoints(result)"
                    :key="`${point.label}-${point.x}`"
                    :cx="point.x"
                    :cy="point.y"
                    r="4"
                  />
                </svg>
                <div class="axis-labels">
                  <span v-for="point in linePoints(result).slice(0, 6)" :key="point.label">
                    {{ point.label }}
                  </span>
                </div>
              </div>

              <div
                v-else-if="resultRows(result).length && chartKind(result) === 'radar'"
                class="chart-view radar-view"
              >
                <svg viewBox="0 0 240 240" role="img" aria-label="雷达图">
                  <circle cx="120" cy="120" r="84" />
                  <circle cx="120" cy="120" r="56" />
                  <circle cx="120" cy="120" r="28" />
                  <line
                    v-for="point in radarPoints(result)"
                    :key="`${point.label}-axis`"
                    x1="120"
                    y1="120"
                    :x2="point.axisX"
                    :y2="point.axisY"
                  />
                  <polygon :points="radarPolygon(result)" />
                  <circle
                    v-for="point in radarPoints(result)"
                    :key="`${point.label}-point`"
                    :cx="point.x"
                    :cy="point.y"
                    r="3.5"
                  />
                </svg>
                <div class="chart-legend compact">
                  <div v-for="item in radarPoints(result)" :key="item.label" class="legend-row">
                    <span>{{ item.label }}</span>
                    <strong>{{ formatNumber(item.value) }}</strong>
                  </div>
                </div>
              </div>

              <div v-else-if="resultRows(result).length && chartKind(result) !== 'table'" class="chart-view bar-view">
                <div v-for="item in chartData(result)" :key="item.label" class="bar-row">
                  <span>{{ item.label }}</span>
                  <div class="bar-track">
                    <i :style="{ width: `${item.width}%`, background: item.color }" />
                  </div>
                  <strong>{{ formatNumber(item.value) }}</strong>
                </div>
              </div>

              <NDataTable
                v-if="tableRows(result).length"
                class="message-table"
                size="small"
                :bordered="false"
                :single-line="false"
                :columns="tableColumns(result)"
                :data="tableRows(result)"
                :row-key="(row) => row.rowKey"
              />

              <section v-if="result.error" class="message-error">
                {{ result.error }}
              </section>
            </section>

            <details v-if="hasSQL(message)" class="message-sql">
              <summary>查看 SQL</summary>
              <code v-if="message.result?.agent?.sql">{{ message.result.agent.sql }}</code>
              <code
                v-for="(task, taskIndex) in message.result?.agent?.sql_tasks || []"
                :key="`${message.id}-sql-${taskIndex}`"
              >{{ task.sql }}</code>
            </details>
          </template>
        </article>
      </div>
    </NScrollbar>

    <div class="composer">
      <div class="composer-row">
        <div v-if="voiceRecording || voiceBusy" class="voice-composer" :class="{ processing: voiceBusy && !voiceRecording }">
          <div class="voice-wave" aria-hidden="true">
            <i
              v-for="(height, index) in voiceBars"
              :key="index"
              :style="{ '--voice-height': `${height}%`, '--voice-delay': `${index * 70}ms` }"
            />
          </div>
        </div>
        <NInput
          v-else
          v-model:value="draft"
          class="ask-input"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 5 }"
          placeholder="输入你的业务问题，例如：今天销售额是多少？"
          @compositionstart="isComposing = true"
          @compositionend="handleCompositionEnd"
          @keydown.enter.exact="handleEnter"
        />
        <div class="composer-actions">
          <NTooltip v-if="voiceEnabled !== false" trigger="hover">
            <template #trigger>
              <NButton
                circle
                :type="voiceRecording || voiceBusy ? 'primary' : 'default'"
                :secondary="voiceRecording || voiceBusy"
                :disabled="loading && !voiceRecording && !voiceBusy"
                @click="toggleVoice"
              >
                <template #icon>
                  <NIcon :component="Mic" />
                </template>
              </NButton>
            </template>
            {{ voiceRecording ? '结束本轮并发送' : voiceBusy ? '停止连续语音' : '语音输入' }}
          </NTooltip>
          <NButton type="primary" :loading="textSendLoading" :disabled="voiceRecording || voiceBusy" @click="submit()">
            <template #icon>
              <NIcon :component="SendHorizontal" />
            </template>
            发送
          </NButton>
        </div>
      </div>
    </div>
  </main>
</template>
