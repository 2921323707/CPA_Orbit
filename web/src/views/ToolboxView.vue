<script setup lang="ts">
import { Check, Copy, ExternalLink, Globe2, KeyRound, Phone, RefreshCw, RotateCw, WalletCards, XCircle } from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { LubanBalance, LubanCountry, LubanNumberSession, LubanService, LubanSmsStatus } from '../types/api'
import { formatDateTime, getErrorMessage } from '../utils/format'

const luban = ref<LubanBalance | null>(null)
const lubanError = ref('')
const lubanKey = ref('')
const savingKey = ref(false)
const countries = ref<LubanCountry[]>([])
const services = ref<LubanService[]>([])
const selectedCountryCode = ref('')
const serviceSearch = ref('')
const selectedServiceId = ref('')
const catalogLoading = ref(false)
const catalogError = ref('')
const numberLoading = ref(false)
const numberSession = ref<LubanNumberSession | null>(null)
const smsStatus = ref<LubanSmsStatus | null>(null)
const pollingError = ref('')
const toast = useToast()
let smsTimer: number | undefined

const selectedCountry = computed(() => countries.value.find((country) => country.code === selectedCountryCode.value))
const sortedServices = computed(() => [...services.value].sort((a, b) => Number(a.cost ?? 0) - Number(b.cost ?? 0)))

async function loadLubanBalance() {
  lubanError.value = ''
  try {
    luban.value = await api.getLubanBalance()
  } catch (err) {
    lubanError.value = getErrorMessage(err)
    const settings = await api.getSettings().catch(() => ({}))
    luban.value = { configured: Boolean('lubanApiKeyConfigured' in settings && settings.lubanApiKeyConfigured) }
  }
}

async function loadLubanCatalog() {
  catalogLoading.value = true
  catalogError.value = ''
  try {
    const result = await api.getLubanCountries()
    countries.value = result.countries ?? []
    if (!selectedCountryCode.value || !countries.value.some((country) => country.code === selectedCountryCode.value)) {
      selectedCountryCode.value = countries.value[0]?.code ?? ''
    }
    await loadLubanServices()
  } catch (err) {
    catalogError.value = getErrorMessage(err)
  } finally {
    catalogLoading.value = false
  }
}

async function loadLubanServices() {
  const country = selectedCountry.value
  if (!country) return
  catalogLoading.value = true
  catalogError.value = ''
  try {
    const result = await api.getLubanServices(country.name_en || country.name_cn, serviceSearch.value.trim())
    services.value = result.services ?? []
    if (!services.value.some((service) => service.service_id === selectedServiceId.value)) selectedServiceId.value = ''
  } catch (err) {
    catalogError.value = getErrorMessage(err)
  } finally {
    catalogLoading.value = false
  }
}

function clearSmsTimer() {
  if (smsTimer !== undefined) window.clearInterval(smsTimer)
  smsTimer = undefined
}

async function pollLubanSms() {
  if (!numberSession.value) return
  try {
    pollingError.value = ''
    const status = await api.getLubanSms(numberSession.value.requestId)
    smsStatus.value = status
    if (status.status === 'received') clearSmsTimer()
  } catch (err) {
    pollingError.value = getErrorMessage(err)
  }
}

async function requestLubanNumber() {
  if (!selectedServiceId.value) return toast.error('请先选择国家和接码服务')
  clearSmsTimer()
  numberLoading.value = true
  pollingError.value = ''
  try {
    numberSession.value = await api.requestLubanNumber(selectedServiceId.value)
    smsStatus.value = { requestId: numberSession.value.requestId, status: 'waiting', number: numberSession.value.number }
    smsTimer = window.setInterval(() => { void pollLubanSms() }, 3000)
    toast.success('号码已获取，请在目标平台填写并发送验证码')
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    numberLoading.value = false
  }
}

async function releaseLubanNumber() {
  if (!numberSession.value) return
  clearSmsTimer()
  try {
    await api.releaseLubanNumber(numberSession.value.requestId)
    numberSession.value = null
    smsStatus.value = null
    toast.success('号码已释放')
  } catch (err) {
    toast.error(getErrorMessage(err))
  }
}

async function copyText(value: string, message: string) {
  try {
    await navigator.clipboard.writeText(value)
    toast.success(message)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

async function saveLubanKey() {
  if (!lubanKey.value.trim()) return toast.error('请输入鲁班接码 API 密钥')
  savingKey.value = true
  try {
    await api.saveLubanKey(lubanKey.value.trim())
    lubanKey.value = ''
    toast.success('鲁班接码密钥已安全保存')
    await loadLubanBalance()
    if (luban.value?.configured) await loadLubanCatalog()
  } catch (err) {
    toast.error(getErrorMessage(err))
  } finally {
    savingKey.value = false
  }
}

onMounted(async () => {
  await loadLubanBalance()
  if (luban.value?.configured) await loadLubanCatalog()
})
onBeforeUnmount(clearSmsTimer)
</script>

<template>
  <div class="page-stack">
    <div class="page-toolbar">
      <div><p class="page-description">管理鲁班接码密钥、余额、服务价格与实时验证码。</p></div>
    </div>

    <section class="panel luban-panel">
      <div class="panel__header panel__header--wrap">
        <div><h2>鲁班接码</h2><p>页面加载时自动通过后端查询账户余额，密钥不会返回浏览器。</p></div>
        <a class="text-link" href="https://lubansms.com/api_docs/" target="_blank" rel="noopener">API 文档<ExternalLink :size="13" /></a>
      </div>
      <div class="luban-grid">
        <article class="luban-balance" :class="{ 'is-configured': luban?.configured }">
          <span class="luban-balance__icon"><WalletCards :size="22" /></span>
          <div><small>接码账户余额</small><strong>{{ luban?.balance == null ? '—' : Number(luban.balance).toFixed(2) }}</strong><span>{{ luban?.configured ? (lubanError || `已于 ${formatDateTime(luban.checkedAt)} 查询`) : '尚未配置 API 密钥' }}</span></div>
        </article>
        <form class="luban-key-form" @submit.prevent="saveLubanKey">
          <label class="field"><span><KeyRound :size="15" />API 密钥</span><input v-model="lubanKey" type="password" autocomplete="new-password" :placeholder="luban?.configured ? '已配置，输入新密钥可替换' : '输入 Luban API Key'" /></label>
          <button class="button button--primary" type="submit" :disabled="savingKey || !lubanKey.trim()">{{ savingKey ? '保存中…' : '保存并查询余额' }}</button>
        </form>
      </div>
    </section>

    <section v-if="luban?.configured" class="panel luban-activation-panel">
      <div class="panel__header panel__header--wrap">
        <div><h2>实时接码</h2><p>选择国家与服务查看单次价格，获取号码后在目标平台填写并发送验证码。</p></div>
        <span class="luban-poll-note"><RotateCw :size="13" /> 验证码每 3 秒自动检查</span>
      </div>
      <div class="luban-activation-grid">
        <div class="luban-catalog">
          <div class="luban-catalog__filters">
            <label class="field"><span><Globe2 :size="15" />国家 / 地区</span><select v-model="selectedCountryCode" @change="loadLubanServices"><option value="" disabled>选择国家</option><option v-for="country in countries" :key="country.code" :value="country.code">{{ country.name_cn || country.name_en }}<template v-if="country.name_en && country.name_cn"> · {{ country.name_en }}</template></option></select></label>
            <label class="field"><span>服务筛选</span><input v-model="serviceSearch" placeholder="例如 ChatGPT、WhatsApp" @keyup.enter="loadLubanServices" /></label>
            <button class="button button--secondary" type="button" :disabled="catalogLoading || !selectedCountryCode" @click="loadLubanServices"><RefreshCw :size="15" :class="{ spinning: catalogLoading }" />查询价格</button>
          </div>
          <div v-if="catalogError" class="inline-alert inline-alert--warning"><XCircle :size="17" /><span>{{ catalogError }}</span></div>
          <div v-if="catalogLoading && !services.length" class="luban-catalog__empty"><span class="spinner" />正在读取国家服务价格…</div>
          <div v-else-if="!sortedServices.length" class="luban-catalog__empty">暂无匹配服务，请更换国家或服务关键词。</div>
          <div v-else class="luban-service-list" role="listbox" aria-label="鲁班接码服务价格">
            <button v-for="service in sortedServices" :key="`${service.service_id}-${service.provider}`" class="luban-service" :class="{ 'is-selected': selectedServiceId === service.service_id }" type="button" role="option" :aria-selected="selectedServiceId === service.service_id" @click="selectedServiceId = service.service_id">
              <span class="luban-service__icon"><Phone :size="16" /></span>
              <span class="luban-service__main"><strong>{{ service.service_name || '未命名服务' }}</strong><small>{{ service.provider || '鲁班供应商' }} · {{ service.country_name_zh || service.country_name_en }}</small></span>
              <span class="luban-service__price">¥{{ Number(service.cost ?? 0).toFixed(2) }}<small>/次</small></span>
              <Check v-if="selectedServiceId === service.service_id" :size="16" class="luban-service__check" />
            </button>
          </div>
        </div>

        <aside class="luban-session" :class="{ 'is-active': numberSession }">
          <div class="luban-session__heading"><span class="luban-session__icon"><Phone :size="19" /></span><div><strong>当前号码</strong><small>{{ numberSession ? '请在目标平台填写此号码' : '选择服务后获取号码' }}</small></div></div>
          <div v-if="numberSession" class="luban-session__number"><strong>{{ numberSession.number }}</strong><button class="icon-button" type="button" aria-label="复制号码" title="复制号码" @click="copyText(numberSession.number, '号码已复制')"><Copy :size="16" /></button></div>
          <div v-else class="luban-session__placeholder">— — — — —</div>
          <div v-if="numberSession" class="luban-session__status" :class="smsStatus?.status === 'received' ? 'is-received' : ''"><span class="luban-session__status-dot" /><span v-if="smsStatus?.status === 'received'">验证码：<strong>{{ smsStatus.code }}</strong></span><span v-else>等待目标平台发送验证码…</span></div>
          <p v-if="pollingError" class="luban-session__error">{{ pollingError }}</p>
          <div class="luban-session__actions">
            <button v-if="!numberSession" class="button button--primary" type="button" :disabled="numberLoading || !selectedServiceId" @click="requestLubanNumber"><Phone :size="15" />{{ numberLoading ? '获取中…' : '获取号码' }}</button>
            <button v-else class="button button--secondary" type="button" @click="pollLubanSms"><RefreshCw :size="15" />立即刷新</button>
            <button v-if="numberSession" class="button button--ghost" type="button" @click="releaseLubanNumber"><XCircle :size="15" />释放号码</button>
          </div>
          <small v-if="numberSession" class="luban-session__hint">request_id {{ numberSession.requestId }} · 收到验证码后请及时完成验证</small>
        </aside>
      </div>
    </section>
  </div>
</template>
