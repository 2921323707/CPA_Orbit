<script setup lang="ts">
import { AlertTriangle, CheckCircle2, CircleDollarSign, FileJson2, ShoppingCart } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import EmptyState from '../components/common/EmptyState.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import KpiCard from '../components/dashboard/KpiCard.vue'
import PriceDistributionChart from '../components/dashboard/PriceDistributionChart.vue'
import { api } from '../services/api'
import type { DashboardResponse } from '../types/api'
import { formatCurrency, formatDateTime, formatNumber, getErrorMessage, truncate } from '../utils/format'
import {
  offerMerchant,
  offerOrderUrl,
  offerUpdatedAt,
  statActionRequired,
  statConnectedCount,
  statLowestPrice,
  statSubscriptionCount,
  subscriptionCheckedAt,
  subscriptionConnected,
  subscriptionFile,
} from '../utils/models'

const loading = ref(true)
const error = ref('')
const dashboard = ref<DashboardResponse | null>(null)

const offers = computed(() => dashboard.value?.offers ?? [])
const priceHistory = computed(() => dashboard.value?.priceHistory ?? [])
const gptPlusPriceHistory = computed(() => dashboard.value?.gptPlusPriceHistory ?? [])
const subscriptions = computed(() => dashboard.value?.subscriptions ?? [])
const alerts = computed(() => dashboard.value?.alerts ?? [])
const stats = computed(() => dashboard.value?.stats ?? {})
const threshold = computed(() => Number(stats.value.threshold ?? alerts.value.find((item) => item.threshold != null)?.threshold ?? 0))
const lowOffer = computed(() => offers.value.find((item) => threshold.value > 0 && Number(item.price) <= threshold.value))
const lowest = computed(() => statLowestPrice(stats.value) ?? offers.value.reduce<number | null>((result, item) => result === null || Number(item.price) < result ? Number(item.price) : result, null))
const subscriptionCount = computed(() => statSubscriptionCount(stats.value))
const connectedCount = computed(() => statConnectedCount(stats.value))
const actionRequired = computed(() => statActionRequired(stats.value))

function subscriptionState(item: DashboardResponse['subscriptions'][number]) {
  if (subscriptionConnected(item)) return { tone: 'success' as const, label: '已连通' }
  const status = String(item.connectivity?.status ?? '').toLowerCase()
  if (!status || status === 'unknown') return { tone: 'neutral' as const, label: '未检查' }
  return { tone: 'danger' as const, label: '异常' }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getDashboard()
    dashboard.value = {
      offers: Array.isArray(data.offers) ? data.offers : [],
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

      <PriceDistributionChart :k12-history="priceHistory" :gpt-plus-history="gptPlusPriceHistory" />

      <div class="split-grid">
        <section class="panel">
          <div class="panel__header">
            <div><h2>前 5 报价</h2><p>当前最低价格优先</p></div>
            <RouterLink class="text-link" to="/offers">查看全部</RouterLink>
          </div>
          <div v-if="offers.length" class="table-wrap">
            <table>
              <thead><tr><th>商家</th><th>标题</th><th class="numeric">价格</th><th>状态</th></tr></thead>
              <tbody>
                <tr v-for="offer in offers.slice(0, 5)" :key="String(offer.id ?? offer.itemId ?? offer.title)">
                  <td>{{ offerMerchant(offer) }}</td>
                  <td :title="offer.title">{{ truncate(offer.title, 28) }}</td>
                  <td class="numeric strong">{{ formatCurrency(offer.price) }}</td>
                  <td><StatusBadge v-if="threshold && offer.price <= threshold" tone="warning" label="低于阈值" /><span v-else class="muted">常规</span></td>
                </tr>
              </tbody>
            </table>
          </div>
          <EmptyState v-else title="暂无报价" description="刷新报价后，这里会展示最低的五条记录。" />
        </section>

        <section class="panel">
          <div class="panel__header">
            <div><h2>最近订阅</h2><p>最新导入和检查状态</p></div>
            <RouterLink class="text-link" to="/subscriptions">管理订阅</RouterLink>
          </div>
          <div v-if="subscriptions.length" class="compact-list">
            <article v-for="item in subscriptions.slice(0, 5)" :key="item.id" class="compact-list__item">
              <div class="compact-list__icon"><FileJson2 :size="17" /></div>
              <div class="compact-list__body">
                <strong>{{ item.email || subscriptionFile(item) }}</strong>
                <span>{{ subscriptionFile(item) }} · {{ formatDateTime(subscriptionCheckedAt(item)) }}</span>
              </div>
              <StatusBadge :tone="subscriptionState(item).tone" :label="subscriptionState(item).label" />
            </article>
          </div>
          <EmptyState v-else title="暂无订阅" description="导入 JSON 订阅文件后即可查看连通状态。" />
        </section>
      </div>
    </template>
  </div>
</template>
