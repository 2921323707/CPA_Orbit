<script setup lang="ts">
import { Activity, CloudCog, RefreshCw, ShieldCheck } from 'lucide-vue-next'
import type { GatewayTargetStatus } from '../../types/api'

defineProps<{ status: GatewayTargetStatus; testing?: boolean }>()
defineEmits<{ test: [id: number]; edit: [id: number] }>()
</script>

<template>
  <article class="gateway-card" :class="[`gateway-card--${status.health.status}`, { 'gateway-card--primary': status.target.primary }]">
    <div class="gateway-card__signal"><span /><span /><span /></div>
    <div class="gateway-card__head">
      <div class="gateway-card__icon"><CloudCog v-if="status.target.kind === 'sub2api'" :size="22" /><ShieldCheck v-else :size="22" /></div>
      <div><span class="eyebrow">{{ status.target.primary ? 'PREFERRED TARGET' : 'ALTERNATE TARGET' }}</span><h3>{{ status.target.name }}</h3></div>
      <span class="gateway-card__kind">{{ status.target.kind }}</span>
    </div>
    <div class="gateway-card__status">
      <span class="gateway-card__dot" />
      <strong>{{ status.health.status === 'ok' ? '运行正常' : status.health.status === 'disabled' ? '已停用' : '连接异常' }}</strong>
      <small v-if="status.health.latencyMs != null">{{ status.health.latencyMs }} ms</small>
    </div>
    <code class="gateway-card__endpoint">{{ status.target.baseUrl }}</code>
    <div class="gateway-card__meta">
      <span><Activity :size="13" />并发 {{ status.target.defaultConcurrency || 1 }}</span>
      <span>优先级 {{ status.target.defaultPriority || 0 }}</span>
		<span>{{ status.target.kind === 'cpa' ? '沿用 CPA 设置' : status.target.adminKeyConfigured ? '密钥已配置' : '缺少密钥' }}</span>
    </div>
    <div class="gateway-card__actions">
      <button class="button button--ghost button--small" type="button" @click="$emit('edit', status.target.id)">配置</button>
      <button class="button button--secondary button--small" type="button" :disabled="testing" @click="$emit('test', status.target.id)"><RefreshCw :size="14" :class="{ 'status-pulse': testing }" />{{ testing ? '检查中' : '检查连接' }}</button>
    </div>
  </article>
</template>
