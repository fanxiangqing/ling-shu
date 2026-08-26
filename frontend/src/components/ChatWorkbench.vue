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
import ResultChart from '@/components/common/ResultChart.vue'
import type { AgentEvent, ChatMessage, DataSourceOption, QueryExecutionResult } from '@/types/domain'
import { renderMarkdown } from '@/utils/markdown'
import { chartDescription, chartKindLabel, chartTitle, formatCell, resolveChartKind, resultColumns, resultRows } from '@/utils/resultCharts'

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
    message.answerStreaming ? 'answer' : 'idle',
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

function hasExecutionResults(message: ChatMessage) {
  return executionResults(message).length > 0
}

function showAnswerBelow(message: ChatMessage) {
  return message.role === 'assistant' && Boolean(message.content.trim()) && (message.answerStreaming || hasExecutionResults(message) || hasCompletedSteps(message))
}

function showPrimaryMessageContent(message: ChatMessage) {
  return !showAnswerBelow(message)
}

function hasCompletedSteps(message: ChatMessage) {
  return !message.pending && messageSteps(message).length > 0
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
  if (isCombinedExecution(message, result)) return chartKindLabel(chartKind(result))
  const task = executionTask(message, result, index)
  return task?.datasource_name || (task?.datasource_id ? `数据源 #${task.datasource_id}` : chartKindLabel(chartKind(result)))
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

function chartKind(result?: QueryExecutionResult) {
  return resolveChartKind(result)
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

function renderMessageContent(content: string) {
  return renderMarkdown(content)
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
    if (item.type === 'execution_result') continue
    if (item.type === 'answer_delta') continue
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
  if (message.answerStreaming) return '输出中'
  if (message.pending) return '运行中'
  const steps = messageSteps(message)
  if (steps.some((step) => step.type === 'error')) return '失败'
  return '已完成'
}

function stepStatusClass(message: ChatMessage) {
  if (message.pending || message.answerStreaming) return 'running'
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
            <NTag v-if="message.pending" size="small" round>{{ message.answerStreaming ? '输出中' : '运行中' }}</NTag>
          </div>
          <div
            v-if="showPrimaryMessageContent(message)"
            class="message-markdown"
            :class="{ streaming: message.answerStreaming }"
            v-html="renderMessageContent(message.content)"
          />

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

          <template v-if="message.role === 'assistant' && hasExecutionResults(message)">
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

              <ResultChart v-if="resultRows(result).length && chartKind(result) !== 'table'" :result="result" />

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
          </template>

          <section v-if="showAnswerBelow(message)" class="message-answer">
            <div class="message-markdown" :class="{ streaming: message.answerStreaming }" v-html="renderMessageContent(message.content)" />
          </section>

          <template v-if="message.role === 'assistant' && hasExecutionResults(message)">
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
