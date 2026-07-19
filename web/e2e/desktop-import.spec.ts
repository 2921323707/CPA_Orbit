import { expect, test, type Page } from '@playwright/test'

async function mockOrbitApi(page: Page) {
  let importRequests = 0

  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname

    if (path.endsWith('/api/health')) {
      return route.fulfill({ json: { status: 'ok', name: 'CPA Orbit' } })
    }
    if (path.endsWith('/api/cpa/status')) {
      return route.fulfill({ json: { online: true, authFileCount: 2, baseUrl: 'http://127.0.0.1:8317/v1' } })
    }
    if (path.endsWith('/api/settings')) {
      return route.fulfill({ json: { themeMode: 'light', baseUrl: 'http://127.0.0.1:8317/v1' } })
    }
    if (path.endsWith('/api/subscriptions/import') && request.method() === 'POST') {
      importRequests += 1
      return route.fulfill({ status: 200, json: { message: 'Imported' } })
    }
    if (path.endsWith('/api/subscriptions') && request.method() === 'GET') {
      return route.fulfill({ json: { subscriptions: [], total: 0, page: 1, pageSize: 10, totalPages: 0, folders: [] } })
    }

    return route.fulfill({ status: 404, json: { message: `Unhandled test route: ${path}` } })
  })

  return () => importRequests
}

test('single-file import proceeds without a blocking browser dialog', async ({ page }) => {
  const importRequestCount = await mockOrbitApi(page)
  const dialogs: string[] = []
  page.on('dialog', async (dialog) => {
    dialogs.push(dialog.message())
    await dialog.dismiss()
  })

  await page.goto('/subscriptions')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'e2e-subscription.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify({ type: 'codex', email: 'e2e@example.invalid', access_token: 'test-only' })),
  })
  await page.getByRole('button', { name: '开始导入' }).click()

  await expect.poll(importRequestCount).toBe(1)
  await expect(page.getByText('导入完成：成功 1 · 失败 0')).toBeVisible()
  expect(dialogs).toEqual([])
})

test('settings directory scrolls within the page without changing route', async ({ page }) => {
  await mockOrbitApi(page)
  await page.goto('/settings')

  await page.getByRole('button', { name: 'CPA 同步' }).click()

  await expect(page).toHaveURL(/\/settings$/)
  await expect(page.locator('#settings-cpa')).toBeInViewport()
})
