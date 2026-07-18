<script setup lang="ts">
import { computed } from 'vue'
import type { QuotaWindow } from '../../types/api'
import { formatDateTime } from '../../utils/format'

const props = defineProps<{ window?: QuotaWindow | null; label: string }>()
const remaining = computed(() => {
  const value = Number(props.window?.remainingPercent)
  return Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : null
})
const formatted = computed(() => remaining.value == null ? '—' : `${remaining.value.toFixed(remaining.value % 1 ? 1 : 0)}%`)
const tone = computed(() => remaining.value == null ? 'unknown' : remaining.value <= 0 ? 'empty' : remaining.value <= 20 ? 'low' : 'normal')
</script>

<template>
  <span v-if="remaining == null" class="muted">—</span>
  <div v-else class="quota-cell" :class="`quota-cell--${tone}`">
    <div class="quota-cell__value"><strong>{{ formatted }}</strong><span>剩余</span></div>
    <div
      class="quota-meter"
      role="progressbar"
      :aria-label="`${label}剩余额度 ${formatted}`"
      aria-valuemin="0"
      aria-valuemax="100"
      :aria-valuenow="remaining"
    ><span :style="{ width: `${remaining}%` }" /></div>
    <small v-if="window?.resetAt" :title="formatDateTime(window.resetAt)">重置 {{ formatDateTime(window.resetAt) }}</small>
  </div>
</template>
