<script setup lang="ts">
import { FolderCog, KeyRound, Save, Send, ShieldCheck } from 'lucide-vue-next'
import { onMounted, reactive, ref } from 'vue'
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

const loading = ref(true)
const saving = ref(false)
const testingWebhook = ref(false)
const error = ref('')
const toast = useToast()
const form = reactive({
  priceThreshold: 0,
  refreshInterval: 1,
  baseUrl: '',
  apiKey: '',
  webhookUrl: '',
  allowRemoteBaseUrl: false,
  cpaAuthDir: '',
  syncToCpaAuthDir: false,
})

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
  <div class="page-stack page-narrow">
    <LoadingState v-if="loading" label="正在加载设置…" />
    <ErrorState v-else-if="error" :message="error" @retry="load" />
    <form v-else class="page-stack" @submit.prevent="save">
      <section class="panel form-section">
        <div class="panel__header"><div><h2>监控设置</h2><p>控制低价阈值和后端刷新节奏。</p></div></div>
        <div class="form-grid">
          <label class="field"><span>低价提醒阈值（元）</span><input v-model.number="form.priceThreshold" type="number" min="0" step="0.01" required /></label>
          <label class="field"><span>刷新周期（分钟）</span><input v-model.number="form.refreshInterval" type="number" min="1" step="1" required /><small>单位为分钟，建议根据上游频率限制合理设置。</small></label>
        </div>
      </section>

      <section class="panel form-section">
        <div class="panel__header"><div><h2>连接设置</h2><p>用于订阅检查的默认地址与认证信息。</p></div></div>
        <div class="form-grid form-grid--single">
          <label class="field"><span>base_url</span><input v-model="form.baseUrl" type="url" placeholder="http://127.0.0.1:8317" /></label>
          <label class="field"><span>API key</span><div class="input-with-icon"><KeyRound :size="17" /><input v-model="form.apiKey" type="password" autocomplete="new-password" placeholder="留空表示不修改" /></div><small>API key 仅在保存时发送给后端，前端不会请求或回显已保存的值。</small></label>
          <label class="check-row"><input v-model="form.allowRemoteBaseUrl" class="switch" type="checkbox" /><span><strong>允许远端 base_url</strong><small>关闭时仅允许本机或内网策略范围内的地址；最终限制由后端执行。</small></span></label>
        </div>
      </section>

      <section class="panel form-section">
        <div class="panel__header"><div><h2>CLIProxyAPI 同步</h2><p>配置 CPA（CLIProxyAPI）的认证文件目录。</p></div><FolderCog :size="22" class="muted" /></div>
        <div class="form-grid form-grid--single">
          <label class="field"><span>CPA auth-dir</span><input v-model="form.cpaAuthDir" type="text" placeholder="例如：C:\path\to\auth-dir" /><small>CLIProxyAPI 会热加载 auth-dir 中新增或更新的订阅文件，无需重启服务。</small></label>
          <label class="check-row"><input v-model="form.syncToCpaAuthDir" class="switch" type="checkbox" /><span><strong>导入后同步到 CPA</strong><small>订阅 JSON 导入成功后，由后端复制到 CPA auth-dir，随后由 CLIProxyAPI 热加载。</small></span></label>
        </div>
      </section>

      <section class="panel form-section">
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
</template>
