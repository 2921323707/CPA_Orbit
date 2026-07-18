<script setup lang="ts">
import { AlertTriangle, ExternalLink, Eye, RefreshCw } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import BaseDrawer from '../components/common/BaseDrawer.vue'
import EmptyState from '../components/common/EmptyState.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import PaginationBar from '../components/common/PaginationBar.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { Offer } from '../types/api'
import { formatCurrency, formatDateTime, formatNumber, getErrorMessage, truncate } from '../utils/format'
import { offerInventory, offerItemId, offerMerchant, offerOrderUrl, offerShopId, offerUpdatedAt, settingsPriceThreshold } from '../utils/models'

const offers = ref<Offer[]>([])
const threshold = ref(0)
const loading = ref(true)
const refreshing = ref(false)
const error = ref('')
const selected = ref<Offer | null>(null)
const page = ref(1)
const pageSize = 10
const toast = useToast()
const sortedOffers = computed(() => [...offers.value].sort((a, b) => Number(a.price) - Number(b.price)))
const totalPages = computed(() => Math.max(1, Math.ceil(sortedOffers.value.length / pageSize)))
const pagedOffers = computed(() => sortedOffers.value.slice((page.value - 1) * pageSize, page.value * pageSize))

const isLow = (offer: Offer) => threshold.value > 0 && Number(offer.price) <= threshold.value

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [offerData, settings] = await Promise.all([api.getOffers(), api.getSettings().catch(() => ({}))])
    offers.value = offerData
    page.value = Math.min(page.value, Math.max(1, Math.ceil(offerData.length / pageSize)))
    threshold.value = settingsPriceThreshold(settings)
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function refresh() {
  refreshing.value = true
  try {
    const result = await api.refreshOffers()
    toast.success(result.message || '报价刷新任务已完成')
    await load()
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    refreshing.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <div class="page-toolbar">
      <div>
        <p class="page-description">展示价格最低的 10 条 K12 报价；阈值为 {{ threshold ? formatCurrency(threshold) : '未设置' }}。</p>
      </div>
      <button class="button button--primary" type="button" :disabled="refreshing" @click="refresh">
        <RefreshCw :size="17" :class="{ spinning: refreshing }" /> {{ refreshing ? '刷新中…' : '刷新报价' }}
      </button>
    </div>

    <LoadingState v-if="loading" label="正在加载报价…" />
    <ErrorState v-else-if="error" :message="error" @retry="load" />
    <section v-else class="panel">
      <div v-if="pagedOffers.length" class="table-wrap">
        <table class="data-table data-table--wide">
          <caption class="sr-only">K12 当前最低十条报价</caption>
          <thead><tr><th>价格</th><th>商家</th><th>链动小铺 ID</th><th>库存</th><th>更新时间</th><th>标题</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="offer in pagedOffers" :key="offerItemId(offer)" :class="{ 'row--highlight': isLow(offer) }">
              <td>
                <div class="price-cell"><strong>{{ formatCurrency(offer.price) }}</strong><StatusBadge v-if="isLow(offer)" tone="warning" label="低于阈值" /></div>
              </td>
              <td>{{ offerMerchant(offer) }}</td>
              <td><code>{{ offerShopId(offer) }}</code></td>
              <td>{{ offerInventory(offer) == null ? '—' : typeof offerInventory(offer) === 'number' ? formatNumber(offerInventory(offer)) : offerInventory(offer) }}</td>
              <td class="nowrap">{{ formatDateTime(offerUpdatedAt(offer)) }}</td>
              <td class="title-cell" :title="offer.title">{{ truncate(offer.title, 42) }}</td>
              <td>
                <div class="table-actions">
                  <button class="button button--ghost button--small" type="button" @click="selected = offer"><Eye :size="15" />详情</button>
                  <a v-if="offerOrderUrl(offer)" class="button button--secondary button--small" :href="offerOrderUrl(offer)" target="_blank" rel="noopener"><ExternalLink :size="15" />购买</a>
                  <span v-else class="muted nowrap">无支付链接</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <PaginationBar v-if="sortedOffers.length" :page="page" :total-pages="totalPages" :total="sortedOffers.length" :page-size="pageSize" @change="page = $event" />
      <EmptyState v-else title="暂无报价" description="点击“刷新报价”从后端获取最新结果。" />
    </section>

    <BaseDrawer :open="Boolean(selected)" title="报价详情" @close="selected = null">
      <template v-if="selected">
        <div v-if="isLow(selected)" class="inline-alert inline-alert--warning"><AlertTriangle :size="18" /><span>该报价低于当前阈值 {{ formatCurrency(threshold) }}</span></div>
        <dl class="detail-list">
          <div><dt>价格</dt><dd class="detail-price">{{ formatCurrency(selected.price) }}</dd></div>
          <div><dt>商家</dt><dd>{{ offerMerchant(selected) }}</dd></div>
          <div><dt>链动小铺 ID</dt><dd><code>{{ offerShopId(selected) }}</code></dd></div>
          <div><dt>itemId</dt><dd><code>{{ offerItemId(selected) }}</code></dd></div>
          <div class="detail-list__full"><dt>完整标题</dt><dd>{{ selected.title || '—' }}</dd></div>
          <div class="detail-list__full"><dt>orderURL</dt><dd class="break-all">{{ offerOrderUrl(selected) || '—' }}</dd></div>
          <div><dt>库存</dt><dd>{{ offerInventory(selected) ?? '—' }}</dd></div>
          <div><dt>更新时间</dt><dd>{{ formatDateTime(offerUpdatedAt(selected)) }}</dd></div>
        </dl>
      </template>
      <template v-if="selected && offerOrderUrl(selected)" #footer>
        <a class="button button--primary" :href="offerOrderUrl(selected)" target="_blank" rel="noopener"><ExternalLink :size="17" />直达支付</a>
      </template>
    </BaseDrawer>
  </div>
</template>
