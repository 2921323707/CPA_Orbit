<script setup lang="ts">
import { AlertTriangle, CheckCircle2, CircleDollarSign, FileJson2 } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import EmptyState from '../components/common/EmptyState.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import KpiCard from '../components/dashboard/KpiCard.vue'
import PriceDistributionChart from '../components/dashboard/PriceDistributionChart.vue'
import { api } from '../services/api'
import type { DashboardResponse } from '../types/api'
import { formatCurrency, formatNumber, getErrorMessage, truncate } from '../utils/format'
import {
  offerMerchant,
  offerOrderUrl,
  statActionRequired,
  statConnectedCount,
  statLowestPrice,
  statSubscriptionCount,
} from '../utils/models'

const loading = ref(true)
const error = ref('')
const dashboard = ref<DashboardResponse | null>(null)

const offers = computed(() => dashboard.value?.offers ?? [])
const gptPlusOffers = computed(() => dashboard.value?.gptPlusOffers ?? [])
const allOffers = computed(() => [
  ...offers.value.map((offer) => ({ ...offer, accountType: 'K12' })),
  ...gptPlusOffers.value.map((offer) => ({ ...offer, accountType: 'GPT Plus' })),
].sort((a, b) => Number(a.price) - Number(b.price)))
const k12TopOffers = computed(() => [...offers.value].sort((a, b) => Number(a.price) - Number(b.price)).slice(0, 3))
const gptPlusTopOffers = computed(() => [...gptPlusOffers.value].sort((a, b) => Number(a.price) - Number(b.price)).slice(0, 3))
const priceHistory = computed(() => dashboard.value?.priceHistory ?? [])
const gptPlusPriceHistory = computed(() => dashboard.value?.gptPlusPriceHistory ?? [])
const alerts = computed(() => dashboard.value?.alerts ?? [])
const stats = computed(() => dashboard.value?.stats ?? {})
const threshold = computed(() => Number(stats.value.threshold ?? alerts.value.find((item) => item.threshold != null)?.threshold ?? 0))
const lowOffer = computed(() => allOffers.value.find((item) => threshold.value > 0 && Number(item.price) <= threshold.value))
const lowest = computed(() => statLowestPrice(stats.value) ?? allOffers.value.reduce<number | null>((result, item) => result === null || Number(item.price) < result ? Number(item.price) : result, null))
const subscriptionCount = computed(() => statSubscriptionCount(stats.value))
const connectedCount = computed(() => statConnectedCount(stats.value))
const actionRequired = computed(() => statActionRequired(stats.value))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getDashboard()
    dashboard.value = {
      offers: Array.isArray(data.offers) ? data.offers : [],
      gptPlusOffers: Array.isArray(data.gptPlusOffers) ? data.gptPlusOffers : [],
      priceHistory: Array.isArray(data.priceHistory) ? data.priceHistory : [],
      gptPlusPriceHistory: Array.isArray(data.gptPlusPriceHistory) ? data.gptPlusPriceHistory : [],
      subscriptions: Array.isArray(data.subscriptions) ? data.subscriptions : [],
      alerts: Array.isArray(data.alerts) ? data.alerts : [],
      stats: data.stats ?? {},
    }
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <LoadingState v-if="loading" label="正在汇总监控数据…" />
    <ErrorState v-else-if="error" :message="error" @retry="load" />
    <template v-else-if="dashboard">
      <section v-if="lowOffer" class="alert-banner" role="status">
        <div class="alert-banner__icon"><AlertTriangle :size="21" /></div>
        <div>
          <strong>发现低于提醒阈值的报价</strong>
          <p>{{ offerMerchant(lowOffer) }} 当前 {{ formatCurrency(lowOffer.price) }}，阈值 {{ formatCurrency(threshold) }}。</p>
        </div>
        <a v-if="offerOrderUrl(lowOffer)" class="button button--primary button--small" :href="offerOrderUrl(lowOffer)" target="_blank" rel="noopener">直达支付</a>
        <RouterLink v-else class="button button--secondary button--small" to="/offers">查看报价</RouterLink>
      </section>

      <section class="kpi-grid" aria-label="关键指标">
        <KpiCard label="最低报价" :value="formatCurrency(lowest)" hint="当前采集结果" :icon="CircleDollarSign" />
        <KpiCard label="订阅总数" :value="formatNumber(subscriptionCount)" hint="已导入订阅文件" :icon="FileJson2" />
        <KpiCard label="已连通" :value="formatNumber(connectedCount)" hint="最近检查通过" :icon="CheckCircle2" tone="success" />
        <KpiCard label="需处理" :value="formatNumber(actionRequired)" hint="未读提醒或异常" :icon="AlertTriangle" :tone="actionRequired ? 'warning' : 'default'" />
      </section>

      <PriceDistributionChart :k12-history="priceHistory" :gpt-plus-history="gptPlusPriceHistory" @history-deleted="load" />

      <div class="split-grid">
        <section class="panel">
          <div class="panel__header">
            <div><h2>最低报价</h2><p>K12 与 GPT Plus 各取最低三条</p></div>
            <RouterLink class="text-link" to="/offers">查看全部</RouterLink>
          </div>
          <div v-if="k12TopOffers.length || gptPlusTopOffers.length" class="overview-offer-groups">
            <div class="overview-offer-group">
              <div class="overview-offer-group__title"><StatusBadge tone="neutral" label="K12" /><span>最低 3 条</span></div>
              <div v-for="offer in k12TopOffers" :key="`k12-${String(offer.id ?? offer.itemId ?? offer.title)}`" class="overview-offer-row"><div><strong>{{ offerMerchant(offer) }}</strong><span :title="offer.title">{{ truncate(offer.title, 34) }}</span></div><strong>{{ formatCurrency(offer.price) }}</strong></div>
            </div>
            <div class="overview-offer-group">
              <div class="overview-offer-group__title"><StatusBadge tone="neutral" label="GPT Plus" /><span>未接码 · 最低 3 条</span></div>
              <div v-for="offer in gptPlusTopOffers" :key="`plus-${String(offer.id ?? offer.itemId ?? offer.title)}`" class="overview-offer-row"><div><strong>{{ offerMerchant(offer) }}</strong><span :title="offer.title">{{ truncate(offer.title, 34) }}</span></div><strong>{{ formatCurrency(offer.price) }}</strong></div>
            </div>
          </div>
          <EmptyState v-else title="暂无报价" description="刷新报价后，这里会分别展示两类账号的最低三条记录。" />
        </section>

        <section class="panel">
          <div class="panel__header">
            <div><h2>待定义</h2><p>预留给后续运营模块</p></div>
          </div>
          <EmptyState title="待定义" description="该区域已移除最近订阅，等待后续确定新的运营内容。" />
        </section>
      </div>
    </template>
  </div>
</template>
