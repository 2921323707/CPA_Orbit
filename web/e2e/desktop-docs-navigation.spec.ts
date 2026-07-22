import { expect, test } from '@playwright/test'

test('docs in-page links do not replace the desktop hash route', async ({ page }) => {
  await page.route('**/api/**', async (route) => {
    await route.fulfill({ status: 404, json: { message: 'Not needed for documentation navigation' } })
  })
  await page.route('https://api.github.com/**', async (route) => {
    await route.fulfill({ status: 503, json: { message: 'Offline in navigation test' } })
  })

  await page.goto('/#/docs')
  await expect(page).toHaveTitle(/^(使用说明|Docs) · CPA Orbit$/)

  const sectionIds = [
    'quick-start',
    'architecture',
    'offers',
    'gpt-plus',
    'subscriptions',
    'alerts',
    'security',
    'troubleshooting',
  ]
  for (const sectionId of sectionIds) {
    await page.locator(`.docs-toc a[href="#${sectionId}"]`).click()
    await expect(page).toHaveURL('http://127.0.0.1:4174/#/docs')
    await expect(page.locator(`#${sectionId}`)).toBeInViewport()
  }

  await page.locator('.docs-release-channel__link').click()
  await expect(page).toHaveURL('http://127.0.0.1:4174/#/docs')
  await expect(page.getByRole('heading', { name: /更新日志|Release Notes/ })).toBeInViewport()
})

test('gateway settings render through the desktop hash route and operations is absent', async ({ page }) => {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    switch (url.pathname) {
      case '/api/health':
        await route.fulfill({ json: { status: 'ok', name: 'CPA Orbit', version: '1.3.0' } })
        return
      case '/api/cpa/status':
        await route.fulfill({ json: { online: true, authFileCount: 1, baseUrl: 'http://127.0.0.1:8317/v1' } })
        return
      case '/api/settings':
        await route.fulfill({ json: { baseUrl: 'http://127.0.0.1:8317/v1', themeMode: 'auto' } })
        return
      case '/api/gateways/overview':
        await route.fulfill({ json: { targets: [{ target: { id: 1, kind: 'sub2api', name: 'Primary Sub2API', baseUrl: 'http://127.0.0.1:8080', enabled: true, primary: true, allowRemote: false, defaultConcurrency: 2, defaultPriority: 0, rateMultiplier: 1, adminKeyConfigured: true }, health: { status: 'ok', latencyMs: 12 } }], bindings: [], operations: [], snapshots: [], checkedAt: '2026-07-22T08:00:00Z' } })
        return
      default:
        await route.fulfill({ status: 404, json: { message: 'Unexpected desktop test request' } })
    }
  })

  await page.goto('/#/settings?section=gateways')
  await expect(page).toHaveTitle(/^设置 · CPA Orbit$/)
  await expect(page.getByRole('heading', { name: '网关配置' })).toBeVisible()
  await expect(page.getByText('Primary Sub2API', { exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: '号池运维' })).toHaveCount(0)

  await page.goto('/#/operations')
  await expect(page).toHaveURL('http://127.0.0.1:4174/#/')
})