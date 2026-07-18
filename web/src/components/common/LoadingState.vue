<script setup lang="ts">
import ArtLoader from './ArtLoader.vue'

withDefaults(defineProps<{ label?: string }>(), { label: '正在加载…' })
</script>

<template>
  <ArtLoader compact :label="label" />
</template>
