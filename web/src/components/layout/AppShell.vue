<script setup lang="ts">
import {
  Bell,
  BookOpenText,
  Bot,
  FileJson2,
  Gauge,
  Github,
  Languages,
  Menu,
  Moon,
  Server as ServerIcon,
  Settings,
  ShoppingCart,
  Sun,
  X,
} from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { API_BASE } from '../../services/api'
import { useLocale } from '../../i18n'
import { PROJECT_GITHUB_URL, PROJECT_NAME, PROJECT_VERSION } from '../../constants/project'
import ArtLoader from '../common/ArtLoader.vue'
import { navigationLoading } from '../../router/navigationLoad'

const route = useRoute()
const githubUrl = PROJECT_GITHUB_URL
const { locale, t, toggleLocale } = useLocale()
const mobileOpen = ref(false)
const online = ref<boolean | null>(null)
const cpaOnline = ref<boolean | null>(null)
const cpaAccounts = ref(0)
const now = ref(new Date())
const storedTheme = localStorage.getItem('cpa-monitor-theme')
const theme = ref<'light' | 'dark'>(storedTheme === 'dark' || (!storedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light')
let clockTimer = 0
let statusTimer = 0

function applyTheme(value: 'light' | 'dark') {
  document.documentElement.dataset.theme = value
  document.documentElement.style.colorScheme = value
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
}

applyTheme(theme.value)
watch(theme, (value) => {
  applyTheme(value)
  localStorage.setItem('cpa-monitor-theme', value)
})

const navigation = computed(() => [
  { to: '/', label: t('nav.dashboard'), icon: Gauge },
  { to: '/offers', label: t('nav.offers'), icon: ShoppingCart },
  { to: '/subscriptions', label: t('nav.subscriptions'), icon: FileJson2 },
  { to: '/alerts', label: t('nav.alerts'), icon: Bell },
  { to: '/gpt-plus', label: t('nav.gptPlus'), icon: Bot },
  { to: '/settings', label: t('nav.settings'), icon: Settings },
  { to: '/docs', label: t('nav.docs'), icon: BookOpenText },
])

const pageTitle = computed(() => t(String(route.meta.titleKey || 'docs.title')))
const backendLabel = computed(() => online.value === true ? t('shell.backendOnline') : online.value === false ? t('shell.backendOffline') : t('shell.backendChecking'))
const cpaLabel = computed(() => cpaOnline.value === true ? `${t('shell.cpaOnline')} · ${cpaAccounts.value}` : cpaOnline.value === false ? t('shell.cpaOffline') : t('shell.cpaChecking'))

watch([locale, pageTitle], () => {
  document.title = `${pageTitle.value} · ${PROJECT_NAME}`
}, { immediate: true })

async function checkBackend() {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 5000)
  try {
    const [response, cpaResponse] = await Promise.all([
      fetch(`${API_BASE}/health`, { signal: controller.signal }),
      fetch(`${API_BASE}/cpa/status`, { signal: controller.signal }),
    ])
    online.value = response.ok
    if (cpaResponse.ok) {
      const cpa = await cpaResponse.json() as { online?: boolean; authFileCount?: number }
      cpaOnline.value = cpa.online === true
      cpaAccounts.value = Number(cpa.authFileCount) || 0
    } else {
      cpaOnline.value = false
    }
  } catch {
    online.value = false
    cpaOnline.value = false
  } finally {
    window.clearTimeout(timeout)
  }
}

watch(() => route.fullPath, () => { mobileOpen.value = false })
onMounted(() => {
  checkBackend()
  clockTimer = window.setInterval(() => { now.value = new Date() }, 1000)
  statusTimer = window.setInterval(checkBackend, 30000)
})
onBeforeUnmount(() => {
  window.clearInterval(clockTimer)
  window.clearInterval(statusTimer)
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
        <span>{{ t('shell.api') }} · v{{ PROJECT_VERSION }}</span>
        <code :title="API_BASE">{{ API_BASE }}</code>
      </div>
    </aside>
    <button v-if="mobileOpen" class="sidebar-backdrop" type="button" :aria-label="t('shell.closeNav')" @click="mobileOpen = false" />

    <div class="app-main">
      <header class="topbar">
        <button class="icon-button mobile-menu" type="button" :aria-label="t('shell.openNav')" @click="mobileOpen = true"><Menu :size="21" /></button>
        <div class="topbar__title">{{ pageTitle }}</div>
        <div class="topbar__meta">
          <span class="server-indicator" :class="online === true ? 'is-online' : online === false ? 'is-offline' : 'is-checking'" role="status" :aria-label="backendLabel" :title="backendLabel">
            <ServerIcon :size="17" />
          </span>
          <span class="backend-status cpa-status" :class="cpaOnline === true ? 'is-online' : cpaOnline === false ? 'is-offline' : 'is-checking'">
            <span class="backend-status__dot" />
            {{ cpaLabel }}
          </span>
          <a v-if="githubUrl" class="icon-button github-placeholder" :href="githubUrl" target="_blank" rel="noopener noreferrer" aria-label="GitHub" title="GitHub"><Github :size="17" /></a>
          <button v-else class="icon-button github-placeholder" type="button" disabled :aria-label="t('shell.githubPending')" :title="t('shell.githubPending')"><Github :size="17" /></button>
          <button class="locale-toggle" type="button" :aria-label="t('shell.language')" :title="t('shell.language')" @click="toggleLocale"><Languages :size="15" /><span>{{ locale === 'zh-CN' ? '中' : 'EN' }}</span></button>
          <button class="icon-button theme-toggle" type="button" :aria-label="theme === 'dark' ? t('shell.light') : t('shell.dark')" :title="theme === 'dark' ? t('shell.light') : t('shell.dark')" @click="toggleTheme"><Sun v-if="theme === 'dark'" :size="18" /><Moon v-else :size="18" /></button>
          <time :datetime="now.toISOString()">{{ now.toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US', { hour12: false }) }}</time>
        </div>
      </header>
      <main class="content content--route">
        <ArtLoader v-if="navigationLoading" />
        <RouterView v-else v-slot="{ Component }">
          <Transition name="page-reveal" mode="out-in">
            <component :is="Component" :key="route.fullPath" />
          </Transition>
        </RouterView>
      </main>
    </div>
  </div>
</template>
