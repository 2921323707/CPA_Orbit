<script setup lang="ts">
import {
  BookOpenText,
  FileJson2,
  Gauge,
  Github,
  Languages,
  Menu,
  MonitorCog,
  Moon,
  Server as ServerIcon,
  Settings,
  ShoppingCart,
  Sun,
  Wrench,
  X,
} from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { api, API_BASE } from '../../services/api'
import { useLocale } from '../../i18n'
import { PROJECT_GITHUB_URL, PROJECT_NAME, PROJECT_VERSION } from '../../constants/project'
import ArtLoader from '../common/ArtLoader.vue'
import { initialNavigation, navigationLoading } from '../../router/navigationLoad'

const route = useRoute()
const githubUrl = PROJECT_GITHUB_URL
const { locale, t, toggleLocale } = useLocale()
const mobileOpen = ref(false)
const online = ref<boolean | null>(null)
const cpaOnline = ref<boolean | null>(null)
const cpaAccounts = ref(0)
const cpaEndpoint = ref('127.0.0.1:8317')
const checkingStatus = ref(false)
const now = ref(new Date())
type ThemeMode = 'light' | 'dark' | 'auto'
const storedThemeMode = localStorage.getItem('cpa-monitor-theme-mode')
const themeMode = ref<ThemeMode>(storedThemeMode === 'light' || storedThemeMode === 'dark' || storedThemeMode === 'auto' ? storedThemeMode : 'auto')
const prefersDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const theme = computed<'light' | 'dark'>(() => themeMode.value === 'auto' ? (prefersDark.value ? 'dark' : 'light') : themeMode.value)
let clockTimer = 0
let statusTimer = 0

function endpointHost(raw: string, fallback: string) {
  try {
    return new URL(raw).host || fallback
  } catch {
    return fallback
  }
}

const monitorEndpoint = endpointHost(API_BASE, '127.0.0.1:8090')

function applyTheme(value: 'light' | 'dark') {
  document.documentElement.dataset.theme = value
  document.documentElement.style.colorScheme = value
}

function setThemeMode(value: ThemeMode) {
  themeMode.value = value
  localStorage.setItem('cpa-monitor-theme-mode', value)
  void api.updateSettings({ themeMode: value }).catch(() => {})
}

function cycleTheme() {
  const next: ThemeMode = themeMode.value === 'auto' ? 'light' : themeMode.value === 'light' ? 'dark' : 'auto'
  setThemeMode(next)
}

applyTheme(theme.value)
watch(theme, (value) => {
  applyTheme(value)
})

const themeLabel = computed(() => themeMode.value === 'auto' ? t('shell.auto') : themeMode.value === 'dark' ? t('shell.light') : t('shell.auto'))
let mediaQuery: MediaQueryList | undefined
const onColorSchemeChange = (event: MediaQueryListEvent) => { prefersDark.value = event.matches }

const navigation = computed(() => [
  { to: '/', label: t('nav.dashboard'), icon: Gauge },
  { to: '/offers', label: t('nav.offers'), icon: ShoppingCart },
  { to: '/subscriptions', label: t('nav.subscriptions'), icon: FileJson2 },
  { to: '/toolbox', label: t('nav.toolbox'), icon: Wrench },
  { to: '/settings', label: t('nav.settings'), icon: Settings },
  { to: '/docs', label: t('nav.docs'), icon: BookOpenText },
])

const pageTitle = computed(() => t(String(route.meta.titleKey || 'docs.title')))
const backendLabel = computed(() => online.value === true ? t('shell.backendOnline') : online.value === false ? t('shell.backendOffline') : t('shell.backendChecking'))
const cpaLabel = computed(() => cpaOnline.value === true ? `${t('shell.cpaOnline')} · ${cpaAccounts.value}` : cpaOnline.value === false ? t('shell.cpaOffline') : t('shell.cpaChecking'))

watch([locale, pageTitle], () => {
  document.title = `${pageTitle.value} · ${PROJECT_NAME}`
}, { immediate: true })

async function fetchWithTimeout(url: string, timeoutMs: number) {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs)
  try {
    return await fetch(url, { signal: controller.signal })
  } finally {
    window.clearTimeout(timeout)
  }
}

async function checkEmbeddedBackend() {
  try {
    const response = await fetchWithTimeout(`${API_BASE}/health`, 5000)
    online.value = response.ok
  } catch {
    online.value = false
  }
}

async function checkCpaService() {
  try {
    const response = await fetchWithTimeout(`${API_BASE}/cpa/status`, 6500)
    if (response.ok) {
      const cpa = await response.json() as { online?: boolean; authFileCount?: number; baseUrl?: string }
      cpaOnline.value = cpa.online === true
      cpaAccounts.value = Number(cpa.authFileCount) || 0
      if (cpa.baseUrl) cpaEndpoint.value = endpointHost(cpa.baseUrl, cpaEndpoint.value)
    } else {
      cpaOnline.value = false
    }
  } catch {
    cpaOnline.value = false
  }
}

async function checkBackend() {
	if (checkingStatus.value) return
	checkingStatus.value = true
	try {
		await Promise.all([checkEmbeddedBackend(), checkCpaService()])
	} finally {
		checkingStatus.value = false
	}
}

async function loadShellSettings() {
	try {
		const settings = await api.getSettings()
		if (settings.themeMode === 'light' || settings.themeMode === 'dark' || settings.themeMode === 'auto') {
			themeMode.value = settings.themeMode
			localStorage.setItem('cpa-monitor-theme-mode', settings.themeMode)
		}
		if (settings.baseUrl) cpaEndpoint.value = endpointHost(settings.baseUrl, cpaEndpoint.value)
	} catch {
		// The status indicator reports backend availability independently.
	}
}

watch(() => route.fullPath, () => { mobileOpen.value = false })
onMounted(() => {
	mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
	mediaQuery.addEventListener?.('change', onColorSchemeChange)
	void checkBackend()
	void loadShellSettings()
  clockTimer = window.setInterval(() => { now.value = new Date() }, 1000)
  statusTimer = window.setInterval(checkBackend, 30000)
})
onBeforeUnmount(() => {
  window.clearInterval(clockTimer)
  window.clearInterval(statusTimer)
  mediaQuery?.removeEventListener?.('change', onColorSchemeChange)
})
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar" :class="{ 'sidebar--open': mobileOpen }">
      <div class="brand">
        <img class="brand__logo" src="/favicon.svg" alt="" aria-hidden="true" />
        <div><strong>{{ t('app.name') }}</strong><small>{{ t('app.subtitle') }}</small></div>
        <button class="icon-button sidebar__close" type="button" :aria-label="t('shell.closeNav')" @click="mobileOpen = false"><X :size="20" /></button>
      </div>
      <nav :aria-label="t('shell.mainNav')">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to" class="nav-item" active-class="" exact-active-class="router-link-active">
          <component :is="item.icon" :size="19" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="sidebar__footer">
        <div class="sidebar__endpoint"><span>{{ t('shell.monitorApi') }} · v{{ PROJECT_VERSION }}</span><code :title="API_BASE">{{ monitorEndpoint }}</code></div>
        <div class="sidebar__endpoint"><span>{{ t('shell.cpaApi') }}</span><code>{{ cpaEndpoint }}</code></div>
      </div>
    </aside>
    <button v-if="mobileOpen" class="sidebar-backdrop" type="button" :aria-label="t('shell.closeNav')" @click="mobileOpen = false" />

    <div class="app-main">
      <header class="topbar">
        <button class="icon-button mobile-menu" type="button" :aria-label="t('shell.openNav')" @click="mobileOpen = true"><Menu :size="21" /></button>
        <div class="topbar__title">{{ pageTitle }}</div>
        <div class="topbar__meta">
		  <button class="server-indicator" :class="online === true ? 'is-online' : online === false ? 'is-offline' : 'is-checking'" type="button" :disabled="checkingStatus" :aria-label="`${backendLabel}，${t('shell.refreshStatus')}`" :title="`${backendLabel} · ${t('shell.refreshStatus')}`" @click="checkBackend">
			<ServerIcon :size="17" :class="{ 'status-pulse': checkingStatus }" />
		  </button>
          <span class="backend-status cpa-status" :class="cpaOnline === true ? 'is-online' : cpaOnline === false ? 'is-offline' : 'is-checking'">
            <span class="backend-status__dot" />
            {{ cpaLabel }}
          </span>
          <a v-if="githubUrl" class="icon-button github-placeholder" :href="githubUrl" target="_blank" rel="noopener noreferrer" aria-label="GitHub" title="GitHub"><Github :size="17" /></a>
          <button v-else class="icon-button github-placeholder" type="button" disabled :aria-label="t('shell.githubPending')" :title="t('shell.githubPending')"><Github :size="17" /></button>
          <button class="locale-toggle" type="button" :aria-label="t('shell.language')" :title="t('shell.language')" @click="toggleLocale"><Languages :size="15" /><span>{{ locale === 'zh-CN' ? '中' : 'EN' }}</span></button>
          <button class="icon-button theme-toggle" type="button" :aria-label="themeLabel" :title="themeLabel" @click="cycleTheme"><MonitorCog v-if="themeMode === 'auto'" :size="18" /><Sun v-else-if="themeMode === 'dark'" :size="18" /><Moon v-else :size="18" /></button>
          <time :datetime="now.toISOString()">{{ now.toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US', { hour12: false }) }}</time>
        </div>
      </header>
      <main class="content content--route">
		<ArtLoader v-if="initialNavigation" />
		<template v-else>
		  <div v-if="navigationLoading" class="route-progress" role="progressbar" :aria-label="t('common.loader')"><span /></div>
		  <RouterView v-slot="{ Component }">
			<Transition name="page-reveal" mode="out-in">
			  <component :is="Component" :key="route.fullPath" />
			</Transition>
		  </RouterView>
		</template>
      </main>
    </div>
  </div>
</template>
