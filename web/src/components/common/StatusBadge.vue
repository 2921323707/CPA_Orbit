<script setup lang="ts">
import { AlertTriangle, CheckCircle2, CircleHelp, XCircle } from 'lucide-vue-next'

withDefaults(defineProps<{
  tone?: 'success' | 'warning' | 'danger' | 'neutral'
  label: string
}>(), { tone: 'neutral' })
</script>

<template>
  <span class="status-badge" :class="`status-badge--${tone}`">
    <CheckCircle2 v-if="tone === 'success'" :size="14" aria-hidden="true" />
    <AlertTriangle v-else-if="tone === 'warning'" :size="14" aria-hidden="true" />
    <XCircle v-else-if="tone === 'danger'" :size="14" aria-hidden="true" />
    <CircleHelp v-else :size="14" aria-hidden="true" />
    {{ label }}
  </span>
</template>
