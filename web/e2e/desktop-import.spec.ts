import { expect, test, type Page } from '@playwright/test'

async function mockOrbitApi(page: Page) {
  let importRequests = 0
  const importPrices: Array<string | null> = []

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
      importPrices.push(new URL(request.url()).searchParams.get('acquisitionPrice'))
      return route.fulfill({ status: 200, json: { message: 'Imported' } })
    }
    if (path.endsWith('/api/subscriptions') && request.method() === 'GET') {
      return route.fulfill({
        json: {
          subscriptions: [], total: 0, page: 1, pageSize: 10, totalPages: 0, folders: [],
          insights: { normal: 1, error: 0, priced: 1, totalCost: 12.34, averageCost: 12.34, expiringSoon: 0 },
        },
      })
    }

    return route.fulfill({ status: 404, json: { message: `Unhandled test route: ${path}` } })
  })

  return {
    count: () => importRequests,
    prices: () => [...importPrices],
  }
}

test('single-file import proceeds without a blocking browser dialog', async ({ page }) => {
  const imports = await mockOrbitApi(page)
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

  await expect.poll(imports.count).toBe(1)
  await expect(page.getByText('导入完成：成功 1 · 失败 0')).toBeVisible()
  expect(imports.prices()).toEqual([null])
  expect(dialogs).toEqual([])
})

test('single-file import with a price completes without mixed multipart fields', async ({ page }) => {
  const imports = await mockOrbitApi(page)

  await page.goto('/subscriptions')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'priced-subscription.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify({ type: 'codex', email: 'priced@example.invalid', access_token: 'test-only' })),
  })
  await page.getByRole('spinbutton', { name: '入手价格 可选' }).fill('12.34')
  await page.getByRole('button', { name: '开始导入' }).click()

  await expect.poll(imports.count).toBe(1)
  await expect(page.getByText('导入完成：成功 1 · 失败 0')).toBeVisible()
  await expect(page.getByRole('button', { name: '导入中…' })).toHaveCount(0)
  await expect(page.getByText('¥12.34')).toHaveCount(2)
  expect(imports.prices()).toEqual(['12.34'])
})

test('settings directory opens query-backed independent pages', async ({ page }) => {
  await mockOrbitApi(page)
  await page.goto('/settings')

  const cpaSectionButton = page.getByRole('button', { name: 'CPA 同步' })
  await cpaSectionButton.click()

  await expect(page).toHaveURL(/\/settings\?section=cpa$/)
  await expect(cpaSectionButton).toHaveClass(/is-active/)
  await expect(page.getByRole('heading', { name: 'CLIProxyAPI 同步' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '监控设置' })).toHaveCount(0)
})
