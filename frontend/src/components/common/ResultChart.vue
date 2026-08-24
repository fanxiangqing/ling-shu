<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { BarChart, FunnelChart, LineChart, PieChart } from 'echarts/charts'
import { DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { graphic, init, use } from 'echarts/core'
import type { EChartsOption } from 'echarts'
import { CanvasRenderer } from 'echarts/renderers'
import type { QueryExecutionResult } from '@/types/domain'
import {
  chartColors,
  chartData,
  chartLabelField,
  chartValueField,
  formatNumber,
  metricItems,
  numericFields,
  resolveChartKind,
  resultColumns,
  resultRows,
  toNumber
} from '@/utils/resultCharts'

use([BarChart, LineChart, PieChart, FunnelChart, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent, CanvasRenderer])

const props = defineProps<{
  result: QueryExecutionResult
}>()

const chartRef = ref<HTMLDivElement | null>(null)
let chart: ReturnType<typeof init> | null = null
let resizeObserver: ResizeObserver | null = null

const rows = computed(() => resultRows(props.result))
const columns = computed(() => resultColumns(props.result))
const numeric = computed(() => numericFields(rows.value, columns.value))
const displayKind = computed(() => resolveChartKind(props.result))
const metrics = computed(() => metricItems(props.result))
const chartHeight = computed(() => {
  if (displayKind.value === 'bar') return `${Math.min(460, Math.max(280, chartData(props.result).length * 34 + 96))}px`
  if (displayKind.value === 'pie' || displayKind.value === 'funnel') return '320px'
  return '300px'
})

const chartOption = computed<EChartsOption | null>(() => {
  if (displayKind.value === 'line') return lineOption()
  if (displayKind.value === 'bar') return barOption()
  if (displayKind.value === 'pie') return pieOption()
  if (displayKind.value === 'funnel') return funnelOption()
  return null
})

function baseTextStyle() {
  return {
    color: '#42524c',
    fontFamily: 'inherit'
  }
}

function lineOption(): EChartsOption {
  const labelField = chartLabelField(props.result.chart, rows.value, columns.value, numeric.value)
  const valueField = chartValueField(props.result.chart, numeric.value)
  const labels = rows.value.map((row, index) => String(row[labelField] ?? `第 ${index + 1} 项`))
  const values = rows.value.map((row) => toNumber(row[valueField]))

  return {
    color: chartColors,
    backgroundColor: 'transparent',
    textStyle: baseTextStyle(),
    grid: { top: 24, right: 28, bottom: 46, left: 54, containLabel: true },
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value) => formatNumber(Number(value))
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: labels,
      axisLabel: { color: '#66756f', hideOverlap: true, margin: 12 },
      axisLine: { lineStyle: { color: '#dbe5e0' } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      splitNumber: 4,
      axisLabel: { color: '#66756f', formatter: (value: number) => compactAxisNumber(value) },
      splitLine: { lineStyle: { color: '#edf3ef' } }
    },
    series: [
      {
        name: valueField,
        type: 'line',
        data: values,
        smooth: true,
        symbol: 'circle',
        symbolSize: 7,
        connectNulls: true,
        lineStyle: { width: 3 },
        areaStyle: {
          opacity: 0.16
        },
        emphasis: { focus: 'series' }
      }
    ]
  }
}

function barOption(): EChartsOption {
  const data = chartData(props.result, 40)
  const horizontal = data.length > 6 || data.some((item) => item.label.length > 8)
  const labels = data.map((item) => item.label)
  const values = data.map((item) => item.value)
  const axisStyle = {
    axisLine: { lineStyle: { color: '#dbe5e0' } },
    axisTick: { show: false }
  }

  return {
    color: chartColors,
    backgroundColor: 'transparent',
    textStyle: baseTextStyle(),
    grid: horizontal
      ? { top: 18, right: 36, bottom: data.length > 14 ? 56 : 24, left: 96, containLabel: true }
      : { top: 18, right: 28, bottom: data.length > 10 ? 58 : 44, left: 54, containLabel: true },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      valueFormatter: (value) => formatNumber(Number(value))
    },
    dataZoom: data.length > 14 ? [{ type: 'slider', height: 18, bottom: 14, brushSelect: false }] : undefined,
    xAxis: horizontal
      ? {
          type: 'value',
          axisLabel: { color: '#66756f', formatter: (value: number) => compactAxisNumber(value) },
          splitLine: { lineStyle: { color: '#edf3ef' } },
          ...axisStyle
        }
      : {
          type: 'category',
          data: labels,
          axisLabel: { color: '#66756f', hideOverlap: true, interval: 0, rotate: data.length > 8 ? 24 : 0 },
          ...axisStyle
        },
    yAxis: horizontal
      ? {
          type: 'category',
          data: labels,
          inverse: true,
          axisLabel: { color: '#66756f', width: 120, overflow: 'truncate' },
          ...axisStyle
        }
      : {
          type: 'value',
          axisLabel: { color: '#66756f', formatter: (value: number) => compactAxisNumber(value) },
          splitLine: { lineStyle: { color: '#edf3ef' } },
          ...axisStyle
        },
    series: [
      {
        type: 'bar',
        data: values,
        barMaxWidth: 30,
        itemStyle: {
          borderRadius: horizontal ? [0, 8, 8, 0] : [8, 8, 0, 0],
          color: new graphic.LinearGradient(horizontal ? 0 : 0, horizontal ? 0 : 1, horizontal ? 1 : 0, horizontal ? 0 : 0, [
            { offset: 0, color: '#22b98c' },
            { offset: 1, color: '#0f7d5f' }
          ])
        }
      }
    ]
  }
}

function pieOption(): EChartsOption {
  const data = chartData(props.result, 12).map((item) => ({ name: item.label, value: item.value }))
  return {
    color: chartColors,
    backgroundColor: 'transparent',
    textStyle: baseTextStyle(),
    tooltip: {
      trigger: 'item',
      valueFormatter: (value) => formatNumber(Number(value))
    },
    legend: {
      type: 'scroll',
      orient: 'vertical',
      right: 10,
      top: 32,
      bottom: 20,
      itemWidth: 9,
      itemHeight: 9,
      textStyle: { color: '#42524c', fontSize: 12 }
    },
    series: [
      {
        type: 'pie',
        radius: ['48%', '72%'],
        center: ['34%', '50%'],
        avoidLabelOverlap: true,
        minAngle: 6,
        label: {
          color: '#25312d',
          formatter: '{b}\n{d}%'
        },
        labelLine: { length: 12, length2: 8 },
        itemStyle: {
          borderColor: '#fbfdfc',
          borderWidth: 3
        },
        data
      }
    ]
  }
}

function funnelOption(): EChartsOption {
  const data = chartData(props.result, 12)
    .map((item) => ({ name: item.label, value: item.value }))
    .sort((left, right) => right.value - left.value)
  return {
    color: chartColors,
    backgroundColor: 'transparent',
    textStyle: baseTextStyle(),
    tooltip: {
      trigger: 'item',
      valueFormatter: (value) => formatNumber(Number(value))
    },
    series: [
      {
        type: 'funnel',
        left: '8%',
        top: 22,
        bottom: 20,
        width: '84%',
        minSize: '24%',
        maxSize: '100%',
        sort: 'descending',
        gap: 3,
        label: {
          color: '#25312d',
          formatter: ({ name, value }) => `${name}  ${formatNumber(Number(value))}`
        },
        itemStyle: {
          borderColor: '#fbfdfc',
          borderWidth: 2
        },
        data
      }
    ]
  }
}

function compactAxisNumber(value: number) {
  const abs = Math.abs(value)
  if (abs >= 100000000) return `${formatNumber(value / 100000000)}亿`
  if (abs >= 10000) return `${formatNumber(value / 10000)}万`
  return formatNumber(value)
}

async function renderChart() {
  await nextTick()
  if (!chartRef.value || !chartOption.value) {
    chart?.dispose()
    chart = null
    return
  }
  if (!chart) chart = init(chartRef.value)
  chart.setOption(chartOption.value, true)
  chart.resize()
}

function observeChartElement() {
  resizeObserver?.disconnect()
  if (chartRef.value) resizeObserver?.observe(chartRef.value)
}

onMounted(() => {
  resizeObserver = new ResizeObserver(() => chart?.resize())
  observeChartElement()
  void renderChart()
})

watch(chartRef, () => {
  observeChartElement()
  void renderChart()
})

watch(chartOption, () => void renderChart(), { deep: true, flush: 'post' })

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <div class="result-chart">
    <div v-if="displayKind === 'metric'" class="metric-summary-grid">
      <div v-for="item in metrics" :key="item.label" class="metric-tile" :style="{ '--metric-accent': item.color }">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
      </div>
    </div>
    <div v-else-if="!chartOption" class="chart-empty">
      暂无适合图表展示的数据
    </div>
    <div v-else ref="chartRef" class="result-chart-canvas" :style="{ height: chartHeight }" />
  </div>
</template>
