import { computed, ref } from 'vue'

export type Locale = 'zh-CN' | 'en-US'

const STORAGE_KEY = 'cpa-monitor-locale'
const stored = typeof window !== 'undefined' ? window.localStorage.getItem(STORAGE_KEY) : null
export const locale = ref<Locale>(stored === 'en-US' ? 'en-US' : 'zh-CN')

const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'app.name': 'CPA Orbit',
    'app.subtitle': '运营控制台',
    'nav.dashboard': '总览',
    'nav.offers': 'Price',
    'nav.subscriptions': '订阅文件',
    'nav.operations': '号池运维',
    'nav.alerts': '提醒中心',
    'nav.toolbox': '工具箱',
    'nav.settings': '设置',
    'nav.docs': '使用说明',
    'shell.api': 'API',
    'shell.monitorApi': 'Monitor API',
    'shell.cpaApi': '本地 CPA',
    'shell.backendOnline': '后端在线',
    'shell.backendOffline': '后端离线',
    'shell.backendChecking': '后端检查中',
    'shell.refreshStatus': '立即刷新服务状态',
    'shell.cpaOnline': 'CPA 在线',
    'shell.cpaOffline': 'CPA 离线',
    'shell.cpaChecking': 'CPA 检查中',
    'shell.dark': '切换到深色模式',
    'shell.light': '切换到浅色模式',
    'shell.auto': '使用自动主题',
    'shell.githubPending': 'GitHub 项目地址待配置',
    'shell.language': '语言',
    'shell.mainNav': '主导航',
    'shell.openNav': '打开导航',
    'shell.closeNav': '关闭导航',
    'docs.title': '使用说明',
    'docs.release': '版本更新',
    'docs.version': '当前版本 v1.2.0',
    'docs.releaseChannel': '发布通道',
    'docs.releasePending': 'GitHub Releases 待连接',
    'docs.releaseSyncing': '正在同步 GitHub Releases',
    'docs.releaseSynced': '已同步 GitHub Releases',
    'docs.releaseFallback': 'GitHub 暂不可用 · 使用本地日志',
    'common.pagination': '表格分页',
    'common.total': '共',
    'common.items': '条',
    'common.perPage': '每页',
    'common.page': '第',
    'common.prev': '上一页',
    'common.next': '下一页',
    'common.loader': '正在整理数据视图…',
    'common.loaderCaption': '导航已就绪 · 数据正在编排',
  },
  'en-US': {
    'app.name': 'CPA Orbit',
    'app.subtitle': 'Operations Console',
    'nav.dashboard': 'Overview',
    'nav.offers': 'Price',
    'nav.subscriptions': 'Subscriptions',
    'nav.operations': 'Pool Operations',
    'nav.alerts': 'Alerts',
    'nav.toolbox': 'Toolbox',
    'nav.settings': 'Settings',
    'nav.docs': 'Docs',
    'shell.api': 'API',
    'shell.monitorApi': 'Monitor API',
    'shell.cpaApi': 'Local CPA',
    'shell.backendOnline': 'Backend online',
    'shell.backendOffline': 'Backend offline',
    'shell.backendChecking': 'Checking backend',
    'shell.refreshStatus': 'Refresh service status',
    'shell.cpaOnline': 'CPA online',
    'shell.cpaOffline': 'CPA offline',
    'shell.cpaChecking': 'Checking CPA',
    'shell.dark': 'Switch to dark mode',
    'shell.light': 'Switch to light mode',
    'shell.auto': 'Use automatic theme',
    'shell.githubPending': 'GitHub project URL not configured',
    'shell.language': 'Language',
    'shell.mainNav': 'Main navigation',
    'shell.openNav': 'Open navigation',
    'shell.closeNav': 'Close navigation',
    'docs.title': 'Documentation',
    'docs.release': 'Release notes',
    'docs.version': 'Current version v1.2.0',
    'docs.releaseChannel': 'Release channel',
    'docs.releasePending': 'GitHub Releases pending',
    'docs.releaseSyncing': 'Syncing GitHub Releases',
    'docs.releaseSynced': 'GitHub Releases synced',
    'docs.releaseFallback': 'GitHub unavailable · Local notes',
    'common.pagination': 'Table pagination',
    'common.total': 'Total',
    'common.items': 'items',
    'common.perPage': 'Per page',
    'common.page': 'Page',
    'common.prev': 'Previous',
    'common.next': 'Next',
    'common.loader': 'Preparing the data view…',
    'common.loaderCaption': 'Navigation ready · Arranging data',
  },
}

export function setLocale(value: Locale) {
  locale.value = value
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, value)
    document.documentElement.lang = value === 'en-US' ? 'en' : 'zh-CN'
  }
}

export function toggleLocale() {
  setLocale(locale.value === 'zh-CN' ? 'en-US' : 'zh-CN')
}

export function useLocale() {
  const isEnglish = computed(() => locale.value === 'en-US')
  const t = (key: string) => messages[locale.value][key] ?? messages['zh-CN'][key] ?? key
  return { locale, isEnglish, t, setLocale, toggleLocale }
}

if (typeof document !== 'undefined') document.documentElement.lang = locale.value === 'en-US' ? 'en' : 'zh-CN'
