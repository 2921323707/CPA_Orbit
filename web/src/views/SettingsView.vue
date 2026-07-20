<script setup lang="ts">
import { Activity, Bell, BellRing, Cable, FolderCog, KeyRound, ListTree, MonitorCog, Save, Send, ShieldCheck } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AlertsView from './AlertsView.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { Settings } from '../types/api'
import { getErrorMessage } from '../utils/format'
import {
  settingsAllowRemote,
  settingsBaseUrl,
  settingsCpaAuthDir,
  settingsPriceThreshold,
  settingsRefreshInterval,
  settingsSyncToCpa,
  settingsWebhookUrl,
} from '../utils/models'

type SettingsSection = 'monitor' | 'desktop' | 'connection' | 'cpa' | 'webhook' | 'alerts'

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

      <form v-else class="page-stack settings-content" @submit.prevent="save">
        <section v-if="activeSection === 'monitor'" class="panel form-section settings-section">
          <div class="panel__header"><div><h2>监控设置</h2><p>控制低价阈值和后端刷新节奏。</p></div></div>
          <div class="form-grid">
            <label class="field"><span>低价提醒阈值（元）</span><input v-model.number="form.priceThreshold" type="number" min="0" step="0.01" required /></label>
            <label class="field"><span>刷新周期（分钟）</span><input v-model.number="form.refreshInterval" type="number" min="1" step="1" required /><small>单位为分钟，建议根据上游频率限制合理设置。</small></label>
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
  </div>
</template>
