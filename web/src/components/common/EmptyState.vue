<script setup lang="ts">
import { Inbox } from 'lucide-vue-next'

withDefaults(defineProps<{ title?: string; description?: string }>(), {
  title: '暂无数据',
  description: '这里还没有可显示的内容。',
})
</script>

<template>
  <div class="state-card state-card--stacked">
    <Inbox :size="28" aria-hidden="true" />
    <strong>{{ title }}</strong>
    <span>{{ description }}</span>
    <slot />
  </div>
</template>
