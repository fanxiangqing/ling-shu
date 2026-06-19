import type {
  AgentEvent,
  ChatMessageRecord,
  SendChatMessageResult
} from '@/types/domain'

export function pendingChatResult(question: string, steps: AgentEvent[]): SendChatMessageResult {
  return {
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
  if (event.type === 'llm_delta') return event.name === 'llm.plan' ? '正在判断任务类型。' : '正在生成查询计划，并持续校验。'
  if (event.type === 'error') return event.content || '执行过程遇到错误，正在尝试修复。'
  if (event.name === 'sql.execute') return 'SQL 已通过审核，正在查询数据。'
  if (event.content) return event.content
  return fallback
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
