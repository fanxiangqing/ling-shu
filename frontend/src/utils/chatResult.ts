import type {
  AgentEvent,
  ChatMessage,
  ChatMessageRecord,
  SendChatMessageResult
} from '@/types/domain'

export const ANSWER_DELTA_EVENT = 'answer_delta'
export const EXECUTION_RESULT_EVENT = 'execution_result'

export function pendingChatResult(question: string, steps: AgentEvent[], current?: SendChatMessageResult | null): SendChatMessageResult {
  const next: SendChatMessageResult = {
    agent: {
      question,
      sql: '',
      explanation: '正在执行',
      review: {
        passed: false,
        risk_level: 'pending',
        normalized_sql: ''
      },
      steps
    }
  }
  if (!current) return next
  return {
    ...next,
    execution: current.execution,
    executions: current.executions,
    loops: current.loops,
    max_loops: current.max_loops
  }
}

export function failedChatResult(question: string, error: string, steps: AgentEvent[]): SendChatMessageResult {
  const nextStep = Math.max(0, ...steps.map((step) => Number(step.step) || 0)) + 1
  return {
    agent: {
      question,
      sql: '',
      explanation: error,
      review: {
        passed: false,
        risk_level: 'failed',
        normalized_sql: '',
        blocked_reason: error
      },
      steps: [
        ...steps,
        {
          type: 'error',
          step: nextStep,
          name: 'stream.error',
          content: error,
          occurred_at: new Date().toISOString()
        }
      ]
    }
  }
}

export function streamMessageContent(event: AgentEvent, fallback: string) {
  if (isExecutionResultEvent(event)) return fallback
  if (isAnswerDeltaEvent(event)) return fallback
  if (event.type === 'final') return fallback
  if (event.type === 'llm_delta') return event.name === 'llm.plan' ? '正在判断任务类型。' : '正在生成查询计划，并持续校验。'
  if (event.type === 'error') return event.content || '执行过程遇到错误，正在尝试修复。'
  if (event.name === 'sql.execute') return 'SQL 已通过审核，正在查询数据。'
  if (event.content) return event.content
  return fallback
}

export function isAnswerDeltaEvent(event: AgentEvent) {
  return event.type === ANSWER_DELTA_EVENT
}

export function isExecutionResultEvent(event: AgentEvent) {
  return event.type === EXECUTION_RESULT_EVENT
}

export function applyExecutionResult(message: ChatMessage, event: AgentEvent, question: string) {
  if (!isExecutionResultEvent(event)) return false
  const current = message.result || pendingChatResult(question, [])
  message.result = {
    ...current,
    execution: event.execution || current.execution,
    executions: event.executions || current.executions
  }
  if (!message.answerStreaming) message.content = ''
  return true
}

export function applyAnswerDelta(message: ChatMessage, event: AgentEvent) {
  if (!isAnswerDeltaEvent(event)) return false
  if (!message.answerStreaming) {
    message.content = ''
    message.answerStreaming = true
  }
  message.content += event.content || ''
  return true
}

export function assistantResultText(result?: SendChatMessageResult | null) {
  if (!result) return '已完成。'
  const multiAnswer = result.executions?.map((item) => item.answer).filter(Boolean).join('；')
  const agentAnswer = cleanResultText(result.agent?.answer || result.agent?.explanation || '')
  const executionAnswer = cleanResultText(result.execution?.answer || '')
  if (result.agent?.intent === 'query' && executionAnswer) return executionAnswer
  return agentAnswer || executionAnswer || multiAnswer || result.agent?.sql || '已完成。'
}

function cleanResultText(value: string) {
  const text = value.trim()
  if (!text || (text.includes('{') && text.includes('}'))) return ''
  return text
}

export function parseAgentResultMessage(item: ChatMessageRecord) {
  if (item.content_type !== 'agent_result') return null
  try {
    return JSON.parse(item.content) as SendChatMessageResult
  } catch {
    return null
  }
}

export function readableMessageContent(item: ChatMessageRecord, parsed = parseAgentResultMessage(item)) {
  if (!parsed) return item.content
  return assistantResultText(parsed)
}
