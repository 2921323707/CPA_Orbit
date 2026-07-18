<script setup lang="ts">
import { computed } from 'vue'
import { useLocale } from '../../i18n'

const props = withDefaults(defineProps<{ label?: string; compact?: boolean }>(), {
  label: '',
  compact: false,
})
const { t } = useLocale()
const displayLabel = computed(() => props.label || t('common.loader'))
</script>

<template>
  <div class="art-loader" :class="{ 'art-loader--compact': props.compact }" role="status" aria-live="polite">
    <div class="art-loader__instrument" aria-hidden="true">
      <span class="art-loader__orbit art-loader__orbit--outer" />
      <span class="art-loader__orbit art-loader__orbit--middle" />
      <span class="art-loader__orbit art-loader__orbit--inner" />
      <span class="art-loader__axis art-loader__axis--horizontal" />
      <span class="art-loader__axis art-loader__axis--vertical" />
      <span class="art-loader__scan" />
      <i class="art-loader__signal" />
      <b class="art-loader__core" />
    </div>
    <div class="art-loader__copy">
      <span class="art-loader__eyebrow">CPA / DATA OBSERVATORY</span>
      <strong>{{ displayLabel }}</strong>
      <span class="art-loader__caption">{{ t('common.loaderCaption') }}</span>
    </div>
  </div>
</template>
