<script setup lang="ts">
import { Activity, AlertTriangle, BarChart3, Coins, DatabaseZap, Plus, RefreshCw, Route, Save, ServerCog, X } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import ErrorState from '../components/common/ErrorState.vue'
import GatewayStatusCard from '../components/dashboard/GatewayStatusCard.vue'
import TokenUsageChart from '../components/dashboard/TokenUsageChart.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { GatewayOverview, GatewayTarget, UsageBucket } from '../types/api'
import { formatDateTime, getErrorMessage } from '../utils/format'

const overview = ref<GatewayOverview | null>(null)
const buckets = ref<UsageBucket[]>([])
const loading = ref(true)
const error = ref('')
const testingId = ref<number | null>(null)
const saving = ref(false)
const formOpen = ref(false)
const toast = useToast()
const form = reactive({ id: 0, kind: 'sub2api' as 'sub2api' | 'cpa', name: 'Primary Sub2API', baseUrl: 'http://127.0.0.1:8080', adminKey: '', enabled: true, primary: true, allowRemote: false, groupIds: '', concurrency: 2, priority: 0, rateMultiplier: 1 })

const primarySub2API = computed(() => overview.value?.targets.find((item) => item.target.kind === 'sub2api' && item.target.primary) || overview.value?.targets.find((item) => item.target.kind === 'sub2api'))
const snapshot = computed(() => overview.value?.snapshots.find((item) => item.targetId === primarySub2API.value?.target.id))
function metric(...keys: string[]) {
	const nested = snapshot.value?.data?.stats
	const stats = nested && typeof nested === 'object' ? nested as Record<string, unknown> : undefined
  for (const key of keys) {
		const value = stats?.[key] ?? snapshot.value?.data?.[key]
    if (typeof value === 'number') return value
  }
  return 0
}
const totalTokens = computed(() => metric('today_tokens', 'total_tokens'))
const requests = computed(() => metric('today_requests', 'total_requests'))
const cost = computed(() => metric('today_actual_cost', 'total_actual_cost', 'today_cost', 'total_cost'))
const activeBindings = computed(() => overview.value?.bindings.filter((item) => item.observedState === 'active').length || 0)
const failedOperations = computed(() => overview.value?.operations.filter((item) => item.status === 'failed').length || 0)

async function load() {
  loading.value = true
  error.value = ''
  try {
    overview.value = await api.getGatewayOverview()
    if (primarySub2API.value) {
      buckets.value = (await api.getGatewayUsage(primarySub2API.value.target.id, 7)).buckets
    } else buckets.value = []
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function refreshTelemetry() {
	loading.value = true
	try {
		await api.collectGatewayTelemetry()
		toast.success('最新用量与运行快照已采集')
	} catch (err) {
		toast.show(`${getErrorMessage(err)}；正在显示最近一次有效快照`, 'info')
	} finally {
		await load()
	}
}

function newTarget() {
  Object.assign(form, { id: 0, kind: 'sub2api', name: 'Primary Sub2API', baseUrl: 'http://127.0.0.1:8080', adminKey: '', enabled: true, primary: true, allowRemote: false, groupIds: '', concurrency: 2, priority: 0, rateMultiplier: 1 })
  formOpen.value = true
}

function editTarget(id: number) {
  const target = overview.value?.targets.find((item) => item.target.id === id)?.target
  if (!target) return
  Object.assign(form, { id: target.id, kind: target.kind, name: target.name, baseUrl: target.baseUrl, adminKey: '', enabled: target.enabled, primary: target.primary, allowRemote: target.allowRemote, groupIds: (target.defaultGroupIds || []).join(','), concurrency: target.defaultConcurrency || 1, priority: target.defaultPriority || 0, rateMultiplier: target.rateMultiplier || 1 })
  formOpen.value = true
}

async function saveTarget() {
  saving.value = true
  try {
    const defaultGroupIds = form.groupIds.split(',').map((value) => Number(value.trim())).filter((value) => Number.isInteger(value) && value > 0)
    await api.saveGatewayTarget({ id: form.id || undefined, kind: form.kind, name: form.name.trim(), baseUrl: form.baseUrl.trim(), adminKey: form.adminKey.trim() || undefined, enabled: form.enabled, primary: form.primary, allowRemote: form.allowRemote, defaultGroupIds, defaultConcurrency: Number(form.concurrency), defaultPriority: Number(form.priority), rateMultiplier: Number(form.rateMultiplier) })
    toast.success('网关配置已保存，管理密钥不会回显')
    form.adminKey = ''
    formOpen.value = false
    await load()
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    saving.value = false
  }
}

async function testTarget(id: number) {
  testingId.value = id
  try {
    const result = await api.testGatewayTarget(id)
    result.status === 'ok' ? toast.success(`连接正常 · ${result.latencyMs || 0} ms`) : toast.error(result.message || '网关连接异常')
    await load()
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    testingId.value = null
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack operations-page">
    <section class="operations-hero">
      <div><span class="eyebrow">GATEWAY CONTROL PLANE</span><h1>订阅号池运行台</h1><p>Sub2API 承担主调度与 Token 明细，CPA 保留为轻量故障回退。Orbit 负责资产、绑定与可恢复操作。</p></div>
		<div class="operations-hero__actions"><button class="button button--secondary" type="button" :disabled="loading" @click="refreshTelemetry"><RefreshCw :size="16" />{{ loading ? '采集中…' : '刷新用量' }}</button><button class="button button--primary" type="button" @click="newTarget"><Plus :size="16" />添加 Sub2API</button></div>
    </section>

    <ErrorState v-if="error" :message="error" @retry="load" />
    <template v-else>
      <section class="operations-kpis" aria-label="运行指标">
        <article><Route :size="19" /><span>活动绑定</span><strong>{{ activeBindings }}</strong><small>主备关系已持久化</small></article>
        <article><DatabaseZap :size="19" /><span>今日 Token</span><strong>{{ totalTokens.toLocaleString() }}</strong><small>输入、输出与缓存合计</small></article>
        <article><Activity :size="19" /><span>今日请求</span><strong>{{ requests.toLocaleString() }}</strong><small>来自 Sub2API 快照</small></article>
        <article><Coins :size="19" /><span>实际成本</span><strong>${{ cost.toFixed(4) }}</strong><small>{{ snapshot?.stale ? '快照已陈旧' : '最近有效快照' }}</small></article>
      </section>

      <section class="gateway-rack">
        <div class="section-heading"><div><span class="eyebrow">ROUTE RACK</span><h2>主备网关</h2></div><span>{{ overview?.targets.length || 0 }} 个目标</span></div>
        <div v-if="overview?.targets.length" class="gateway-grid"><GatewayStatusCard v-for="item in overview.targets" :key="item.target.id" :status="item" :testing="testingId === item.target.id" @test="testTarget" @edit="editTarget" /></div>
        <div v-else class="operations-empty"><ServerCog :size="28" /><strong>尚未配置运行网关</strong><span>添加 Sub2API 后即可把 GPT Plus/Codex 订阅转成可调度号池。</span></div>
      </section>

      <section class="operations-grid">
        <article class="panel operations-chart-panel">
          <div class="panel__header"><div><h2>Token 流量</h2><p>本地保留 15 分钟聚合，最长 90 天；明细仍以 Sub2API 为准。</p></div><BarChart3 :size="21" /></div>
          <TokenUsageChart :buckets="buckets" />
        </article>
        <article class="panel operations-log-panel">
          <div class="panel__header"><div><h2>同步操作</h2><p>部署、撤销与迁移的最近执行记录。</p></div><span v-if="failedOperations" class="operation-alert"><AlertTriangle :size="14" />{{ failedOperations }} 异常</span></div>
          <div class="operation-list">
            <div v-for="operation in overview?.operations" :key="operation.id" class="operation-row"><span class="operation-row__state" :class="`is-${operation.status}`" /><div><strong>{{ operation.kind === 'deploy' ? '部署订阅' : '撤销绑定' }}</strong><small>{{ operation.subscriptionId }} · Target {{ operation.targetId }}</small></div><div><span>{{ operation.status }}</span><time>{{ formatDateTime(operation.completedAt || operation.createdAt) }}</time></div></div>
            <div v-if="!overview?.operations.length" class="operations-empty operations-empty--small">暂无同步操作</div>
          </div>
        </article>
      </section>
    </template>

    <div v-if="formOpen" class="gateway-modal" role="dialog" aria-modal="true" aria-label="配置网关">
      <button class="gateway-modal__backdrop" type="button" aria-label="关闭" @click="formOpen = false" />
      <form class="gateway-form" @submit.prevent="saveTarget">
        <div class="gateway-form__head"><div><span class="eyebrow">TARGET PROFILE</span><h2>{{ form.id ? '编辑网关' : '添加网关' }}</h2></div><button class="icon-button" type="button" aria-label="关闭" @click="formOpen = false"><X :size="19" /></button></div>
        <div class="form-grid">
			<label class="field"><span>网关类型</span><select v-model="form.kind" :disabled="form.id > 0"><option value="sub2api">Sub2API · 主号池</option><option value="cpa">CPA · 轻量备份</option></select></label>
          <label class="field"><span>显示名称</span><input v-model="form.name" required /></label>
          <label class="field field--wide"><span>管理地址</span><input v-model="form.baseUrl" type="url" required placeholder="http://127.0.0.1:8080" /><small>远程地址必须显式允许并使用 HTTPS。</small></label>
			<label v-if="form.kind === 'sub2api'" class="field field--wide"><span>管理密钥 <small>{{ form.id ? '留空保持原密钥' : '写入后不再显示' }}</small></span><input v-model="form.adminKey" type="password" autocomplete="new-password" /></label>
			<div v-else class="field field--wide gateway-form__legacy-note"><span>CPA 连接沿用“设置 → CPA 同步”中的管理密钥与授权目录；这里仅控制主备角色和启停。</span></div>
          <label class="field"><span>默认分组 ID</span><input v-model="form.groupIds" placeholder="3, 5" /></label>
          <label class="field"><span>账号并发</span><input v-model.number="form.concurrency" type="number" min="1" max="1000" /></label>
          <label class="field"><span>调度优先级</span><input v-model.number="form.priority" type="number" min="-1000" max="1000" /></label>
          <label class="field"><span>成本倍率</span><input v-model.number="form.rateMultiplier" type="number" min="0.01" max="1000" step="0.01" /></label>
        </div>
        <div class="gateway-form__switches"><label class="check-row"><input v-model="form.enabled" class="switch" type="checkbox" /><span><strong>启用目标</strong><small>允许部署和采集。</small></span></label><label class="check-row"><input v-model="form.primary" class="switch" type="checkbox" /><span><strong>设为主网关</strong><small>系统只保留一个主目标。</small></span></label><label class="check-row"><input v-model="form.allowRemote" class="switch" type="checkbox" /><span><strong>允许远程地址</strong><small>仅 HTTPS，密钥随管理请求发送。</small></span></label></div>
        <div class="form-actions"><span class="security-note">凭据正文不会出现在运维日志中。</span><button class="button button--primary" type="submit" :disabled="saving"><Save :size="16" />{{ saving ? '保存中…' : '保存目标' }}</button></div>
      </form>
    </div>
  </div>
</template>
