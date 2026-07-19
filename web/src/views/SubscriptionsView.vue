<script setup lang="ts">
import { CloudUpload, Eye, ExternalLink, FileJson2, FolderSync, KeyRound, Play, Search, TestTube2, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import BaseDrawer from '../components/common/BaseDrawer.vue'
import EmptyState from '../components/common/EmptyState.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import PaginationBar from '../components/common/PaginationBar.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import QuotaCell from '../components/subscriptions/QuotaCell.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { Subscription, SubscriptionConnectivity } from '../types/api'
import { formatCurrency, formatDateTime, getErrorMessage } from '../utils/format'
import {
  subscriptionAccountId,
  subscriptionAcquisitionPrice,
  subscriptionBaseUrl,
  subscriptionCheckedAt,
  subscriptionFile,
  subscriptionLastError,
  subscriptionPlan,
  subscriptionProvider,
} from '../utils/models'

const subscriptions = ref<Subscription[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const totalPages = ref(0)
const folders = ref<string[]>([])
const loading = ref(true)
const error = ref('')
const selected = ref<Subscription | null>(null)
const selectedIds = ref(new Set<string | number>())
const files = ref<File[]>([])
const acquisitionPrice = ref('')
const importing = ref(false)
const dragging = ref(false)
const search = ref('')
const statusFilter = ref('all')
const folderFilter = ref('all')
const testingIds = ref(new Set<string | number>())
const batchTesting = ref(false)
const batchProgress = ref({ current: 0, total: 0 })
const batchSyncing = ref(false)
const batchDeleting = ref(false)
const syncingId = ref<string | number | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const toast = useToast()

const selectedItems = computed(() => subscriptions.value.filter((item) => selectedIds.value.has(item.id)))
const allPageSelected = computed(() => subscriptions.value.length > 0 && subscriptions.value.every((item) => selectedIds.value.has(item.id)))
function categoryFromConnectivity(connectivity?: SubscriptionConnectivity | null) {
  const fiveHour = connectivity?.quota?.fiveHour?.remainingPercent
  const sevenDay = connectivity?.quota?.sevenDay?.remainingPercent
  if (fiveHour != null && fiveHour <= 0 && (sevenDay == null || sevenDay > 0)) return 'normal'
  if (connectivity?.status === 'ok') return 'normal'
  return 'error'
}

function connectivityState(item: Subscription): { tone: 'success' | 'warning' | 'danger'; label: string } {
  const category = item.category || categoryFromConnectivity(item.connectivity)
  const fiveHour = item.connectivity?.quota?.fiveHour?.remainingPercent
  const sevenDay = item.connectivity?.quota?.sevenDay?.remainingPercent
  if (category === 'normal' && fiveHour != null && fiveHour <= 0 && (sevenDay == null || sevenDay > 0)) return { tone: 'warning', label: '5H 额度耗尽' }
  if (category === 'normal') return { tone: 'success', label: '正常' }
  return { tone: 'danger', label: '异常' }
}

function addFiles(list: FileList | File[]) {
  const incoming = Array.from(list).filter((file) => file.name.toLowerCase().endsWith('.json'))
  if (incoming.length !== Array.from(list).length) toast.error('仅支持 JSON 文件，已忽略其他格式')
  const existing = new Set(files.value.map((file) => `${file.name}-${file.size}-${file.lastModified}`))
  files.value = [...files.value, ...incoming.filter((file) => !existing.has(`${file.name}-${file.size}-${file.lastModified}`))]
}

function onDrop(event: DragEvent) {
  dragging.value = false
  if (event.dataTransfer?.files) addFiles(event.dataTransfer.files)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await api.getSubscriptions({
      page: page.value,
      pageSize,
      folder: folderFilter.value,
      status: statusFilter.value,
      search: search.value.trim(),
    })
    subscriptions.value = result.subscriptions
    total.value = result.total
    page.value = result.page || 1
    totalPages.value = result.totalPages
    folders.value = result.folders || []
    selectedIds.value = new Set()
    for (const item of subscriptions.value) {
      await testOne(item, true)
    }
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function importFiles() {
  if (!files.value.length) return toast.error('请先选择 JSON 文件')
  importing.value = true
  const queued = [...files.value]
  if (queued.length === 1 && !acquisitionPrice.value.trim()) {
    toast.show('未填写入手价格，将按可选字段为空继续导入', 'info')
  }
  let imported = 0
  let failed = 0
  const failureDetails: string[] = []
  for (const file of queued) {
    try {
      await api.importSubscriptions({ file, acquisitionPrice: queued.length === 1 ? acquisitionPrice.value.trim() : undefined })
      imported += 1
    } catch (err) {
      failed += 1
      failureDetails.push(`${file.name}：${getErrorMessage(err)}`)
    }
  }
  const message = `导入完成：成功 ${imported} · 失败 ${failed}`
  if (failed) toast.error(`${message}；${failureDetails.slice(0, 2).join('；')}`)
  else toast.success(message)
  files.value = []
  acquisitionPrice.value = ''
  if (fileInput.value) fileInput.value.value = ''
  page.value = 1
  await load()
  importing.value = false
}

async function testOne(item: Subscription, quiet = false) {
  const id = item.id
  testingIds.value = new Set(testingIds.value).add(id)
  try {
    const result = await api.testSubscription(id)
    const target = subscriptions.value.find((candidate) => candidate.id === id)
    if (target) {
      target.connectivity = result
      target.category = categoryFromConnectivity(result)
    }
    const status = String(result.status ?? 'error').toLowerCase()
    if (!quiet) {
      if (status === 'ok') toast.success(`${item.email || subscriptionFile(item)} 正常，延迟 ${result.latencyMs ?? 0} ms`)
      else if (status === 'not_in_cpa_pool') toast.show(`${item.email || subscriptionFile(item)} 尚未同步到 CPA 活动池`, 'info')
      else toast.error(`${item.email || subscriptionFile(item)}：${connectivityState({ ...item, connectivity: result }).label} · ${result.error || '检测失败'}`)
    }
    return status
  } catch (err) {
    if (!quiet) toast.error(getErrorMessage(err))
    return 'request_error'
  } finally {
    const next = new Set(testingIds.value)
    next.delete(id)
    testingIds.value = next
  }
}

async function testBatch() {
  const items = [...selectedItems.value]
  if (!items.length) return
  batchTesting.value = true
  batchProgress.value = { current: 0, total: items.length }
  const counts = new Map<string, number>()
  for (const item of items) {
    const status = await testOne(item, true)
    counts.set(status, (counts.get(status) || 0) + 1)
    batchProgress.value.current += 1
  }
  const normal = counts.get('ok') || 0
  const fiveHour = items.filter((item) => item.connectivity?.quota?.fiveHour?.remainingPercent === 0 && (item.connectivity?.quota?.sevenDay?.remainingPercent ?? 1) > 0).length
  toast.show(`批量检测完成：正常 ${normal} · 5H耗尽 ${fiveHour} · 异常 ${items.length - normal - fiveHour}`, normal === items.length ? 'success' : 'info')
  batchTesting.value = false
  await load()
}

async function syncToCpa(item: Subscription, quiet = false) {
  syncingId.value = item.id
  try {
    await api.syncSubscription(item.id)
    if (!quiet) toast.success('已同步到 CPA 运行池')
    return true
  } catch (err) {
    if (!quiet) toast.error(getErrorMessage(err))
    return false
  } finally {
    syncingId.value = null
  }
}

function togglePageSelection(checked: boolean) {
  selectedIds.value = checked ? new Set(subscriptions.value.map((item) => item.id)) : new Set()
}

function toggleSelection(id: string | number, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked) next.add(id)
  else next.delete(id)
  selectedIds.value = next
}

async function syncSelected() {
  const items = [...selectedItems.value]
  if (!items.length) return
  batchSyncing.value = true
  let synced = 0
  for (const item of items) {
    if (await syncToCpa(item, true)) synced += 1
  }
  batchSyncing.value = false
  toast.show(`批量同步完成：${synced}/${items.length}`, synced === items.length ? 'success' : 'info')
  await load()
}

async function deleteSelected() {
  const items = [...selectedItems.value]
  if (!items.length) return
  if (!window.confirm(`确认删除选中的 ${items.length} 个归档文件？此操作不会删除 CPA 运行池中的副本。`)) return
  batchDeleting.value = true
  let deleted = 0
  for (const item of items) {
    try {
      await api.deleteSubscription(item.id)
      deleted += 1
    } catch {
      // Continue deleting the remaining selected archives.
    }
  }
  batchDeleting.value = false
  if (deleted === items.length) toast.success(`已删除 ${deleted} 个归档文件`)
  else toast.error(`已删除 ${deleted}/${items.length} 个归档文件`)
  if (subscriptions.value.length === deleted && page.value > 1) page.value -= 1
  await load()
}

async function goToPage(nextPage: number) {
  if (nextPage < 1 || nextPage > totalPages.value || nextPage === page.value) return
  page.value = nextPage
  await load()
}

let searchTimer = 0
watch([statusFilter, folderFilter], () => {
  page.value = 1
  load()
})
watch(search, () => {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    page.value = 1
    load()
  }, 300)
})

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <section class="panel import-panel">
      <div class="panel__header"><div><h2>导入订阅 JSON</h2><p>支持单个或批量导入；单个导入时可记录入手价格。</p></div></div>
      <div class="import-layout">
        <div class="drop-zone" :class="{ 'drop-zone--active': dragging }" role="button" tabindex="0" @click="fileInput?.click()" @keydown.enter="fileInput?.click()" @keydown.space.prevent="fileInput?.click()" @dragover.prevent="dragging = true" @dragleave.prevent="dragging = false" @drop.prevent="onDrop">
          <CloudUpload :size="30" aria-hidden="true" />
          <strong>拖拽 JSON 文件到这里</strong>
          <span>或点击选择文件</span>
          <input ref="fileInput" class="sr-only" type="file" accept="application/json,.json" multiple @change="addFiles(($event.target as HTMLInputElement).files || [])" />
        </div>
        <div v-if="files.length === 1" class="import-fields">
          <label class="field"><span>入手价格 <small>可选</small></span><input v-model="acquisitionPrice" type="number" min="0" step="0.01" inputmode="decimal" placeholder="例如 12.00" /></label>
          <small class="import-field-hint">仅单个导入时记录；批量导入请逐个补录。</small>
        </div>
      </div>
      <div class="import-tools" aria-label="订阅辅助工具">
        <a class="import-tool" href="https://cvt.okcode.cc.cd/" target="_blank" rel="noopener noreferrer">
          <span class="import-tool__icon"><FileJson2 :size="19" /></span>
          <span class="import-tool__copy"><strong>JSON 转换台</strong><small>转换、整理后再导入 CPA 订阅</small></span>
          <ExternalLink :size="15" aria-hidden="true" />
        </a>
        <a class="import-tool import-tool--cdk" href="https://www.kezongai.top/" target="_blank" rel="noopener noreferrer">
          <span class="import-tool__icon"><KeyRound :size="19" /></span>
          <span class="import-tool__copy"><strong>CDK 兑换入口</strong><small>打开兑换平台处理兑换码</small></span>
          <ExternalLink :size="15" aria-hidden="true" />
        </a>
      </div>
      <div v-if="files.length" class="file-queue">
        <div v-for="(file, index) in files" :key="`${file.name}-${file.lastModified}`" class="file-chip"><FileJson2 :size="15" /><span>{{ file.name }}</span><button type="button" :aria-label="`移除 ${file.name}`" @click="files.splice(index, 1)"><Trash2 :size="14" /></button></div>
      </div>
      <div class="form-actions"><span class="muted">已选择 {{ files.length }} 个文件</span><button class="button button--primary" type="button" :disabled="!files.length || importing" @click="importFiles"><CloudUpload :size="17" />{{ importing ? '导入中…' : '开始导入' }}</button></div>
    </section>

    <section class="panel">
      <div class="panel__header panel__header--wrap">
        <div><h2>订阅列表</h2><p>共 {{ total }} 条 · 第 {{ page }}/{{ totalPages || 1 }} 页 · 每页 10 条 · 加载时自动刷新额度</p></div>
        <div class="batch-actions">
          <span class="selection-count">已选 {{ selectedIds.size }} 条</span>
          <button class="button button--secondary button--small" type="button" :disabled="batchTesting || !selectedItems.length" @click="testBatch"><Play :size="15" />{{ batchTesting ? `检测 ${batchProgress.current}/${batchProgress.total}` : '批量检测' }}</button>
          <button class="button button--secondary button--small" type="button" :disabled="batchSyncing || !selectedItems.length" @click="syncSelected"><FolderSync :size="15" />{{ batchSyncing ? '同步中' : '批量同步' }}</button>
          <button class="button button--danger button--small" type="button" :disabled="batchDeleting || !selectedItems.length" @click="deleteSelected"><Trash2 :size="15" />{{ batchDeleting ? '删除中' : '删除所选' }}</button>
        </div>
      </div>
      <div class="filters" aria-label="订阅筛选">
        <label class="search-field"><Search :size="17" /><span class="sr-only">搜索订阅</span><input v-model="search" type="search" placeholder="搜索邮箱、文件、account id" /></label>
        <label class="field field--compact"><span class="sr-only">状态筛选</span><select v-model="statusFilter"><option value="all">全部分类</option><option value="normal">正常（含 5H 额度耗尽）</option><option value="error">异常</option></select></label>
        <label class="field field--compact"><span class="sr-only">文件夹筛选</span><select v-model="folderFilter"><option value="all">全部文件夹</option><option v-for="folder in folders" :key="folder" :value="folder">{{ folder }}</option></select></label>
      </div>

      <LoadingState v-if="loading" label="正在加载订阅…" />
      <ErrorState v-else-if="error" :message="error" @retry="load" />
      <template v-else-if="subscriptions.length">
        <div class="table-wrap subscription-list-wrap">
        <table class="data-table data-table--wide subscription-table">
          <thead><tr><th class="selection-column"><input type="checkbox" :checked="allPageSelected" aria-label="全选本页" @change="togglePageSelection(($event.target as HTMLInputElement).checked)" /></th><th>邮箱</th><th>分类</th><th class="numeric">入手价</th><th class="numeric">延迟</th><th>5H 额度</th><th>7D 额度</th><th>订阅文件</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="item in subscriptions" :key="item.id" :class="{ 'row--selected': selectedIds.has(item.id) }">
              <td class="selection-column"><input type="checkbox" :checked="selectedIds.has(item.id)" :aria-label="`选择 ${item.email || subscriptionFile(item)}`" @change="toggleSelection(item.id, ($event.target as HTMLInputElement).checked)" /></td>
              <td class="strong subscription-email" data-label="邮箱" :title="item.email || ''">{{ item.email || '—' }}</td>
              <td data-label="分类"><StatusBadge :tone="connectivityState(item).tone" :label="connectivityState(item).label" /></td>
              <td class="numeric nowrap" data-label="入手价">{{ subscriptionAcquisitionPrice(item) == null ? '—' : formatCurrency(subscriptionAcquisitionPrice(item)) }}</td>
              <td class="numeric nowrap" data-label="延迟">{{ item.connectivity?.latencyMs == null || !item.connectivity?.httpStatus ? '—' : `${item.connectivity.latencyMs} ms` }}</td>
              <td data-label="5H 额度"><QuotaCell label="5 小时" :window="item.connectivity?.quota?.fiveHour" /></td>
              <td data-label="7D 额度"><QuotaCell label="7 天" :window="item.connectivity?.quota?.sevenDay" /></td>
              <td data-label="订阅文件"><span class="file-name" :title="subscriptionFile(item)"><FileJson2 class="file-name__icon" :size="15" aria-hidden="true" />{{ subscriptionFile(item) }}</span></td>
              <td data-label="操作"><div class="table-actions"><button class="button button--ghost button--small" type="button" @click="selected = item"><Eye :size="15" />详情</button><button class="button button--secondary button--small" type="button" :disabled="testingIds.has(item.id) || batchTesting" @click="testOne(item)"><TestTube2 :size="15" />{{ testingIds.has(item.id) ? '检测中' : '检测' }}</button></div></td>
            </tr>
          </tbody>
        </table>
        </div>
        <PaginationBar :page="page" :total-pages="totalPages" :total="total" :page-size="pageSize" @change="goToPage" />
      </template>
      <EmptyState v-else title="没有匹配的订阅" description="调整筛选条件，或先在上方导入 JSON 文件。" />
    </section>

    <BaseDrawer :open="Boolean(selected)" title="订阅详情" @close="selected = null">
      <template v-if="selected">
        <div class="drawer-status-row">
          <StatusBadge :tone="connectivityState(selected).tone" :label="connectivityState(selected).label" />
          <StatusBadge v-if="selected.connectivity?.httpStatus && !connectivityState(selected).label.includes('HTTP')" :tone="selected.connectivity.httpStatus === 200 ? 'success' : 'danger'" :label="`HTTP ${selected.connectivity.httpStatus}`" />
        </div>
        <dl class="detail-list">
          <div><dt>邮箱</dt><dd>{{ selected.email || '—' }}</dd></div>
          <div><dt>account id</dt><dd><code>{{ subscriptionAccountId(selected) }}</code></dd></div>
          <div><dt>Provider</dt><dd>{{ subscriptionProvider(selected) }}</dd></div>
          <div><dt>入手价格</dt><dd>{{ subscriptionAcquisitionPrice(selected) == null ? '—' : formatCurrency(subscriptionAcquisitionPrice(selected)) }}</dd></div>
          <div><dt>计划</dt><dd>{{ selected.connectivity?.quota?.planType || subscriptionPlan(selected) }}</dd></div>
          <div><dt>最后检查</dt><dd>{{ formatDateTime(subscriptionCheckedAt(selected)) }}</dd></div>
          <div><dt>调用延迟</dt><dd>{{ selected.connectivity?.latencyMs == null || !selected.connectivity?.httpStatus ? '—' : `${selected.connectivity.latencyMs} ms` }}</dd></div>
          <div><dt>CPA 状态</dt><dd>{{ selected.connectivity?.cpaStatusMessage || selected.connectivity?.cpaStatus || '—' }}</dd></div>
          <div><dt>5H 剩余额度</dt><dd><QuotaCell label="5 小时" :window="selected.connectivity?.quota?.fiveHour" /></dd></div>
          <div><dt>7D 剩余额度</dt><dd><QuotaCell label="7 天" :window="selected.connectivity?.quota?.sevenDay" /></dd></div>
          <div><dt>下次可重试</dt><dd>{{ formatDateTime(selected.connectivity?.nextRetryAt) }}</dd></div>
          <div><dt>附加 Credits</dt><dd>{{ selected.connectivity?.quota?.unlimited ? '无限' : selected.connectivity?.quota?.creditsBalance ?? '—' }}</dd></div>
          <div class="detail-list__full"><dt>订阅文件</dt><dd class="break-all">{{ subscriptionFile(selected) }}</dd></div>
          <div class="detail-list__full"><dt>base_url</dt><dd class="break-all"><code>{{ subscriptionBaseUrl(selected) || '—' }}</code></dd></div>
          <div class="detail-list__full"><dt>检测结果</dt><dd :class="{ 'error-text': subscriptionLastError(selected) }">{{ subscriptionLastError(selected) || '正常' }}</dd></div>
        </dl>
        <p class="security-note">为保护凭据，本页面不会显示 token。</p>
      </template>
      <template v-if="selected" #footer>
        <button class="button button--secondary" type="button" :disabled="testingIds.has(selected.id)" @click="testOne(selected)"><TestTube2 :size="17" />检测状态与额度</button>
        <button class="button button--primary" type="button" :disabled="syncingId === selected.id" @click="syncToCpa(selected)"><FolderSync :size="17" />{{ syncingId === selected.id ? '同步中…' : '同步到 CPA' }}</button>
      </template>
    </BaseDrawer>
  </div>
</template>
