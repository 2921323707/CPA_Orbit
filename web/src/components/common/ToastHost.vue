<script setup lang="ts">
import { CheckCircle2, Info, X, XCircle } from 'lucide-vue-next'
import { useToast } from '../../composables/useToast'

const { toasts, remove } = useToast()
</script>

<template>
  <div class="toast-host" aria-live="polite" aria-atomic="false">
    <TransitionGroup name="toast">
      <div v-for="toast in toasts" :key="toast.id" class="toast" :class="`toast--${toast.type}`" role="status">
        <CheckCircle2 v-if="toast.type === 'success'" :size="19" />
        <XCircle v-else-if="toast.type === 'error'" :size="19" />
        <Info v-else :size="19" />
        <span>{{ toast.message }}</span>
        <button type="button" aria-label="关闭通知" @click="remove(toast.id)"><X :size="16" /></button>
      </div>
    </TransitionGroup>
  </div>
</template>
