<script setup lang="ts">
import { AlertCircle, RotateCcw } from 'lucide-vue-next'

withDefaults(defineProps<{ message?: string; retryable?: boolean }>(), {
  message: '加载失败，请稍后重试。',
  retryable: true,
})

defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="state-card state-card--error" role="alert">
    <AlertCircle :size="22" aria-hidden="true" />
    <span>{{ message }}</span>
    <button v-if="retryable" class="button button--secondary button--small" type="button" @click="$emit('retry')">
      <RotateCcw :size="15" /> 重试
    </button>
  </div>
</template>
