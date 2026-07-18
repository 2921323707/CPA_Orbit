<script setup lang="ts">
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { useLocale } from '../../i18n'

defineProps<{ page: number; totalPages: number; total: number; pageSize?: number }>()
const emit = defineEmits<{ change: [page: number] }>()
const { isEnglish, t } = useLocale()
</script>

<template>
  <nav v-if="total > 0" class="pagination pagination--table" :aria-label="t('common.pagination')">
    <span class="pagination__summary">{{ isEnglish ? `${t('common.total')} ${total} ${t('common.items')} · ${t('common.perPage')} ${pageSize ?? 10}` : `${t('common.total')} ${total} ${t('common.items')} · ${t('common.perPage')} ${pageSize ?? 10} ${t('common.items')}` }}</span>
    <div class="pagination__controls">
      <button class="button button--secondary button--small" type="button" :disabled="page <= 1" @click="emit('change', page - 1)"><ChevronLeft :size="15" />{{ t('common.prev') }}</button>
      <span>{{ t('common.page') }} {{ page }} / {{ totalPages || 1 }}<template v-if="!isEnglish"> 页</template></span>
      <button class="button button--secondary button--small" type="button" :disabled="page >= totalPages" @click="emit('change', page + 1)">{{ t('common.next') }}<ChevronRight :size="15" /></button>
    </div>
  </nav>
</template>
