<script setup lang="ts">
import type { Component } from 'vue'

withDefaults(defineProps<{
  label: string
  value: string | number
  hint?: string
  icon: Component
  tone?: 'default' | 'success' | 'warning' | 'danger'
}>(), { hint: '', tone: 'default' })
</script>

<template>
  <article class="kpi-card" :class="`kpi-card--${tone}`">
    <div class="kpi-card__icon" aria-hidden="true"><component :is="icon" :size="20" /></div>
    <div>
      <p>{{ label }}</p>
      <strong>{{ value }}</strong>
      <small v-if="hint">{{ hint }}</small>
    </div>
  </article>
</template>
