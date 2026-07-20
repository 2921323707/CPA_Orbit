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
import type { Offer, OfferFeed } from '../types/api'
import { formatCurrency, formatDateTime, formatNumber, getErrorMessage, truncate } from '../utils/format'
import { offerInventory, offerItemId, offerMerchant, offerOrderUrl, offerShopId, offerUpdatedAt, settingsPriceThreshold } from '../utils/models'

type PriceSource = 'K12' | 'GPT Plus'

const k12Offers = ref<Offer[]>([])
const gptPlusFeed = ref<OfferFeed | null>(null)
const threshold = ref(0)
const loading = ref(true)
const refreshing = ref(false)
const error = ref('')
const selected = ref<{ offer: Offer; source: PriceSource } | null>(null)
const k12Page = ref(1)
const gptPlusPage = ref(1)
const pageSize = 5
const toast = useToast()

const sortedK12Offers = computed(() => [...k12Offers.value].sort((a, b) => Number(a.price) - Number(b.price)))
const sortedGptPlusOffers = computed(() => [...(gptPlusFeed.value?.offers ?? [])].sort((a, b) => Number(a.price) - Number(b.price)))
const k12TotalPages = computed(() => Math.max(1, Math.ceil(sortedK12Offers.value.length / pageSize)))
const gptPlusTotalPages = computed(() => Math.max(1, Math.ceil(sortedGptPlusOffers.value.length / pageSize)))
const pagedK12Offers = computed(() => sortedK12Offers.value.slice((k12Page.value - 1) * pageSize, k12Page.value * pageSize))
const pagedGptPlusOffers = computed(() => sortedGptPlusOffers.value.slice((gptPlusPage.value - 1) * pageSize, gptPlusPage.value * pageSize))

const isLow = (offer: Offer) => threshold.value > 0 && Number(offer.price) <= threshold.value
const offerKey = (source: PriceSource, offer: Offer, index: number) => `${source}-${offerItemId(offer) || offer.id || offer.title || index}`

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [offerData, plusData, settings] = await Promise.all([
      api.getOffers(),
      api.getGptPlus(),
      api.getSettings().catch(() => ({})),
    ])
    k12Offers.value = offerData
    gptPlusFeed.value = plusData
    k12Page.value = Math.min(k12Page.value, Math.max(1, Math.ceil(offerData.length / pageSize)))
    gptPlusPage.value = Math.min(gptPlusPage.value, Math.max(1, Math.ceil((plusData.offers?.length ?? 0) / pageSize)))
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
    await api.refreshOffers()
    k12Page.value = 1
    gptPlusPage.value = 1
    await load()
    toast.success('K12 与 GPT Plus 报价已刷新')
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    refreshing.value = false
  }
}

function showDetails(offer: Offer, source: PriceSource) {
  selected.value = { offer, source }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <div class="page-toolbar">
      <div>
        <p class="page-description">统一展示 K12 与 GPT Plus 未接码账号报价；阈值为 {{ threshold ? formatCurrency(threshold) : '未设置' }}。</p>
        <p v-if="gptPlusFeed?.updatedAt" class="page-meta">GPT Plus 最近采集：{{ formatDateTime(gptPlusFeed.updatedAt) }}</p>
      </div>
      <button class="button button--primary" type="button" :disabled="refreshing" @click="refresh">
        <RefreshCw :size="17" :class="{ spinning: refreshing }" /> {{ refreshing ? '刷新中…' : '刷新全部报价' }}
      </button>
    </div>

    <div v-if="gptPlusFeed?.lastError" class="inline-alert inline-alert--warning"><AlertTriangle :size="18" /><span>GPT Plus 本次抓取失败，当前展示上一次成功快照：{{ gptPlusFeed.lastError }}</span></div>

    <LoadingState v-if="loading" label="正在加载 K12 与 GPT Plus 报价…" />
    <ErrorState v-else-if="error" :message="error" @retry="load" />
    <template v-else>
      <section class="panel">
        <div class="panel__header panel__header--wrap">
          <div><h2>K12 报价</h2><p>PriceAI 链动小铺 K12 当前最低报价，最多 5 条</p></div>
          <a class="text-link" href="https://priceai.cc/products/chatgpt-team-business?tags=team_k12&collector=liandongShop&max=5" target="_blank" rel="noopener">查看来源 <ExternalLink :size="13" /></a>
        </div>
        <div v-if="pagedK12Offers.length" class="table-wrap price-table-wrap">
          <table class="data-table price-table">
            <caption class="sr-only">K12 当前报价</caption>
            <thead><tr><th>价格</th><th>商家</th><th>链动小铺 ID</th><th>库存</th><th>更新时间</th><th>标题</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="(offer, index) in pagedK12Offers" :key="offerKey('K12', offer, index)" :class="{ 'row--highlight': isLow(offer) }">
                <td><div class="price-cell"><strong>{{ formatCurrency(offer.price) }}</strong><StatusBadge v-if="isLow(offer)" tone="warning" label="低于阈值" /></div></td>
                <td>{{ offerMerchant(offer) }}</td>
                <td><code>{{ offerShopId(offer) }}</code></td>
                <td>{{ offerInventory(offer) == null ? '—' : typeof offerInventory(offer) === 'number' ? formatNumber(offerInventory(offer)) : offerInventory(offer) }}</td>
                <td class="nowrap">{{ formatDateTime(offerUpdatedAt(offer)) }}</td>
                <td class="title-cell" :title="offer.title">{{ truncate(offer.title, 42) }}</td>
                <td><div class="table-actions"><button class="button button--ghost button--small" type="button" @click="showDetails(offer, 'K12')"><Eye :size="15" />详情</button><a v-if="offerOrderUrl(offer)" class="button button--secondary button--small" :href="offerOrderUrl(offer)" target="_blank" rel="noopener"><ExternalLink :size="15" />购买</a><span v-else class="muted nowrap">无支付链接</span></div></td>
              </tr>
            </tbody>
          </table>
        </div>
        <PaginationBar v-if="sortedK12Offers.length > pageSize" :page="k12Page" :total-pages="k12TotalPages" :total="sortedK12Offers.length" :page-size="pageSize" @change="k12Page = $event" />
        <EmptyState v-else-if="!sortedK12Offers.length" title="暂无 K12 报价" description="点击“刷新全部报价”从 PriceAI 获取当前快照。" />
      </section>

      <section class="panel">
        <div class="panel__header panel__header--wrap">
          <div><h2>GPT Plus 报价 <StatusBadge tone="neutral" label="未接码" /></h2><p>仅展示 account_unverified 标签的未接码账号</p></div>
          <a class="text-link" href="https://priceai.cc/products/chatgpt-plus?tags=account_unverified" target="_blank" rel="noopener">查看来源 <ExternalLink :size="13" /></a>
        </div>
        <div v-if="pagedGptPlusOffers.length" class="table-wrap price-table-wrap">
          <table class="data-table price-table">
            <caption class="sr-only">GPT Plus 未接码账号报价</caption>
            <thead><tr><th>价格</th><th>类型</th><th>商家</th><th>库存</th><th>更新时间</th><th>商品</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="(offer, index) in pagedGptPlusOffers" :key="offerKey('GPT Plus', offer, index)" :class="{ 'row--highlight': isLow(offer) }">
                <td><div class="price-cell"><strong>{{ formatCurrency(offer.price) }}</strong><StatusBadge v-if="isLow(offer)" tone="warning" label="低于阈值" /></div></td>
                <td><StatusBadge tone="neutral" label="未接码" /></td>
                <td>{{ offerMerchant(offer) }}</td>
                <td>{{ offerInventory(offer) ?? offer.stock ?? '—' }}</td>
                <td class="nowrap">{{ formatDateTime(offerUpdatedAt(offer)) }}</td>
                <td class="title-cell" :title="offer.title">{{ truncate(offer.title, 50) }}</td>
                <td><div class="table-actions"><button class="button button--ghost button--small" type="button" @click="showDetails(offer, 'GPT Plus')"><Eye :size="15" />详情</button><a v-if="offerOrderUrl(offer)" class="button button--secondary button--small" :href="offerOrderUrl(offer)" target="_blank" rel="noopener"><ExternalLink :size="15" />购买</a><span v-else class="muted nowrap">无支付链接</span></div></td>
              </tr>
            </tbody>
          </table>
        </div>
        <PaginationBar v-if="sortedGptPlusOffers.length > pageSize" :page="gptPlusPage" :total-pages="gptPlusTotalPages" :total="sortedGptPlusOffers.length" :page-size="pageSize" @change="gptPlusPage = $event" />
        <EmptyState v-else-if="!sortedGptPlusOffers.length" title="暂无 GPT Plus 未接码报价" description="点击“刷新全部报价”从 PriceAI 获取当前快照。" />
      </section>
    </template>

    <BaseDrawer :open="Boolean(selected)" :title="`${selected?.source ?? ''} 报价详情`" @close="selected = null">
      <template v-if="selected">
        <div v-if="selected.source === 'GPT Plus'" class="inline-alert inline-alert--warning"><AlertTriangle :size="18" /><span>该报价来自未接码账号筛选页，购买后仍需自行完成接码验证。</span></div>
        <div v-else-if="isLow(selected.offer)" class="inline-alert inline-alert--warning"><AlertTriangle :size="18" /><span>该报价低于当前阈值 {{ formatCurrency(threshold) }}</span></div>
        <dl class="detail-list">
          <div><dt>账号类型</dt><dd>{{ selected.source }}<template v-if="selected.source === 'GPT Plus'"> · 未接码</template></dd></div>
          <div><dt>价格</dt><dd class="detail-price">{{ formatCurrency(selected.offer.price) }}</dd></div>
          <div><dt>商家</dt><dd>{{ offerMerchant(selected.offer) }}</dd></div>
          <div><dt>链动小铺 ID</dt><dd><code>{{ offerShopId(selected.offer) }}</code></dd></div>
          <div><dt>itemId</dt><dd><code>{{ offerItemId(selected.offer) }}</code></dd></div>
          <div class="detail-list__full"><dt>完整标题</dt><dd>{{ selected.offer.title || '—' }}</dd></div>
          <div class="detail-list__full"><dt>orderURL</dt><dd class="break-all">{{ offerOrderUrl(selected.offer) || '—' }}</dd></div>
          <div><dt>库存</dt><dd>{{ offerInventory(selected.offer) ?? '—' }}</dd></div>
          <div><dt>更新时间</dt><dd>{{ formatDateTime(offerUpdatedAt(selected.offer)) }}</dd></div>
        </dl>
      </template>
      <template v-if="selected && offerOrderUrl(selected.offer)" #footer>
        <a class="button button--primary" :href="offerOrderUrl(selected.offer)" target="_blank" rel="noopener"><ExternalLink :size="17" />直达支付</a>
      </template>
    </BaseDrawer>
  </div>
</template>
