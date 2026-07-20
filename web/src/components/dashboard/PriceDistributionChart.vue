<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { LineChart, Table2, Trash2 } from 'lucide-vue-next'
import PaginationBar from '../common/PaginationBar.vue'
import type { PriceSample } from '../../types/api'
import { formatCurrency, getErrorMessage } from '../../utils/format'
import { api } from '../../services/api'
import { useToast } from '../../composables/useToast'

const props = defineProps<{ k12History?: PriceSample[]; gptPlusHistory?: PriceSample[] }>()
const emit = defineEmits<{ historyDeleted: [] }>()
type Mode = 'k12' | 'gpt-plus'
type RangeMinutes = 120 | 1440 | 10080 | 20160
interface RealPoint { at: Date; average: number }
interface Series { id: 'k12' | 'gpt-plus'; label: string; points: RealPoint[] }

const mode = ref<Mode>('k12')
const rangeMinutes = ref<RangeMinutes>(120)
const showTable = ref(false)
const hovered = ref('')
const tablePage = ref(1)
const tablePageSize = 10
const deleting = ref('')
const toast = useToast()
const modes = [{ value: 'k12' as const, label: 'K12' }, { value: 'gpt-plus' as const, label: 'GPT Plus' }]
const ranges = [{ value: 120 as const, label: '2H' }, { value: 1440 as const, label: '24H' }, { value: 10080 as const, label: '7D' }, { value: 20160 as const, label: '14D' }]
const width = 760
const height = 220
const padding = { top: 18, right: 24, bottom: 38, left: 62 }
const plotWidth = width - padding.left - padding.right
const plotHeight = height - padding.top - padding.bottom

function parseDate(value: unknown): Date | null {
  if (!value) return null
  const text = String(value).trim()
  const date = new Date(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}/.test(text) ? text.replace(' ', 'T') : text)
  return Number.isNaN(date.getTime()) ? null : date
}

function normalize(history: PriceSample[] | undefined): RealPoint[] {
  return (history ?? []).map((sample) => ({ at: parseDate(sample.at), average: Number(sample.average) }))
    .filter((sample): sample is RealPoint => Boolean(sample.at) && sample.average > 0)
    .sort((a, b) => a.at.getTime() - b.at.getTime())
}

const endTime = computed(() => Date.now())
const startTime = computed(() => endTime.value - rangeMinutes.value * 60_000)
const allSeries = computed<Series[]>(() => [
  { id: 'k12', label: 'K12', points: normalize(props.k12History) },
  { id: 'gpt-plus', label: 'GPT Plus', points: normalize(props.gptPlusHistory) },
])
const visibleSeries = computed(() => allSeries.value.filter((series) => series.id === mode.value).map((series) => ({
  ...series,
  points: series.points.filter((point) => point.at.getTime() >= startTime.value && point.at.getTime() <= endTime.value),
})))
const allVisiblePoints = computed(() => visibleSeries.value.flatMap((series) => series.points))
const values = computed(() => allVisiblePoints.value.map((point) => point.average))
const minValue = computed(() => values.value.length ? Math.min(...values.value) : 0)
const maxValue = computed(() => values.value.length ? Math.max(...values.value) : 1)
const valuePadding = computed(() => Math.max((maxValue.value - minValue.value) * 0.16, maxValue.value * 0.08, 0.02))
const baseline = computed(() => Math.max(0, minValue.value - valuePadding.value))
const chartMax = computed(() => maxValue.value + valuePadding.value)
const chartRange = computed(() => Math.max(chartMax.value - baseline.value, 0.04))
const x = (date: Date) => padding.left + ((date.getTime() - startTime.value) / (endTime.value - startTime.value)) * plotWidth
const y = (price: number) => padding.top + plotHeight - ((price - baseline.value) / chartRange.value) * plotHeight
const pathFor = (points: RealPoint[]) => points.map((point, index) => `${index ? 'L' : 'M'} ${x(point.at).toFixed(2)} ${y(point.average).toFixed(2)}`).join(' ')
const yTicks = computed(() => Array.from({ length: 5 }, (_, index) => {
  const ratio = index / 4
  return { value: baseline.value + chartRange.value * ratio, y: padding.top + plotHeight * (1 - ratio) }
}).reverse())
const xTicks = computed(() => Array.from({ length: 5 }, (_, index) => {
  const ratio = index / 4
  return { at: new Date(startTime.value + (endTime.value - startTime.value) * ratio), x: padding.left + plotWidth * ratio }
}))
const currentRangeLabel = computed(() => ranges.find((item) => item.value === rangeMinutes.value)?.label ?? '')
const tableRows = computed(() => visibleSeries.value.flatMap((series) => series.points.map((point) => ({ ...point, source: series.label, sourceId: series.id }))).sort((a, b) => b.at.getTime() - a.at.getTime()))
const tableTotalPages = computed(() => Math.max(1, Math.ceil(tableRows.value.length / tablePageSize)))
const pagedTableRows = computed(() => tableRows.value.slice((tablePage.value - 1) * tablePageSize, tablePage.value * tablePageSize))
const formatAxisTime = (date: Date) => rangeMinutes.value <= 1440 ? date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) : date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
const formatExactTime = (date: Date) => date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
const seriesAverage = (series: Series) => series.points.length ? series.points.reduce((sum, point) => sum + point.average, 0) / series.points.length : null

async function deleteSample(row: { source: string; sourceId: 'k12' | 'gpt-plus'; at: Date; average: number }) {
  if (!window.confirm(`确认删除 ${row.source} ${formatExactTime(row.at)} 的报价记录 ${formatCurrency(row.average)}？`)) return
  const key = `${row.sourceId}-${row.at.getTime()}`
  deleting.value = key
  try {
    await api.deletePriceHistory(row.sourceId, row.at.toISOString())
    toast.success('异常报价记录已删除，图表已重新计算')
    emit('historyDeleted')
  } catch (error) {
    toast.error(getErrorMessage(error))
  } finally {
    deleting.value = ''
  }
}

watch([mode, rangeMinutes], () => { tablePage.value = 1; hovered.value = '' })
</script>

<template>
  <section class="panel chart-panel" aria-labelledby="price-chart-title">
    <div class="panel__header panel__header--wrap">
      <div><div class="chart-title-row"><LineChart :size="18" /><h2 id="price-chart-title">平均报价走势</h2></div><p>真实查询均价，可切换查看 K12 或 GPT Plus</p></div>
      <div class="chart-controls chart-controls--stacked">
        <div class="segmented-control source-control" aria-label="报价来源">
          <button v-for="item in modes" :key="item.value" type="button" :class="{ 'is-active': mode === item.value }" @click="mode = item.value">{{ item.label }}</button>
        </div>
        <div class="segmented-control period-control" aria-label="查看周期">
          <button v-for="range in ranges" :key="range.value" type="button" :class="{ 'is-active': rangeMinutes === range.value }" @click="rangeMinutes = range.value">{{ range.label }}</button>
        </div>
        <button class="button button--ghost button--small" type="button" :disabled="!allVisiblePoints.length" :aria-expanded="showTable" @click="showTable = !showTable"><Table2 :size="16" />{{ showTable ? '收起数据详情' : '展开数据详情' }}</button>
      </div>
    </div>

    <div v-if="allVisiblePoints.length" class="chart-summary chart-summary--series">
      <span v-for="series in visibleSeries" :key="series.id" class="series-stat" :class="`series-stat--${series.id}`"><i />{{ series.label }} <strong>{{ series.points.length }}</strong> 次 · 均价 <strong>{{ formatCurrency(seriesAverage(series)) }}</strong><template v-if="series.points.length"> · 最新 <strong>{{ formatCurrency(series.points[series.points.length - 1].average) }}</strong></template></span>
    </div>

    <div v-if="allVisiblePoints.length" class="chart-wrap real-price-wrap">
      <svg class="price-chart" :viewBox="`0 0 ${width} ${height}`" role="img" aria-labelledby="price-chart-title price-chart-desc">
        <desc id="price-chart-desc">所选周期内当前账号类型的真实查询平均报价折线，不包含模拟值。</desc>
        <g v-for="tick in yTicks" :key="tick.y"><line :x1="padding.left" :x2="width - padding.right" :y1="tick.y" :y2="tick.y" class="chart-grid" /><text :x="padding.left - 10" :y="tick.y + 4" text-anchor="end" class="chart-label">¥{{ tick.value.toFixed(2) }}</text></g>
        <g v-for="tick in xTicks" :key="tick.at.getTime()"><line :x1="tick.x" :x2="tick.x" :y1="padding.top" :y2="padding.top + plotHeight" class="chart-grid chart-grid--vertical" /><text :x="tick.x" :y="height - 16" text-anchor="middle" class="chart-label">{{ formatAxisTime(tick.at) }}</text></g>
        <g v-for="series in visibleSeries" :key="series.id">
          <path v-if="series.points.length > 1" :d="pathFor(series.points)" class="chart-line" :class="`chart-line--${series.id}`" />
          <g v-for="(point, index) in series.points" :key="`${series.id}-${point.at.getTime()}`" class="price-point" @mouseenter="hovered = `${series.id}-${index}`" @mouseleave="hovered = ''" @focus="hovered = `${series.id}-${index}`" @blur="hovered = ''">
            <circle :cx="x(point.at)" :cy="y(point.average)" r="12" class="chart-hit" tabindex="0" :aria-label="`${series.label}，${formatExactTime(point.at)}，平均报价 ${formatCurrency(point.average)}`" />
            <circle :cx="x(point.at)" :cy="y(point.average)" :r="hovered === `${series.id}-${index}` ? 5 : 3.5" class="chart-dot" :class="`chart-dot--${series.id}`" />
            <g v-if="hovered === `${series.id}-${index}`" class="chart-tooltip" aria-hidden="true"><rect :x="Math.min(Math.max(x(point.at) - 80, 4), width - 164)" :y="Math.max(y(point.average) - 62, 4)" width="160" height="46" rx="8" /><text :x="Math.min(Math.max(x(point.at), 84), width - 84)" :y="Math.max(y(point.average) - 43, 22)" text-anchor="middle">{{ series.label }} · {{ formatExactTime(point.at) }}</text><text :x="Math.min(Math.max(x(point.at), 84), width - 84)" :y="Math.max(y(point.average) - 25, 40)" text-anchor="middle" class="chart-tooltip__value">均价 {{ formatCurrency(point.average) }}</text></g>
          </g>
        </g>
      </svg>
      <p class="chart-axis-caption">横轴：真实查询时间 · 纵轴：平均报价 · 当前周期：{{ currentRangeLabel }}</p>
    </div>
    <div v-else class="chart-empty"><LineChart :size="24" /><strong>这个周期还没有真实查询记录</strong><span>两个报价源都会在成功抓取后写入 14 天历史。</span></div>

    <div v-if="showTable && tableRows.length" class="chart-table">
      <div class="table-wrap"><table><caption class="sr-only">真实平均报价记录</caption><thead><tr><th>来源</th><th>真实查询时间</th><th class="numeric">平均报价</th><th>操作</th></tr></thead><tbody><tr v-for="row in pagedTableRows" :key="`${row.sourceId}-${row.at.getTime()}`"><td><span class="series-label" :class="`series-label--${row.sourceId}`"><i />{{ row.source }}</span></td><td>{{ formatExactTime(row.at) }}</td><td class="numeric strong">{{ formatCurrency(row.average) }}</td><td><button class="button button--danger button--small" type="button" :disabled="deleting === `${row.sourceId}-${row.at.getTime()}`" @click="deleteSample(row)"><Trash2 :size="14" />{{ deleting === `${row.sourceId}-${row.at.getTime()}` ? '删除中' : '删除' }}</button></td></tr></tbody></table></div>
      <PaginationBar :page="tablePage" :total-pages="tableTotalPages" :total="tableRows.length" :page-size="tablePageSize" @change="tablePage = $event" />
    </div>
  </section>
</template>
