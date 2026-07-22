import { expect, test, type Page } from '@playwright/test'

async function mockOrbitApi(page: Page) {
  let importRequests = 0
  const importPrices: Array<string | null> = []
	const importDeployments: Array<string | null> = []

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
		importDeployments.push(new URL(request.url()).searchParams.get('deploy'))
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
		deployments: () => [...importDeployments],
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
  await page.getByRole('button', { name: '导入并部署' }).click()

  await expect.poll(imports.count).toBe(1)
  await expect(page.getByText('导入完成：成功 1 · 失败 0')).toBeVisible()
  expect(imports.prices()).toEqual([null])
	expect(imports.deployments()).toEqual(['true'])
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
  await page.getByRole('button', { name: '导入并部署' }).click()

  await expect.poll(imports.count).toBe(1)
  await expect(page.getByText('导入完成：成功 1 · 失败 0')).toBeVisible()
  await expect(page.getByRole('button', { name: '导入中…' })).toHaveCount(0)
  await expect(page.getByText('¥12.34')).toHaveCount(2)
  expect(imports.prices()).toEqual(['12.34'])
	expect(imports.deployments()).toEqual(['true'])
})

test('operations page shows Sub2API primary, CPA fallback, and token telemetry', async ({ page }) => {
	await page.route('**/api/**', async (route) => {
		const path = new URL(route.request().url()).pathname
		if (path.endsWith('/api/health')) return route.fulfill({ json: { status: 'ok', name: 'CPA Orbit' } })
		if (path.endsWith('/api/cpa/status')) return route.fulfill({ json: { online: true, authFileCount: 2 } })
		if (path.endsWith('/api/settings')) return route.fulfill({ json: { themeMode: 'light' } })
		if (path.endsWith('/api/gateways/overview')) return route.fulfill({ json: {
			targets: [
				{ target: { id: 1, kind: 'sub2api', name: '主号池', baseUrl: 'http://127.0.0.1:8080', enabled: true, primary: true, allowRemote: false, defaultConcurrency: 2, defaultPriority: 0, rateMultiplier: 1, adminKeyConfigured: true }, health: { status: 'ok', latencyMs: 12 } },
				{ target: { id: 2, kind: 'cpa', name: 'CPA 备份', baseUrl: 'http://127.0.0.1:8317/v1', enabled: true, primary: false, allowRemote: false, defaultConcurrency: 1, defaultPriority: 0, rateMultiplier: 1 }, health: { status: 'ok', latencyMs: 8 } },
			],
			bindings: [{ id: 1, subscriptionId: 'plus@example.invalid', targetId: 1, mode: 'primary', ownership: 'managed', desiredState: 'active', observedState: 'active' }],
			operations: [{ id: 1, subscriptionId: 'plus@example.invalid', targetId: 1, kind: 'deploy', status: 'succeeded', attempt: 1, createdAt: '2026-07-22T08:00:00Z' }],
			snapshots: [{ targetId: 1, data: { stats: { today_tokens: 3456, today_requests: 18, today_actual_cost: 0.42 } }, stale: false, lastAttemptAt: '2026-07-22T08:00:00Z' }], checkedAt: '2026-07-22T08:00:00Z',
		} })
		if (path.endsWith('/api/gateways/usage')) return route.fulfill({ json: { buckets: [{ id: 1, targetId: 1, bucketAt: '2026-07-22T08:00:00Z', bucketMinutes: 15, requests: 18, successes: 18, failures: 0, inputTokens: 2000, outputTokens: 1000, cacheCreationTokens: 0, cacheReadTokens: 456, cost: 0.5, actualCost: 0.42, averageDurationMs: 800, firstTokenMs: 220 }], snapshots: [], from: '2026-07-15T00:00:00Z', to: '2026-07-22T00:00:00Z' } })
		return route.fulfill({ status: 404, json: { message: `Unhandled test route: ${path}` } })
	})

	await page.goto('/operations')
	await expect(page.getByRole('heading', { name: '订阅号池运行台' })).toBeVisible()
	await expect(page.getByLabel('运行指标').getByText('3,456', { exact: true })).toBeVisible()
	await expect(page.getByText('主号池')).toBeVisible()
	await expect(page.getByText('CPA 备份')).toBeVisible()
	await expect(page.getByRole('img', { name: 'Token 用量趋势' })).toBeVisible()
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
