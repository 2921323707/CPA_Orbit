<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { onBeforeUnmount, watch } from 'vue'

const props = withDefaults(defineProps<{ open: boolean; title: string; width?: string }>(), { width: '540px' })
const emit = defineEmits<{ close: [] }>()

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && props.open) emit('close')
}

watch(() => props.open, (open) => {
  document.body.style.overflow = open ? 'hidden' : ''
}, { immediate: true })
window.addEventListener('keydown', onKeydown)
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <Transition name="drawer">
      <div v-if="open" class="drawer-layer" role="presentation" @mousedown.self="$emit('close')">
        <section class="drawer" :style="{ maxWidth: width }" role="dialog" aria-modal="true" :aria-label="title">
          <header class="drawer__header">
            <h2>{{ title }}</h2>
            <button class="icon-button" type="button" aria-label="关闭详情" @click="$emit('close')">
              <X :size="20" />
            </button>
          </header>
          <div class="drawer__body"><slot /></div>
          <footer v-if="$slots.footer" class="drawer__footer"><slot name="footer" /></footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>
