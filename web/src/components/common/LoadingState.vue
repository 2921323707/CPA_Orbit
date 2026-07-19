<script setup lang="ts">
withDefaults(defineProps<{ label?: string }>(), { label: '正在加载…' })
</script>

<template>
  <div class="loading-state" role="status" aria-live="polite" aria-busy="true">
    <div class="loading-state__heading"><span class="spinner" aria-hidden="true" /><strong>{{ label }}</strong></div>
    <div class="loading-state__skeleton" aria-hidden="true">
      <i /><i /><i />
    </div>
  </div>
</template>
