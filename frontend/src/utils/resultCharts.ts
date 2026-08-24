import type { ChartSuggestion, QueryExecutionResult } from '@/types/domain'

export type ChartDisplayKind = 'line' | 'bar' | 'pie' | 'funnel' | 'radar' | 'metric' | 'table'

export const chartColors = ['#0f8f6b', '#2d6cdf', '#f59e0b', '#ef6f6c', '#8b5cf6', '#14b8a6', '#64748b', '#d946ef']

const chartKinds = new Set(['line', 'bar', 'pie', 'funnel', 'radar', 'metric', 'table'])

export function resultRows(result?: QueryExecutionResult) {
  return result?.rows || []
}

export function resultColumns(result?: QueryExecutionResult) {
  if (result?.columns?.length) return result.columns
  return Object.keys(result?.rows?.[0] || {})
}

export function toNumber(value: unknown) {
  if (typeof value === 'number') return value
  if (typeof value === 'string') {
    const parsed = Number(value.replace(/,/g, '').replace('%', '').trim())
    return Number.isFinite(parsed) ? parsed : Number.NaN
  }
  return Number.NaN
}

export function formatNumber(value: number) {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value)
}

export function formatCell(value: unknown) {
  if (value === null || value === undefined) return '-'
  if (typeof value === 'number') return formatNumber(value)
  const numeric = toNumber(value)
  if (typeof value === 'string' && value.trim() !== '' && Number.isFinite(numeric)) return formatNumber(numeric)
  return String(value)
}

export function numericFields(rows: Record<string, unknown>[], columns: string[]) {
  return columns.filter((column) => {
    const checked = rows.filter((row) => row[column] !== null && row[column] !== undefined && row[column] !== '')
    return checked.length > 0 && checked.every((row) => Number.isFinite(toNumber(row[column])))
  })
}

export function firstTimeField(rows: Record<string, unknown>[], columns: string[]) {
  return columns.find((column) => {
    const name = column.toLowerCase()
    if (
      name.includes('date') ||
      name.includes('time') ||
      name.includes('day') ||
      name.includes('month') ||
      name.includes('year') ||
      name.includes('日期') ||
      name.includes('时间') ||
      name.includes('月份') ||
      name.includes('年度')
    ) {
      return true
    }
    return rows.some((row) => typeof row[column] === 'string' && /^\d{4}[-/]\d{1,2}[-/]\d{1,2}|^\d{1,2}[-/]\d{1,2}$/.test(String(row[column])))
  })
}

export function firstLabelField(rows: Record<string, unknown>[], columns: string[], numeric: string[]) {
  return columns.find((column) => !numeric.includes(column) && rows.some((row) => row[column] !== null && row[column] !== undefined && row[column] !== ''))
}

export function rawChartKind(result?: QueryExecutionResult) {
  const raw = (result?.chart?.type || result?.execution?.chart_type || '').toLowerCase()
  return chartKinds.has(raw) ? (raw as ChartDisplayKind) : ''
}

export function chartLabelField(chart: ChartSuggestion | undefined, rows: Record<string, unknown>[], columns: string[], numeric: string[]) {
  return chart?.name_field || chart?.x_field || firstTimeField(rows, columns) || firstLabelField(rows, columns, numeric) || columns[0]
}

export function chartValueField(chart: ChartSuggestion | undefined, numeric: string[]) {
  const preferred = chart?.value_field || chart?.y_fields?.[0]
  if (preferred && numeric.includes(preferred)) return preferred
  return numeric[0]
}

export function chartData(result?: QueryExecutionResult, limit = 30) {
  const rows = resultRows(result)
  const columns = resultColumns(result)
  const numeric = numericFields(rows, columns)
  const labelField = chartLabelField(result?.chart, rows, columns, numeric)
  const valueField = chartValueField(result?.chart, numeric)
  if (!rows.length || !valueField) return []

  const values = rows
    .map((row, index) => ({
      label: formatCell(row[labelField] ?? row[columns[0]] ?? `第 ${index + 1} 项`),
      value: toNumber(row[valueField]),
      color: chartColors[index % chartColors.length],
      raw: row
    }))
    .filter((item) => Number.isFinite(item.value))
    .slice(0, limit)

  const max = Math.max(...values.map((item) => Math.abs(item.value)), 0)
  const total = values.reduce((sum, item) => sum + Math.abs(item.value), 0)
  return values.map((item) => ({
    ...item,
    width: max > 0 ? Math.max(4, Math.round((Math.abs(item.value) / max) * 100)) : 0,
    percent: total > 0 ? Math.round((Math.abs(item.value) / total) * 1000) / 10 : 0
  }))
}

export function metricItems(result?: QueryExecutionResult) {
  const rows = resultRows(result)
  const columns = resultColumns(result)
  if (!rows.length) return []

  const numeric = numericFields(rows, columns)
  const first = rows[0]
  const fields = numeric.length ? numeric : columns
  return fields.slice(0, 8).map((field, index) => {
    const numericValue = toNumber(first[field])
    return {
      label: field,
      value: formatCell(first[field]),
      numericValue: Number.isFinite(numericValue) ? numericValue : undefined,
      color: chartColors[index % chartColors.length]
    }
  })
}

export function resolveChartKind(result?: QueryExecutionResult): ChartDisplayKind {
  const rows = resultRows(result)
  const columns = resultColumns(result)
  const numeric = numericFields(rows, columns)
  if (!rows.length || !numeric.length) return 'table'

  const raw = rawChartKind(result)
  if (rows.length === 1) return 'metric'

  if (raw === 'line' && rows.length >= 2) return 'line'
  if (raw === 'bar') return 'bar'
  if (raw === 'funnel' && chartData(result).length >= 2) return 'funnel'
  if (raw === 'pie' && chartData(result).length >= 2) return 'pie'
  if (raw === 'metric') return 'bar'
  if (raw === 'radar') return 'bar'
  if (raw === 'table') return 'table'

  if (firstTimeField(rows, columns) && rows.length >= 2) return 'line'
  if (firstLabelField(rows, columns, numeric)) {
    const data = chartData(result)
    if (data.length >= 2 && data.length <= 8 && data.reduce((sum, item) => sum + Math.abs(item.value), 0) > 0) return 'pie'
    return 'bar'
  }
  return 'table'
}

export function chartKindLabel(kind: ChartDisplayKind) {
  const names: Record<ChartDisplayKind, string> = {
    line: '折线图',
    bar: '柱状图',
    pie: '饼图',
    funnel: '漏斗图',
    radar: '雷达图',
    metric: '指标卡',
    table: '表格'
  }
  return names[kind]
}

export function chartTitle(result?: QueryExecutionResult) {
  const kind = resolveChartKind(result)
  if (rawChartKind(result) === kind && result?.chart?.title) return result.chart.title
  const names: Record<ChartDisplayKind, string> = {
    line: '趋势分析',
    bar: '分类对比',
    pie: '占比分析',
    funnel: '转化漏斗',
    radar: '多指标雷达',
    metric: '关键指标',
    table: '明细结果'
  }
  return names[kind] || '查询结果'
}

export function chartDescription(result?: QueryExecutionResult) {
  const rows = resultRows(result)
  const kind = resolveChartKind(result)
  if (kind === 'metric') {
    const count = metricItems(result).length
    return count > 1 ? `返回 1 行结果，已按 ${count} 个指标卡展示。` : '单点结果已按指标卡展示。'
  }
  if (rawChartKind(result) === kind && result?.chart?.reason) return result.chart.reason
  return rows ? `返回 ${rows.length} 行数据，已自动选择${chartKindLabel(kind)}展示。` : '暂无可视化数据。'
}
