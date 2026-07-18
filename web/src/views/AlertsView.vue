<script setup lang="ts">
import { BellRing, ExternalLink, Monitor, Send, Volume2 } from 'lucide-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import EmptyState from '../components/common/EmptyState.vue'
import ErrorState from '../components/common/ErrorState.vue'
import LoadingState from '../components/common/LoadingState.vue'
import PaginationBar from '../components/common/PaginationBar.vue'
import StatusBadge from '../components/common/StatusBadge.vue'
import { useToast } from '../composables/useToast'
import { api } from '../services/api'
import type { Alert } from '../types/api'
import { formatCurrency, formatDateTime, getErrorMessage } from '../utils/format'
import { alertCreatedAt } from '../utils/models'

interface AlertPreferences { ui: boolean; browser: boolean; sound: boolean }
const STORAGE_KEY = 'cpa-monitor-alert-preferences'
const defaults: AlertPreferences = { ui: true, browser: false, sound: false }

const alerts = ref<Alert[]>([])
const loading = ref(true)
const error = ref('')
const page = ref(1)
const pageSize = 10
const preferences = ref<AlertPreferences>({ ...defaults })
const notificationPermission = ref<NotificationPermission | 'unsupported'>('Notification' in window ? Notification.permission : 'unsupported')
const toast = useToast()
const totalPages = computed(() => Math.max(1, Math.ceil(alerts.value.length / pageSize)))
const pagedAlerts = computed(() => alerts.value.slice((page.value - 1) * pageSize, page.value * pageSize))

function loadPreferences() {
  try {
    preferences.value = { ...defaults, ...JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}') }
  } catch {
    preferences.value = { ...defaults }
  }
}

watch(preferences, (value) => localStorage.setItem(STORAGE_KEY, JSON.stringify(value)), { deep: true })

async function toggleBrowser() {
  if (!('Notification' in window)) {
    preferences.value.browser = false
    return toast.error('当前浏览器不支持系统通知')
  }
  if (!preferences.value.browser) return
  const permission = await Notification.requestPermission()
  notificationPermission.value = permission
  if (permission !== 'granted') {
    preferences.value.browser = false
    toast.error('浏览器通知权限未获允许')
  } else {
    new Notification('CPA Monitor', { body: '浏览器提醒已开启。' })
  }
}

function playSound() {
  const AudioContextClass = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
  if (!AudioContextClass) return
  const context = new AudioContextClass()
  const oscillator = context.createOscillator()
  const gain = context.createGain()
  oscillator.frequency.value = 660
  gain.gain.setValueAtTime(0.12, context.currentTime)
  gain.gain.exponentialRampToValueAtTime(0.001, context.currentTime + 0.25)
  oscillator.connect(gain).connect(context.destination)
  oscillator.start()
  oscillator.stop(context.currentTime + 0.25)
  oscillator.addEventListener('ended', () => context.close())
}

function toggleSound() {
  if (preferences.value.sound) playSound()
}

function preview() {
  if (preferences.value.ui) toast.show('提醒预览：检测到新的低价报价', 'info')
  if (preferences.value.browser && notificationPermission.value === 'granted') new Notification('CPA Monitor 低价提醒', { body: '检测到新的低价报价。' })
  if (preferences.value.sound) playSound()
  if (!preferences.value.ui && !preferences.value.browser && !preferences.value.sound) toast.error('请至少开启一种提醒方式')
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    alerts.value = await api.getAlerts()
    page.value = Math.min(page.value, Math.max(1, Math.ceil(alerts.value.length / pageSize)))
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
  }
}

function alertTone(alert: Alert): 'danger' | 'warning' | 'success' | 'neutral' {
  const level = String(alert.level ?? alert.type ?? '').toLowerCase()
  if (['critical', 'error', 'danger'].includes(level)) return 'danger'
  if (['warning', 'warn', 'low_price'].includes(level) || (alert.price != null && alert.threshold != null && alert.price <= alert.threshold)) return 'warning'
  if (['success', 'resolved'].includes(level)) return 'success'
  return 'neutral'
}

function alertLabel(alert: Alert) {
  if (alert.price != null && alert.threshold != null && alert.price <= alert.threshold) return '低价'
  return String(alert.level ?? alert.type ?? '提醒')
}

onMounted(() => { loadPreferences(); load() })
</script>

<template>
  <div class="page-stack">
    <section class="panel">
      <div class="panel__header panel__header--wrap">
        <div><h2>提醒方式</h2><p>偏好仅保存在当前浏览器的 localStorage 中。</p></div>
        <button class="button button--secondary" type="button" @click="preview"><Send :size="16" />发送预览</button>
      </div>
      <div class="preference-grid">
        <label class="preference-card">
          <span class="preference-card__icon"><Monitor :size="20" /></span>
          <span><strong>界面提醒</strong><small>在管理后台显示 Toast 提示</small></span>
          <input v-model="preferences.ui" class="switch" type="checkbox" />
        </label>
        <label class="preference-card">
          <span class="preference-card__icon"><BellRing :size="20" /></span>
          <span><strong>浏览器通知</strong><small>权限：{{ notificationPermission === 'unsupported' ? '不支持' : notificationPermission }}</small></span>
          <input v-model="preferences.browser" class="switch" type="checkbox" @change="toggleBrowser" />
        </label>
        <label class="preference-card">
          <span class="preference-card__icon"><Volume2 :size="20" /></span>
          <span><strong>提示声音</strong><small>在新提醒到达时播放短音</small></span>
          <input v-model="preferences.sound" class="switch" type="checkbox" @change="toggleSound" />
        </label>
      </div>
    </section>

    <section class="panel">
      <div class="panel__header"><div><h2>历史提醒</h2><p>来自后端的监控提醒记录</p></div></div>
      <LoadingState v-if="loading" label="正在加载提醒…" />
      <ErrorState v-else-if="error" :message="error" @retry="load" />
      <div v-else-if="alerts.length" class="table-wrap">
        <table class="data-table">
          <thead><tr><th>时间</th><th>级别</th><th>商家</th><th>内容</th><th class="numeric">价格 / 阈值</th><th>操作</th></tr></thead>
          <tbody><tr v-for="(item, index) in pagedAlerts" :key="String(item.id ?? index)"><td class="nowrap">{{ formatDateTime(alertCreatedAt(item)) }}</td><td><StatusBadge :tone="alertTone(item)" :label="alertLabel(item)" /></td><td class="strong">{{ item.merchant || '—' }}</td><td>{{ item.message || item.title || '监控提醒' }}</td><td class="numeric nowrap">{{ item.price == null ? '—' : formatCurrency(item.price) }}<span v-if="item.threshold != null" class="muted"> / {{ formatCurrency(item.threshold) }}</span></td><td><a v-if="item.orderUrl" class="text-link" :href="item.orderUrl" target="_blank" rel="noopener">直达支付 <ExternalLink :size="13" /></a><span v-else>{{ item.read ? '已读' : '未读' }}</span></td></tr></tbody>
        </table>
      </div>
      <PaginationBar v-if="alerts.length" :page="page" :total-pages="totalPages" :total="alerts.length" :page-size="pageSize" @change="page = $event" />
      <EmptyState v-else title="暂无历史提醒" description="触发低价或订阅异常后，记录会出现在这里。" />
    </section>
  </div>
</template>
