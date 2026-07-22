<script setup lang="ts">
import { computed } from 'vue'
import type { UsageBucket } from '../../types/api'

const props = defineProps<{ buckets: UsageBucket[] }>()
const width = 900
const height = 220
const padding = 28

const points = computed(() => {
  const grouped = new Map<string, number>()
  for (const bucket of props.buckets) {
    const key = bucket.bucketAt
    grouped.set(key, (grouped.get(key) || 0) + bucket.inputTokens + bucket.outputTokens + bucket.cacheCreationTokens + bucket.cacheReadTokens)
  }
  const rows = [...grouped.entries()].sort(([a], [b]) => a.localeCompare(b))
  const max = Math.max(1, ...rows.map(([, value]) => value))
  return rows.map(([at, value], index) => ({
    at,
    value,
    x: rows.length <= 1 ? padding : padding + index * ((width - padding * 2) / (rows.length - 1)),
    y: height - padding - (value / max) * (height - padding * 2),
  }))
})

const line = computed(() => points.value.map((point, index) => `${index ? 'L' : 'M'} ${point.x} ${point.y}`).join(' '))
const area = computed(() => points.value.length ? `${line.value} L ${points.value[points.value.length - 1].x} ${height - padding} L ${points.value[0].x} ${height - padding} Z` : '')
const total = computed(() => points.value.reduce((sum, point) => sum + point.value, 0))
</script>

<template>
  <div class="token-chart">
    <div v-if="!points.length" class="token-chart__empty">等待 Sub2API 产生完整的 15 分钟用量窗口</div>
    <template v-else>
      <svg :viewBox="`0 0 ${width} ${height}`" role="img" aria-label="Token 用量趋势">
        <defs><linearGradient id="token-area" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#2dd4bf" stop-opacity=".32" /><stop offset="1" stop-color="#2dd4bf" stop-opacity="0" /></linearGradient></defs>
        <line v-for="index in 4" :key="index" :x1="padding" :x2="width-padding" :y1="padding + (index-1)*(height-padding*2)/3" :y2="padding + (index-1)*(height-padding*2)/3" class="token-chart__grid" />
        <path :d="area" fill="url(#token-area)" />
        <path :d="line" class="token-chart__line" />
        <circle v-for="point in points" :key="point.at" :cx="point.x" :cy="point.y" r="3.5"><title>{{ new Date(point.at).toLocaleString() }} · {{ point.value.toLocaleString() }} tokens</title></circle>
      </svg>
      <div class="token-chart__legend"><span><i />15 分钟总 Token</span><strong>{{ total.toLocaleString() }}</strong></div>
    </template>
  </div>
</template>
