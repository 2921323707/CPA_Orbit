<script setup lang="ts">
import { CloudUpload, Eye, ExternalLink, FileJson2, FolderSync, KeyRound, Network, Play, RefreshCw, Search, TestTube2, Trash2, Unplug } from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import BaseDrawer from '../components/common/BaseDrawer.vue'
import EmptyState from '../components/common/EmptyState.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import PaginationBar from '../components/common/PaginationBar.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import QuotaCell from '../components/subscriptions/QuotaCell.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { DeploymentBinding, GatewayTarget, ImportPreflightResponse, ImportTargetOption, Subscription, SubscriptionConnectivity, SubscriptionInsights, SubscriptionPollStatus } from '../types/api'
import { formatCurrency, formatDateTime, getErrorMessage } from '../utils/format'
import {
  subscriptionAccountId,
  subscriptionAcquisitionPrice,
  subscriptionBaseUrl,
  subscriptionCheckedAt,
  subscriptionCheckFreshness,
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
const insights = ref<SubscriptionInsights>({ normal: 0, pending: 0, error: 0, priced: 0, totalCost: 0, averageCost: 0, expiringSoon: 0 })
const loading = ref(true)
const error = ref('')
const selected = ref<Subscription | null>(null)
const selectedIds = ref(new Set<string | number>())
interface ImportQueueItem {
  key: string
  file: File
  status: 'queued' | 'preflighting' | 'ready' | 'committing' | 'committed' | 'error'
  preflight?: ImportPreflightResponse
  selectedTargetId?: number
  error?: string
}

const importQueue = ref<ImportQueueItem[]>([])
const acquisitionPrice = ref<string | number>('')
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
const deployingId = ref<string | number | null>(null)
const bindings = ref<Record<string, DeploymentBinding[]>>({})
const gatewayTargets = ref<GatewayTarget[]>([])
const pollStatus = ref<SubscriptionPollStatus | null>(null)
const pollingNow = ref(false)
const pollError = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const toast = useToast()

const selectedItems = computed(() => subscriptions.value.filter((item) => selectedIds.value.has(item.id)))
const allPageSelected = computed(() => subscriptions.value.length > 0 && subscriptions.value.every((item) => selectedIds.value.has(item.id)))
const primaryTarget = computed(() => gatewayTargets.value.find((target) => target.enabled && target.primary))
function itemBindings(item: Subscription) { return bindings.value[String(item.id)] || [] }
function activeBinding(item: Subscription) { return itemBindings(item).find((binding) => binding.observedState === 'active' && binding.mode === 'primary') || itemBindings(item).find((binding) => binding.observedState === 'active') }
function targetName(targetId?: number) { return gatewayTargets.value.find((target) => target.id === targetId)?.name || (targetId ? `Target ${targetId}` : '未部署') }
function categoryFromConnectivity(connectivity?: SubscriptionConnectivity | null) {
  const fiveHour = connectivity?.quota?.fiveHour?.remainingPercent
  const sevenDay = connectivity?.quota?.sevenDay?.remainingPercent
  if (fiveHour != null && fiveHour <= 0 && (sevenDay == null || sevenDay > 0)) return 'normal'
  if (connectivity?.status === 'ok') return 'normal'
	const status = String(connectivity?.status || 'unknown').toLowerCase()
	if (status === 'unknown' || status === 'pending') return 'pending'
  return 'error'
}

function connectivityState(item: Subscription): { tone: 'success' | 'warning' | 'danger' | 'neutral'; label: string } {
  const category = item.category || categoryFromConnectivity(item.connectivity)
  const fiveHour = item.connectivity?.quota?.fiveHour?.remainingPercent
  const sevenDay = item.connectivity?.quota?.sevenDay?.remainingPercent
  if (category === 'normal' && fiveHour != null && fiveHour <= 0 && (sevenDay == null || sevenDay > 0)) return { tone: 'warning', label: '5H 额度耗尽' }
  if (category === 'normal') return { tone: 'success', label: '正常' }
	if (category === 'pending') return { tone: 'neutral', label: '待检测' }
  return { tone: 'danger', label: '异常' }
}

function fileKey(file: File) {
  return `${file.name}-${file.size}-${file.lastModified}`
}

function compatibleTargets(item: ImportQueueItem) {
  return (item.preflight?.targets || []).filter((target) => target.enabled && target.compatible)
}

function addFiles(list: FileList | File[]) {
  const selectedFiles = Array.from(list)
  const incoming = selectedFiles.filter((file) => file.name.toLowerCase().endsWith('.json'))
  if (incoming.length !== selectedFiles.length) toast.error('仅支持 JSON 文件，已忽略其他格式')
  const existing = new Set(importQueue.value.map((item) => item.key))
  const additions = incoming.filter((file) => !existing.has(fileKey(file))).map((file) => ({ key: fileKey(file), file, status: 'queued' as const }))
  importQueue.value = [...importQueue.value, ...additions]
  for (const addition of additions) {
    const item = importQueue.value.find((candidate) => candidate.key === addition.key)
    if (item) void preflightItem(item)
  }
}

function removeImportItem(key: string) {
  importQueue.value = importQueue.value.filter((item) => item.key !== key)
  if (fileInput.value) fileInput.value.value = ''
}

function onDrop(event: DragEvent) {
  dragging.value = false
  if (event.dataTransfer?.files) addFiles(event.dataTransfer.files)
}

async function preflightItem(item: ImportQueueItem) {
  item.status = 'preflighting'
  item.error = ''
  item.preflight = undefined
  item.selectedTargetId = undefined
  try {
    const result = await api.preflightSubscriptionImport({ file: item.file })
    item.preflight = result
    item.status = 'ready'
  } catch (err) {
    item.status = 'error'
    item.error = getErrorMessage(err)
  }
}

function targetReason(target: ImportTargetOption) {
  const labels: Record<string, string> = {
    compatible_cpa_auth: '兼容 CPA Auth JSON',
    compatible_codex_session: '兼容 Sub2API Codex 会话',
    compatible_sub2api_package: '兼容 Sub2API 单账号数据包',
    sub2api_package_requires_sub2api: 'Sub2API 数据包仅可部署到 Sub2API',
    missing_supported_credential: '缺少该目标支持的凭据',
  }
  return labels[target.reasonCode] || target.reasonCode || '未提供兼容性原因'
}

function analysisState(item: ImportQueueItem, kind: 'duplicate' | 'conflict') {
  const state = item.preflight?.analysis[kind]
  if (!state) return { label: '后端未提供状态', tone: 'neutral' as const }
  const value = String(state.state || 'unknown').toLowerCase()
  if (value === 'none' || value === 'clear') return { label: state.message || (kind === 'duplicate' ? '未发现重复' : '未发现冲突'), tone: 'success' as const }
  if (value === 'duplicate' || value === 'conflict') return { label: state.message || state.reasonCode || (kind === 'duplicate' ? '检测到重复' : '检测到冲突'), tone: 'danger' as const }
  return { label: state.message || state.reasonCode || '后端未提供状态', tone: 'neutral' as const }
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
    insights.value = result.insights || { normal: 0, pending: 0, error: 0, priced: 0, totalCost: 0, averageCost: 0, expiringSoon: 0 }
    const [targetResult, bindingResults] = await Promise.all([
      api.getGatewayTargets().catch(() => ({ targets: [] as GatewayTarget[] })),
      Promise.all(result.subscriptions.map(async (item) => {
        const response = await api.getSubscriptionBindings(item.id).catch(() => ({ bindings: [] as DeploymentBinding[] }))
        return [String(item.id), response.bindings] as const
      })),
    ])
    gatewayTargets.value = targetResult.targets
    bindings.value = Object.fromEntries(bindingResults)
    selectedIds.value = new Set()
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function commitImports() {
  if (!importQueue.value.length) return toast.error('请先选择 JSON 文件')
  const ready = importQueue.value.filter((item) => item.status === 'ready')
  if (ready.length !== importQueue.value.length) return toast.error('请等待每个文件完成预检并处理错误')
  if (ready.some((item) => !item.selectedTargetId || !compatibleTargets(item).some((target) => target.targetId === item.selectedTargetId))) {
    return toast.error('每个文件必须明确选择一个兼容目标')
  }
  const normalizedPrice = String(acquisitionPrice.value ?? '').trim()
  importing.value = true
  let imported = 0
  const failures: string[] = []
  for (const item of ready) {
    item.status = 'committing'
    try {
      await api.commitSubscriptionImport({
        file: item.file,
        preflightToken: item.preflight!.preflightToken,
        targetId: item.selectedTargetId!,
        acquisitionPrice: ready.length === 1 ? normalizedPrice : undefined,
      })
      item.status = 'committed'
      imported += 1
    } catch (err) {
      item.status = 'error'
      item.error = getErrorMessage(err)
      failures.push(`${item.file.name}：${item.error}`)
    }
  }
  importing.value = false
  const message = `导入提交完成：成功 ${imported} · 失败 ${failures.length}`
  if (failures.length) toast.error(`${message}；${failures.slice(0, 2).join('；')}`)
  else toast.success(message)
  importQueue.value = importQueue.value.filter((item) => item.status !== 'committed')
  if (!importQueue.value.length) {
    acquisitionPrice.value = ''
    if (fileInput.value) fileInput.value.value = ''
  }
  page.value = 1
  await load()
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

async function deployToPool(item: Subscription, quiet = false) {
  deployingId.value = item.id
  try {
    const binding = await api.deploySubscription(item.id)
    bindings.value[String(item.id)] = [...itemBindings(item).filter((candidate) => candidate.targetId !== binding.targetId), binding]
    if (!quiet) toast.success(`已部署到 ${targetName(binding.targetId)}`)
    return true
  } catch (err) {
    if (!quiet) toast.error(getErrorMessage(err))
    return false
  } finally {
    deployingId.value = null
  }
}

async function detachFromPool(item: Subscription) {
  const binding = activeBinding(item)
  if (!binding || !window.confirm(`确认从 ${targetName(binding.targetId)} 撤销该账号？托管账号会从远端池删除，归档仍保留。`)) return
  deployingId.value = item.id
  try {
    const updated = await api.detachSubscription(item.id, binding.targetId)
    bindings.value[String(item.id)] = [...itemBindings(item).filter((candidate) => candidate.targetId !== updated.targetId), updated]
    toast.success('运行绑定已撤销，订阅归档保持不变')
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    deployingId.value = null
  }
}

async function migrateToPrimary(item: Subscription) {
	const binding = activeBinding(item)
	const target = primaryTarget.value
	if (!binding || !target || binding.targetId === target.id) return
	if (!window.confirm(`确认将该账号从 ${targetName(binding.targetId)} 迁移到 ${target.name}？失败时系统会尝试恢复原绑定。`)) return
	deployingId.value = item.id
	try {
		const updated = await api.migrateSubscription(item.id, binding.targetId, target.id)
		bindings.value[String(item.id)] = [...itemBindings(item).filter((candidate) => candidate.targetId !== binding.targetId && candidate.targetId !== updated.targetId), { ...binding, desiredState: 'detached', observedState: 'detached' }, updated]
		toast.success(`已切回 ${target.name}`)
	} catch (err) {
		toast.error(getErrorMessage(err))
	} finally {
		deployingId.value = null
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
    if (await deployToPool(item, true)) synced += 1
  }
  batchSyncing.value = false
  toast.show(`批量部署完成：${synced}/${items.length}`, synced === items.length ? 'success' : 'info')
  await load()
}

async function deleteSelected() {
  const items = [...selectedItems.value]
  if (!items.length) return
  if (!window.confirm(`确认删除选中的 ${items.length} 个订阅？Orbit 托管的 Sub2API/CPA 运行副本会先安全撤销；外部接管账号不会被远端删除。`)) return
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

async function loadPollStatus(quiet = false) {
  try {
    pollStatus.value = await api.getSubscriptionPollStatus()
    pollError.value = ''
  } catch (err) {
    if (!quiet) pollError.value = getErrorMessage(err)
  }
}

async function pollNow() {
  pollingNow.value = true
  try {
    await api.pollSubscriptionsNow()
    toast.success('账号状态与额度轮询已启动')
    await loadPollStatus()
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    pollingNow.value = false
  }
}

function pollStatusText() {
  if (!pollStatus.value) return pollError.value || '轮询状态不可用'
  if (pollStatus.value.running) return `运行中 ${pollStatus.value.completed}/${pollStatus.value.totalAccounts}`
  if (!pollStatus.value.enabled) return '定时轮询已关闭'
  return `每 ${pollStatus.value.intervalMinutes} 分钟 · 下次 ${formatDateTime(pollStatus.value.nextRunAt)}`
}

let searchTimer = 0
let pollTimer = 0
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

onMounted(() => {
  void load()
  void loadPollStatus()
  pollTimer = window.setInterval(() => {
    void loadPollStatus(true)
    if (!pollStatus.value?.running) return
    void load()
  }, 5000)
})
onBeforeUnmount(() => {
  window.clearTimeout(searchTimer)
  window.clearInterval(pollTimer)
})
</script>

<template>
  <div class="page-stack">
    <section class="panel import-panel">
      <div class="panel__header"><div><h2>导入订阅 JSON</h2><p>每个文件先由后端安全预检，再明确选择唯一兼容目标后提交。</p></div></div>
      <div class="import-layout">
        <div class="drop-zone" :class="{ 'drop-zone--active': dragging }" role="button" tabindex="0" @click="fileInput?.click()" @keydown.enter="fileInput?.click()" @keydown.space.prevent="fileInput?.click()" @dragover.prevent="dragging = true" @dragleave.prevent="dragging = false" @drop.prevent="onDrop">
          <CloudUpload :size="30" aria-hidden="true" />
          <strong>拖拽 JSON 文件到这里</strong>
          <span>或点击选择文件；凭据值不会显示在预检结果中</span>
          <input ref="fileInput" class="sr-only" type="file" accept="application/json,.json" multiple @change="addFiles(($event.target as HTMLInputElement).files || [])" />
        </div>
        <div v-if="importQueue.length === 1" class="import-fields">
          <label class="field"><span>入手价格 <small>可选</small></span><input v-model="acquisitionPrice" type="number" min="0" step="0.01" inputmode="decimal" placeholder="例如 12.00" /></label>
          <small class="import-field-hint">仅单文件提交时随 commit 请求发送；不影响兼容性判断。</small>
        </div>
      </div>
      <div class="import-tools import-tools--single" aria-label="订阅辅助工具">
        <a class="import-tool import-tool--cdk" href="https://www.kezongai.top/" target="_blank" rel="noopener noreferrer">
          <span class="import-tool__icon"><KeyRound :size="19" /></span>
          <span class="import-tool__copy"><strong>CDK 兑换入口</strong><small>打开兑换平台处理兑换码</small></span>
          <ExternalLink :size="15" aria-hidden="true" />
        </a>
      </div>
      <div v-if="importQueue.length" class="import-preflight-list" aria-label="导入预检结果">
        <article v-for="item in importQueue" :key="item.key" class="import-preflight-card">
          <header><span class="file-name"><FileJson2 :size="15" />{{ item.file.name }}</span><button type="button" :aria-label="`移除 ${item.file.name}`" :disabled="item.status === 'committing'" @click="removeImportItem(item.key)"><Trash2 :size="14" /></button></header>
          <p v-if="item.status === 'queued' || item.status === 'preflighting'" class="import-preflight-progress"><RefreshCw class="spinning" :size="15" />后端安全预检中…</p>
          <div v-else-if="item.preflight" class="import-analysis">
            <dl>
              <div><dt>格式</dt><dd>{{ item.preflight.analysis.format }}</dd></div>
              <div><dt>Provider</dt><dd>{{ item.preflight.analysis.identity.provider || 'unknown' }}</dd></div>
              <div><dt>掩码邮箱</dt><dd>{{ item.preflight.analysis.identity.email || '—' }}</dd></div>
              <div><dt>掩码账号</dt><dd>{{ item.preflight.analysis.identity.accountId || '—' }}</dd></div>
              <div class="import-analysis__wide"><dt>安全识别字段</dt><dd class="recognized-fields"><code v-for="field in item.preflight.analysis.identity.recognizedFields" :key="field">{{ field }}</code><span v-if="!item.preflight.analysis.identity.recognizedFields.length">无</span></dd></div>
            </dl>
            <div class="import-analysis-states"><StatusBadge :tone="analysisState(item, 'duplicate').tone" :label="`重复：${analysisState(item, 'duplicate').label}`" /><StatusBadge :tone="analysisState(item, 'conflict').tone" :label="`冲突：${analysisState(item, 'conflict').label}`" /></div>
            <fieldset class="target-options"><legend>选择唯一目标</legend>
              <label v-for="target in item.preflight.targets" :key="target.targetId" :class="{ 'is-disabled': !target.enabled || !target.compatible }">
                <input v-model="item.selectedTargetId" type="radio" :name="`target-${item.key}`" :value="target.targetId" :disabled="!target.enabled || !target.compatible" />
                <span><strong>{{ target.name }} · {{ target.kind === 'sub2api' ? 'Sub2API' : 'CPA' }}</strong><small><code>{{ target.reasonCode }}</code> · {{ target.enabled ? targetReason(target) : '目标已停用' }}</small></span>
              </label>
              <p v-if="!compatibleTargets(item).length" class="error-text">没有已启用且兼容的目标，请先在设置中配置网关。</p>
            </fieldset>
          </div>
          <p v-if="item.error" class="error-text">{{ item.error }}</p>
          <button v-if="item.status === 'error'" class="button button--secondary button--small" type="button" @click="preflightItem(item)"><RefreshCw :size="14" />重新预检</button>
        </article>
      </div>
      <div class="form-actions"><span class="muted">已选择 {{ importQueue.length }} 个文件</span><button class="button button--primary" type="button" :disabled="!importQueue.length || importing || importQueue.some((item) => item.status !== 'ready' || !item.selectedTargetId)" @click="commitImports"><CloudUpload :size="17" />{{ importing ? '提交中…' : '确认归档并部署' }}</button></div>
    </section>

    <section class="panel">
      <div class="panel__header panel__header--wrap">
        <div><h2>订阅列表</h2><p>共 {{ total }} 条 · 第 {{ page }}/{{ totalPages || 1 }} 页 · 每页 10 条 · 展示已持久化的检查结果</p></div>
        <div class="batch-actions">
          <span class="poll-status" :class="{ 'is-running': pollStatus?.running }"><RefreshCw :class="{ spinning: pollStatus?.running }" :size="14" />{{ pollStatusText() }}</span>
          <button class="button button--secondary button--small" type="button" :disabled="pollingNow || pollStatus?.running" @click="pollNow"><Play :size="15" />{{ pollStatus?.running ? '轮询中' : '立即轮询' }}</button>
          <span class="selection-count">已选 {{ selectedIds.size }} 条</span>
          <button class="button button--secondary button--small" type="button" :disabled="batchTesting || !selectedItems.length" @click="testBatch"><Play :size="15" />{{ batchTesting ? `检测 ${batchProgress.current}/${batchProgress.total}` : '批量检测' }}</button>
          <button class="button button--secondary button--small" type="button" :disabled="batchSyncing || !selectedItems.length" @click="syncSelected"><Network :size="15" />{{ batchSyncing ? '部署中' : '部署主号池' }}</button>
          <button class="button button--danger button--small" type="button" :disabled="batchDeleting || !selectedItems.length" @click="deleteSelected"><Trash2 :size="15" />{{ batchDeleting ? '删除中' : '删除所选' }}</button>
        </div>
      </div>
      <div class="subscription-insights" aria-label="订阅资产洞察">
        <div class="subscription-insight"><span>当前结果</span><strong>{{ total }}</strong><small>条订阅归档</small></div>
        <div class="subscription-insight subscription-insight--success"><span>账户健康</span><strong>{{ insights.normal }}</strong><small>{{ insights.pending }} 条待检测 · {{ insights.error }} 条异常</small></div>
        <div class="subscription-insight subscription-insight--accent"><span>已记录成本</span><strong>{{ formatCurrency(insights.totalCost) }}</strong><small>{{ insights.priced }} 条有价格</small></div>
        <div class="subscription-insight"><span>平均入手价</span><strong>{{ insights.priced ? formatCurrency(insights.averageCost) : '—' }}</strong><small>仅统计已标价订阅</small></div>
        <div class="subscription-insight" :class="{ 'subscription-insight--warning': insights.expiringSoon > 0 }"><span>即将到期</span><strong>{{ insights.expiringSoon }}</strong><small>剩余 7 天以内</small></div>
      </div>
      <div class="filters" aria-label="订阅筛选">
        <label class="search-field"><Search :size="17" /><span class="sr-only">搜索订阅</span><input v-model="search" type="search" placeholder="搜索邮箱、文件、account id" /></label>
        <label class="field field--compact"><span class="sr-only">状态筛选</span><select v-model="statusFilter"><option value="all">全部分类</option><option value="normal">正常（含 5H 额度耗尽）</option><option value="pending">待检测</option><option value="error">异常</option></select></label>
        <label class="field field--compact"><span class="sr-only">文件夹筛选</span><select v-model="folderFilter"><option value="all">全部文件夹</option><option v-for="folder in folders" :key="folder" :value="folder">{{ folder }}</option></select></label>
      </div>

      <LoadingState v-if="loading" label="正在加载订阅…" />
      <ErrorState v-else-if="error" :message="error" @retry="load" />
      <template v-else-if="subscriptions.length">
        <div class="table-wrap subscription-list-wrap">
        <table class="data-table data-table--wide subscription-table">
          <thead><tr><th class="selection-column"><input type="checkbox" :checked="allPageSelected" aria-label="全选本页" @change="togglePageSelection(($event.target as HTMLInputElement).checked)" /></th><th>邮箱</th><th>分类</th><th>运行池</th><th class="numeric">入手价</th><th class="numeric">延迟</th><th>5H 额度</th><th>7D 额度</th><th>订阅文件</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="item in subscriptions" :key="item.id" :class="{ 'row--selected': selectedIds.has(item.id) }">
              <td class="selection-column"><input type="checkbox" :checked="selectedIds.has(item.id)" :aria-label="`选择 ${item.email || subscriptionFile(item)}`" @change="toggleSelection(item.id, ($event.target as HTMLInputElement).checked)" /></td>
              <td class="strong subscription-email" data-label="邮箱" :title="item.email || ''">{{ item.email || '—' }}</td>
              <td data-label="分类"><div class="check-state"><StatusBadge :tone="connectivityState(item).tone" :label="connectivityState(item).label" /><small :class="`check-state--${subscriptionCheckFreshness(item)}`">{{ subscriptionCheckFreshness(item) === 'never' ? '从未检查' : subscriptionCheckFreshness(item) === 'stale' ? `结果过期 · ${formatDateTime(subscriptionCheckedAt(item))}` : formatDateTime(subscriptionCheckedAt(item)) }}</small></div></td>
              <td data-label="运行池"><span class="pool-binding" :class="{ 'is-active': activeBinding(item) }"><Network :size="13" />{{ targetName(activeBinding(item)?.targetId) }}</span></td>
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
          <div><dt>最后检查</dt><dd>{{ subscriptionCheckFreshness(selected) === 'never' ? '从未检查' : subscriptionCheckFreshness(selected) === 'stale' ? `${formatDateTime(subscriptionCheckedAt(selected))}（已过期）` : formatDateTime(subscriptionCheckedAt(selected)) }}</dd></div>
          <div><dt>调用延迟</dt><dd>{{ selected.connectivity?.latencyMs == null || !selected.connectivity?.httpStatus ? '—' : `${selected.connectivity.latencyMs} ms` }}</dd></div>
          <div><dt>CPA 状态</dt><dd>{{ selected.connectivity?.cpaStatusMessage || selected.connectivity?.cpaStatus || '—' }}</dd></div>
          <div><dt>主运行池</dt><dd>{{ targetName(activeBinding(selected)?.targetId) }}</dd></div>
          <div><dt>绑定归属</dt><dd>{{ activeBinding(selected)?.ownership === 'adopted' ? '外部接管（仅解绑）' : activeBinding(selected) ? 'Orbit 托管' : '未绑定' }}</dd></div>
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
		<button v-if="activeBinding(selected)?.mode === 'fallback' && primaryTarget && activeBinding(selected)?.targetId !== primaryTarget.id" class="button button--primary" type="button" :disabled="deployingId === selected.id" @click="migrateToPrimary(selected)"><Network :size="17" />切回主号池</button>
        <button v-if="activeBinding(selected)" class="button button--danger" type="button" :disabled="deployingId === selected.id" @click="detachFromPool(selected)"><Unplug :size="17" />撤销运行绑定</button>
        <button v-else class="button button--primary" type="button" :disabled="deployingId === selected.id" @click="deployToPool(selected)"><Network :size="17" />{{ deployingId === selected.id ? '部署中…' : '部署到主号池' }}</button>
        <button class="button button--secondary" type="button" :disabled="syncingId === selected.id" @click="syncToCpa(selected)"><FolderSync :size="17" />{{ syncingId === selected.id ? '同步中…' : '手动同步 CPA' }}</button>
      </template>
    </BaseDrawer>
  </div>
</template>
