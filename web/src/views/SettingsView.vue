<script setup lang="ts">
import { Activity, Bell, BellRing, Cable, FolderCog, KeyRound, ListTree, MonitorCog, Network, Plus, Save, Send, ServerCog, ShieldCheck, X } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AlertsView from './AlertsView.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import GatewayStatusCard from '../components/dashboard/GatewayStatusCard.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { GatewayOverview, Settings } from '../types/api'
import { getErrorMessage } from '../utils/format'
import {
  settingsAccountPollInterval,
  settingsAllowRemote,
  settingsBaseUrl,
  settingsCpaAuthDir,
  settingsPriceThreshold,
  settingsRefreshInterval,
  settingsSyncToCpa,
  settingsWebhookUrl,
} from '../utils/models'

type SettingsSection = 'monitor' | 'desktop' | 'connection' | 'gateways' | 'cpa' | 'webhook' | 'alerts'

const loading = ref(true)
const saving = ref(false)
const testingWebhook = ref(false)
const error = ref('')
const toast = useToast()
const route = useRoute()
const router = useRouter()
const settingsSections = [
  { id: 'monitor' as const, label: '监控设置', icon: Activity },
  { id: 'desktop' as const, label: '桌面体验', icon: MonitorCog },
  { id: 'connection' as const, label: '连接设置', icon: Cable },
  { id: 'gateways' as const, label: '网关配置', icon: Network },
  { id: 'cpa' as const, label: 'CPA 同步', icon: FolderCog },
  { id: 'webhook' as const, label: 'Webhook', icon: BellRing },
  { id: 'alerts' as const, label: '提醒中心', icon: Bell },
]
const activeSection = computed<SettingsSection>(() => {
  const requested = String(route.query.section ?? 'monitor')
  return settingsSections.some((item) => item.id === requested) ? requested as SettingsSection : 'monitor'
})
const form = reactive({
  priceThreshold: 0,
  refreshInterval: 1,
  accountPollEnabled: true,
  accountPollMinutes: 5,
  baseUrl: '',
  apiKey: '',
  webhookUrl: '',
  allowRemoteBaseUrl: false,
  cpaAuthDir: '',
  syncToCpaAuthDir: false,
  themeMode: 'auto' as 'light' | 'dark' | 'auto',
  startOnLogin: false,
  closeToTray: true,
  desktopNotifications: true,
  flashOnAlert: true,
})

const gatewayOverview = ref<GatewayOverview | null>(null)
const gatewayLoading = ref(false)
const gatewayError = ref('')
const testingGatewayId = ref<number | null>(null)
const savingGateway = ref(false)
const gatewayFormOpen = ref(false)
const gatewayValidationError = ref('')
const gatewayForm = reactive({ id: 0, kind: 'sub2api' as 'sub2api' | 'cpa', name: 'Primary Sub2API', baseUrl: 'http://127.0.0.1:8080', adminKey: '', enabled: true, primary: true, allowRemote: false, groupIds: '', concurrency: 2, priority: 0, rateMultiplier: 1 })

function setSection(section: SettingsSection) {
  void router.replace({ path: '/settings', query: section === 'monitor' ? {} : { section } })
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await api.getSettings()
    form.priceThreshold = settingsPriceThreshold(data)
    form.refreshInterval = settingsRefreshInterval(data)
    const accountPollMinutes = settingsAccountPollInterval(data)
    form.accountPollEnabled = accountPollMinutes > 0
    form.accountPollMinutes = accountPollMinutes > 0 ? accountPollMinutes : 5
    form.baseUrl = settingsBaseUrl(data)
    form.webhookUrl = settingsWebhookUrl(data)
    form.allowRemoteBaseUrl = settingsAllowRemote(data)
    form.cpaAuthDir = settingsCpaAuthDir(data)
    form.syncToCpaAuthDir = settingsSyncToCpa(data)
    form.themeMode = data.themeMode === 'light' || data.themeMode === 'dark' ? data.themeMode : 'auto'
    form.startOnLogin = data.startOnLogin === true
    form.closeToTray = data.closeToTray !== false
    form.desktopNotifications = data.desktopNotifications !== false
    form.flashOnAlert = data.flashOnAlert !== false
    form.apiKey = ''
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  const payload: Settings = {
    threshold: Number(form.priceThreshold),
    refreshMinutes: Number(form.refreshInterval),
    accountPollMinutes: form.accountPollEnabled ? Number(form.accountPollMinutes) : 0,
    baseUrl: form.baseUrl.trim(),
    webhookUrl: form.webhookUrl.trim(),
    allowRemoteBaseUrl: form.allowRemoteBaseUrl,
    cpaAuthDir: form.cpaAuthDir.trim(),
    syncToCpaAuthDir: form.syncToCpaAuthDir,
    themeMode: form.themeMode,
    startOnLogin: form.startOnLogin,
    closeToTray: form.closeToTray,
    desktopNotifications: form.desktopNotifications,
    flashOnAlert: form.flashOnAlert,
  }
  if (form.apiKey.trim()) payload.apiKey = form.apiKey.trim()
  try {
    const result = await api.updateSettings(payload)
    toast.success(typeof result.message === 'string' ? result.message : '设置已保存')
    form.apiKey = ''
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    saving.value = false
  }
}

async function testWebhook() {
  testingWebhook.value = true
  try {
    const result = await api.testWebhook(form.webhookUrl.trim())
    toast.success(result.message || 'Webhook 测试消息已发送')
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    testingWebhook.value = false
  }
}

async function loadGateways() {
  gatewayLoading.value = true
  gatewayError.value = ''
  try {
    gatewayOverview.value = await api.getGatewayOverview()
  } catch (err) {
    gatewayError.value = getErrorMessage(err)
  } finally {
    gatewayLoading.value = false
  }
}

function newGatewayTarget() {
  Object.assign(gatewayForm, { id: 0, kind: 'sub2api', name: 'Primary Sub2API', baseUrl: 'http://127.0.0.1:8080', adminKey: '', enabled: true, primary: true, allowRemote: false, groupIds: '', concurrency: 2, priority: 0, rateMultiplier: 1 })
  gatewayValidationError.value = ''
  gatewayFormOpen.value = true
}

function editGatewayTarget(id: number) {
  const target = gatewayOverview.value?.targets.find((item) => item.target.id === id)?.target
  if (!target) return
  Object.assign(gatewayForm, { id: target.id, kind: target.kind, name: target.name, baseUrl: target.baseUrl, adminKey: '', enabled: target.enabled, primary: target.primary, allowRemote: target.allowRemote, groupIds: (target.defaultGroupIds || []).join(', '), concurrency: target.defaultConcurrency || 1, priority: target.defaultPriority || 0, rateMultiplier: target.rateMultiplier || 1 })
  gatewayValidationError.value = ''
  gatewayFormOpen.value = true
}

function parseGatewayGroupIds() {
  const values = gatewayForm.groupIds.split(',').map((value) => value.trim()).filter(Boolean)
  const groupIds = values.map(Number)
  return groupIds.every((value) => Number.isInteger(value) && value > 0) ? groupIds : null
}

function validateGatewayTarget() {
  if (!gatewayForm.name.trim()) return '请输入显示名称。'
  let url: URL
  try {
    url = new URL(gatewayForm.baseUrl.trim())
  } catch {
    return '请输入有效的网关管理地址。'
  }
  if (!['http:', 'https:'].includes(url.protocol)) return '网关管理地址仅支持 HTTP 或 HTTPS。'
  const isLoopback = ['127.0.0.1', 'localhost', '::1', '[::1]'].includes(url.hostname)
  if (!isLoopback && !gatewayForm.allowRemote) return '远程地址需要开启“允许远程地址”。'
  if (!isLoopback && url.protocol !== 'https:') return '远程网关必须使用 HTTPS。'
  if (gatewayForm.kind === 'sub2api' && !gatewayForm.id && !gatewayForm.adminKey.trim()) return '新增 Sub2API 时请输入管理密钥。'
  if (parseGatewayGroupIds() === null) return '默认分组 ID 必须是以逗号分隔的正整数。'
  if (!Number.isInteger(Number(gatewayForm.concurrency)) || Number(gatewayForm.concurrency) < 1 || Number(gatewayForm.concurrency) > 1000) return '账号并发必须是 1 到 1000 的整数。'
  if (!Number.isInteger(Number(gatewayForm.priority)) || Number(gatewayForm.priority) < -1000 || Number(gatewayForm.priority) > 1000) return '调度优先级必须是 -1000 到 1000 的整数。'
  if (!Number.isFinite(Number(gatewayForm.rateMultiplier)) || Number(gatewayForm.rateMultiplier) < 0.01 || Number(gatewayForm.rateMultiplier) > 1000) return '成本倍率必须在 0.01 到 1000 之间。'
  return ''
}

async function saveGatewayTarget() {
  gatewayValidationError.value = validateGatewayTarget()
  if (gatewayValidationError.value) return
  savingGateway.value = true
  try {
    await api.saveGatewayTarget({
      id: gatewayForm.id || undefined,
      kind: gatewayForm.kind,
      name: gatewayForm.name.trim(),
      baseUrl: gatewayForm.baseUrl.trim(),
      adminKey: gatewayForm.adminKey.trim() || undefined,
      enabled: gatewayForm.enabled,
      primary: gatewayForm.primary,
      allowRemote: gatewayForm.allowRemote,
      defaultGroupIds: parseGatewayGroupIds() || [],
      defaultConcurrency: Number(gatewayForm.concurrency),
      defaultPriority: Number(gatewayForm.priority),
      rateMultiplier: Number(gatewayForm.rateMultiplier),
    })
    toast.success('网关配置已保存，管理密钥不会回显')
    gatewayForm.adminKey = ''
    gatewayFormOpen.value = false
    await loadGateways()
  } catch (err) {
    gatewayValidationError.value = getErrorMessage(err)
  } finally {
    savingGateway.value = false
  }
}

async function testGatewayTarget(id: number) {
  testingGatewayId.value = id
  try {
    const result = await api.testGatewayTarget(id)
    result.status === 'ok' ? toast.success(`连接正常 · ${result.latencyMs || 0} ms`) : toast.error(result.message || '网关连接异常')
    await loadGateways()
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    testingGatewayId.value = null
  }
}

watch(activeSection, (section) => {
  if (section === 'gateways' && !gatewayOverview.value && !gatewayLoading.value) void loadGateways()
}, { immediate: true })
onMounted(load)
</script>

<template>
  <div class="page-stack">
    <LoadingState v-if="loading" label="正在加载设置…" />
    <ErrorState v-else-if="error" :message="error" @retry="load" />
    <div v-else class="settings-layout">
      <aside class="settings-rail">
        <nav class="panel settings-nav" aria-label="设置二级导航">
          <div class="settings-nav__title"><ListTree :size="16" /><span>设置目录</span></div>
          <button v-for="item in settingsSections" :key="item.id" type="button" :class="{ 'is-active': activeSection === item.id }" @click="setSection(item.id)"><component :is="item.icon" :size="15" /><span>{{ item.label }}</span></button>
        </nav>
        <p class="settings-rail__hint">每个栏目使用独立页面状态；设置由网页端与 APP 共享。</p>
      </aside>

      <AlertsView v-if="activeSection === 'alerts'" class="settings-content" />

      <div v-else-if="activeSection === 'gateways'" class="page-stack settings-content gateway-settings">
        <section class="panel settings-section gateway-settings__intro">
          <div class="panel__header panel__header--wrap">
            <div><h2>网关配置</h2><p>维护 Sub2API 主号池与 CPA 回退目标。目标、凭据状态和健康记录继续保存在现有网关存储中。</p></div>
            <button class="button button--primary" type="button" @click="newGatewayTarget"><Plus :size="16" />添加网关</button>
          </div>
        </section>
        <LoadingState v-if="gatewayLoading && !gatewayOverview" label="正在加载网关配置…" />
        <ErrorState v-else-if="gatewayError && !gatewayOverview" :message="gatewayError" @retry="loadGateways" />
        <section v-else class="gateway-rack gateway-rack--settings" aria-label="网关目标列表">
          <div class="section-heading"><div><span class="eyebrow">GATEWAY TARGETS</span><h2>主备网关</h2></div><span>{{ gatewayOverview?.targets.length || 0 }} 个目标</span></div>
          <div v-if="gatewayOverview?.targets.length" class="gateway-grid"><GatewayStatusCard v-for="item in gatewayOverview.targets" :key="item.target.id" :status="item" :testing="testingGatewayId === item.target.id" @test="testGatewayTarget" @edit="editGatewayTarget" /></div>
          <div v-else class="operations-empty"><ServerCog :size="28" /><strong>尚未配置网关</strong><span>添加 Sub2API 或 CPA 目标后，可在这里编辑配置并检查连接健康。</span></div>
          <div v-if="gatewayError" class="inline-alert inline-alert--warning gateway-settings__warning"><span>{{ gatewayError }}</span><button class="button button--ghost button--small" type="button" @click="loadGateways">重试</button></div>
        </section>
      </div>

      <form v-else class="page-stack settings-content" @submit.prevent="save">
        <section v-if="activeSection === 'monitor'" class="panel form-section settings-section">
          <div class="panel__header"><div><h2>监控设置</h2><p>控制低价阈值和后端刷新节奏。</p></div></div>
          <div class="form-grid">
            <label class="field"><span>低价提醒阈值（元）</span><input v-model.number="form.priceThreshold" type="number" min="0" step="0.01" required /></label>
            <label class="field"><span>行情刷新周期（分钟）</span><input v-model.number="form.refreshInterval" type="number" min="1" max="1440" step="1" required /><small>仅控制价格行情，不影响账号状态或额度。</small></label>
            <label class="check-row form-grid__wide"><input v-model="form.accountPollEnabled" class="switch" type="checkbox" /><span><strong>自动轮询账号状态与额度</strong><small>默认每 5 分钟检查一次；关闭后仍可手动立即轮询。</small></span></label>
            <label v-if="form.accountPollEnabled" class="field"><span>账号轮询周期（分钟）</span><input v-model.number="form.accountPollMinutes" type="number" min="5" max="1440" step="1" required /><small>允许范围为 5–1440 分钟。</small></label>
          </div>
        </section>

        <section v-else-if="activeSection === 'desktop'" class="panel form-section settings-section">
          <div class="panel__header"><div><h2>桌面体验</h2><p>网页端与 APP 共用这组偏好；桌面行为只在 APP 中生效。</p></div><MonitorCog :size="22" class="muted" /></div>
          <div class="form-grid form-grid--single">
            <label class="field"><span>主题模式</span><select v-model="form.themeMode"><option value="auto">Auto · 跟随系统</option><option value="light">Light · 始终亮色</option><option value="dark">Dark · 始终暗色</option></select></label>
            <label class="check-row"><input v-model="form.startOnLogin" class="switch" type="checkbox" /><span><strong>开机自动启动 APP</strong><small>仅写入当前 Windows 用户的启动项。</small></span></label>
            <label class="check-row"><input v-model="form.closeToTray" class="switch" type="checkbox" /><span><strong>关闭窗口时缩小到系统托盘</strong><small>从托盘菜单选择退出才会真正退出。</small></span></label>
            <label class="check-row"><input v-model="form.desktopNotifications" class="switch" type="checkbox" /><span><strong>桌面通知</strong><small>发现低价提醒时发送系统通知。</small></span></label>
            <label class="check-row"><input v-model="form.flashOnAlert" class="switch" type="checkbox" /><span><strong>提醒时闪动窗口</strong><small>窗口在后台时用任务栏闪动提示。</small></span></label>
          </div>
        </section>

        <section v-else-if="activeSection === 'connection'" class="panel form-section settings-section">
          <div class="panel__header"><div><h2>连接设置</h2><p>用于订阅检查的默认地址与认证信息。</p></div></div>
          <div class="form-grid form-grid--single">
            <label class="field"><span>base_url</span><input v-model="form.baseUrl" type="url" placeholder="http://127.0.0.1:8317" /></label>
            <label class="field"><span>API key</span><div class="input-with-icon"><KeyRound :size="17" /><input v-model="form.apiKey" type="password" autocomplete="new-password" placeholder="留空表示不修改" /></div><small>API key 仅在保存时发送给后端，前端不会回显已保存的值。</small></label>
            <label class="check-row"><input v-model="form.allowRemoteBaseUrl" class="switch" type="checkbox" /><span><strong>允许远端 base_url</strong><small>最终安全限制由后端执行。</small></span></label>
          </div>
        </section>

        <section v-else-if="activeSection === 'cpa'" class="panel form-section settings-section">
          <div class="panel__header"><div><h2>CLIProxyAPI 同步</h2><p>配置 CPA 的认证文件目录。</p></div><FolderCog :size="22" class="muted" /></div>
          <div class="form-grid form-grid--single">
            <label class="field"><span>CPA auth-dir</span><input v-model="form.cpaAuthDir" type="text" placeholder="例如：C:\path\to\auth-dir" /></label>
            <label class="check-row"><input v-model="form.syncToCpaAuthDir" class="switch" type="checkbox" /><span><strong>导入后同步到 CPA</strong><small>订阅 JSON 导入成功后复制到 CPA auth-dir。</small></span></label>
          </div>
        </section>

        <section v-else class="panel form-section settings-section">
          <div class="panel__header"><div><h2>Webhook</h2><p>低价和异常提醒可推送到外部系统。</p></div></div>
          <div class="webhook-row">
            <label class="field"><span>Webhook URL</span><input v-model="form.webhookUrl" type="url" placeholder="https://hooks.example.com/…" /></label>
            <button class="button button--secondary" type="button" :disabled="testingWebhook || !form.webhookUrl" @click="testWebhook"><Send :size="16" />{{ testingWebhook ? '测试中…' : '测试 Webhook' }}</button>
          </div>
        </section>

        <div class="sticky-actions">
          <div class="security-summary"><ShieldCheck :size="17" /><span>敏感配置由后端持久化</span></div>
          <button class="button button--primary" type="submit" :disabled="saving"><Save :size="17" />{{ saving ? '保存中…' : '保存设置' }}</button>
        </div>
      </form>
    </div>

    <div v-if="gatewayFormOpen" class="gateway-modal" role="dialog" aria-modal="true" aria-label="配置网关">
      <button class="gateway-modal__backdrop" type="button" aria-label="关闭" @click="gatewayFormOpen = false" />
      <form class="gateway-form" @submit.prevent="saveGatewayTarget">
        <div class="gateway-form__head"><div><span class="eyebrow">TARGET PROFILE</span><h2>{{ gatewayForm.id ? '编辑网关' : '添加网关' }}</h2></div><button class="icon-button" type="button" aria-label="关闭" @click="gatewayFormOpen = false"><X :size="19" /></button></div>
        <div v-if="gatewayValidationError" class="inline-alert inline-alert--warning gateway-form__validation" role="alert">{{ gatewayValidationError }}</div>
        <div class="form-grid">
          <label class="field"><span>网关类型</span><select v-model="gatewayForm.kind" :disabled="gatewayForm.id > 0"><option value="sub2api">Sub2API · 主号池</option><option value="cpa">CPA · 轻量备份</option></select></label>
          <label class="field"><span>显示名称</span><input v-model="gatewayForm.name" required /></label>
          <label class="field field--wide"><span>网关管理地址</span><input v-model="gatewayForm.baseUrl" type="url" required placeholder="http://127.0.0.1:8080" /><small>这是目标网关自己的管理入口；远程地址需显式允许并使用 HTTPS。</small></label>
          <label v-if="gatewayForm.kind === 'sub2api'" class="field field--wide"><span>管理密钥 <small>{{ gatewayForm.id ? '留空保持原密钥' : '写入后不再显示' }}</small></span><input v-model="gatewayForm.adminKey" type="password" autocomplete="new-password" :required="!gatewayForm.id" /></label>
          <div v-else class="field field--wide gateway-form__legacy-note"><span>CPA 连接沿用“设置 → CPA 同步”中的管理密钥与授权目录；这里仅控制主备角色和启停。</span></div>
          <label class="field"><span>默认分组 ID</span><input v-model="gatewayForm.groupIds" placeholder="3, 5" /><small>可留空，或填写以逗号分隔的正整数。</small></label>
          <label class="field"><span>账号并发</span><input v-model.number="gatewayForm.concurrency" type="number" min="1" max="1000" step="1" required /></label>
          <label class="field"><span>调度优先级</span><input v-model.number="gatewayForm.priority" type="number" min="-1000" max="1000" step="1" required /></label>
          <label class="field"><span>成本倍率</span><input v-model.number="gatewayForm.rateMultiplier" type="number" min="0.01" max="1000" step="0.01" required /></label>
        </div>
        <div class="gateway-form__switches"><label class="check-row"><input v-model="gatewayForm.enabled" class="switch" type="checkbox" /><span><strong>启用目标</strong><small>允许部署和健康检查。</small></span></label><label class="check-row"><input v-model="gatewayForm.primary" class="switch" type="checkbox" /><span><strong>设为主网关</strong><small>系统只保留一个主目标。</small></span></label><label class="check-row"><input v-model="gatewayForm.allowRemote" class="switch" type="checkbox" /><span><strong>允许远程地址</strong><small>仅 HTTPS，密钥随管理请求发送。</small></span></label></div>
        <div class="form-actions"><span class="security-note">管理密钥仅写入，不会从后端回显。</span><button class="button button--primary" type="submit" :disabled="savingGateway"><Save :size="16" />{{ savingGateway ? '保存中…' : '保存目标' }}</button></div>
      </form>
    </div>
  </div>
</template>
