import { createRouter, createWebHashHistory, createWebHistory } from 'vue-router'
import { beginNavigation, finishNavigation } from './navigationLoad'

const router = createRouter({
  history: import.meta.env.VITE_DESKTOP === 'true' ? createWebHashHistory() : createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('../views/DashboardView.vue'), meta: { titleKey: 'nav.dashboard' } },
    { path: '/offers', name: 'offers', component: () => import('../views/OffersView.vue'), meta: { titleKey: 'nav.offers' } },
    { path: '/subscriptions', name: 'subscriptions', component: () => import('../views/SubscriptionsView.vue'), meta: { titleKey: 'nav.subscriptions' } },
    { path: '/alerts', redirect: { path: '/settings', query: { section: 'alerts' } } },
    { path: '/gpt-plus', redirect: '/toolbox' },
    { path: '/toolbox', name: 'toolbox', component: () => import('../views/ToolboxView.vue'), meta: { titleKey: 'nav.toolbox' } },
    { path: '/settings', name: 'settings', component: () => import('../views/SettingsView.vue'), meta: { titleKey: 'nav.settings' } },
    { path: '/docs', name: 'docs', component: () => import('../views/DocsView.vue'), meta: { titleKey: 'nav.docs' } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

router.afterEach(() => {
  finishNavigation()
})

router.beforeEach(() => {
  beginNavigation()
  return true
})

router.onError(() => {
  finishNavigation()
})

export default router
