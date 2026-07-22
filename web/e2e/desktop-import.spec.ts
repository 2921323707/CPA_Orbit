import { expect, test, type Page } from '@playwright/test'

type CommitScenario = 'success' | 'safe-401' | 'retry-once'

async function mockOrbitApi(page: Page, commitScenario: CommitScenario = 'success') {
  let preflightRequests = 0
  let commitRequests = 0
  let legacyImportRequests = 0
  const commitPrices: Array<string | null> = []
  const commitTargets: Array<string | null> = []
  const commitTokens: Array<string | undefined> = []

  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname

    if (path.endsWith('/api/health')) return route.fulfill({ json: { status: 'ok', name: 'CPA Orbit' } })
    if (path.endsWith('/api/cpa/status')) return route.fulfill({ json: { online: true, authFileCount: 2, baseUrl: 'http://127.0.0.1:8317/v1' } })
    if (path.endsWith('/api/settings')) return route.fulfill({ json: { themeMode: 'light', baseUrl: 'http://127.0.0.1:8317/v1' } })
    if (path.endsWith('/api/subscriptions/import/preflight') && request.method() === 'POST') {
      preflightRequests += 1
      return route.fulfill({ json: {
        operationId: `operation-${preflightRequests}`,
        expiresAt: '2026-07-22T12:05:00Z',
        preflightToken: `signed-token-${preflightRequests}`,
        analysis: {
          version: 'orbit-auth-v1', format: 'cpa-auth', digest: 'sha256:test',
          identity: { provider: 'codex', type: 'codex', email: 'e***@example.invalid', accountId: 'ac***01', recognizedFields: ['account_id', 'email', 'type'] },
          compatibility: { cpa: { compatible: true, reasonCode: 'compatible_cpa_auth' }, sub2api: { compatible: true, reasonCode: 'compatible_codex_session' } },
        },
        targets: [
          { targetId: 11, kind: 'sub2api', name: 'Primary Sub2API', enabled: true, compatible: true, reasonCode: 'compatible_codex_session' },
          { targetId: 12, kind: 'cpa', name: 'CPA companion', enabled: true, compatible: true, reasonCode: 'compatible_cpa_auth' },
        ],
      } })
    }
    if (path.endsWith('/api/subscriptions/import/commit') && request.method() === 'POST') {
      commitRequests += 1
      commitPrices.push(url.searchParams.get('acquisitionPrice'))
      commitTargets.push(url.searchParams.get('targetId'))
      commitTokens.push(request.headers()['x-orbit-preflight-token'])
      if (commitScenario === 'safe-401') {
        return route.fulfill({ status: 401, json: {
          operationId: 'operation-1', subscriptionId: 'archived-401', targetId: 11,
          outcome: 'failed', retryable: false, httpStatus: 401, archived: true,
          error: { code: 'sub2api_auth_failed', message: 'Sub2API authorization failed' },
          upstream: 'access_token=must-not-render',
        } })
      }
      if (commitScenario === 'retry-once' && commitRequests === 1) {
        return route.fulfill({ status: 504, json: {
          operationId: 'operation-1', subscriptionId: 'archived-retry', targetId: 11,
          outcome: 'uncertain', retryable: true, httpStatus: 504, archived: true,
          error: { code: 'sub2api_transport_uncertain', message: 'Sub2API import result is uncertain because the connection failed' },
        } })
      }
      return route.fulfill({ status: commitScenario === 'retry-once' ? 200 : 201, json: { operationId: 'operation-1', subscriptionId: 'imported', subscription: { id: 'imported', email: 'e2e@example.invalid' }, deployment: { id: 1, subscriptionId: 'imported', targetId: Number(url.searchParams.get('targetId')), mode: 'primary', ownership: 'managed', desiredState: 'active', observedState: 'active' }, outcome: 'succeeded', retryable: false, httpStatus: 200, archived: true, idempotent: commitScenario === 'retry-once' } })
    }
    if (path.endsWith('/api/subscriptions/import') && request.method() === 'POST') {
      legacyImportRequests += 1
      return route.fulfill({ status: 410, json: { message: 'legacy endpoint must not be called' } })
    }
    if (path.endsWith('/api/subscriptions/poll-status')) return route.fulfill({ json: { enabled: true, running: false, intervalMinutes: 5, totalAccounts: 0, completed: 0, succeeded: 0, failed: 0, runsStarted: 0, runsCompleted: 0 } })
    if (path.endsWith('/api/gateways/targets')) return route.fulfill({ json: { targets: [] } })
    if (path.endsWith('/api/subscriptions') && request.method() === 'GET') {
      return route.fulfill({ json: { subscriptions: [], total: 0, page: 1, pageSize: 10, totalPages: 0, folders: [], insights: { normal: 0, pending: 0, error: 0, priced: 0, totalCost: 0, averageCost: 0, expiringSoon: 0 } } })
    }

    return route.fulfill({ status: 404, json: { message: `Unhandled test route: ${path}` } })
  })

  return {
    preflights: () => preflightRequests,
    commits: () => commitRequests,
    legacyImports: () => legacyImportRequests,
    prices: () => [...commitPrices],
    targets: () => [...commitTargets],
    tokens: () => [...commitTokens],
  }
}

test('single-file import requires preflight and one explicit compatible target', async ({ page }) => {
  const imports = await mockOrbitApi(page)

  await page.goto('/subscriptions')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'e2e-subscription.json',
    mimeType: 'application/json',
    buffer: Buffer.from(`﻿${JSON.stringify({ type: 'codex', email: 'e2e@example.invalid', access_token: 'test-only' })}`),
  })

  await expect.poll(imports.preflights).toBe(1)
  await expect(page.getByText('e***@example.invalid')).toBeVisible()
  await expect(page.getByText('account_id', { exact: true })).toBeVisible()
  await expect(page.getByText('compatible_codex_session')).toBeVisible()
  const commit = page.getByRole('button', { name: '确认归档并部署' })
  await expect(commit).toBeDisabled()
  await page.getByRole('radio', { name: /Primary Sub2API/ }).check()
  await expect(commit).toBeEnabled()
  await commit.click()

  await expect.poll(imports.commits).toBe(1)
  await expect(page.getByText('导入提交完成：成功 1 · 失败 0')).toBeVisible()
  expect(imports.targets()).toEqual(['11'])
  expect(imports.tokens()).toEqual(['signed-token-1'])
  expect(imports.legacyImports()).toBe(0)
})

test('single-file commit sends acquisition price outside file-only multipart', async ({ page }) => {
  const imports = await mockOrbitApi(page)

  await page.goto('/subscriptions')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'priced-subscription.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify({ type: 'codex', email: 'priced@example.invalid', access_token: 'test-only' })),
  })
  await expect.poll(imports.preflights).toBe(1)
  await page.getByRole('spinbutton', { name: '入手价格 可选' }).fill('12.34')
  await page.getByRole('radio', { name: /CPA companion/ }).check()
  await page.getByRole('button', { name: '确认归档并部署' }).click()

  await expect.poll(imports.commits).toBe(1)
  await expect(page.getByText('导入提交完成：成功 1 · 失败 0')).toBeVisible()
  expect(imports.prices()).toEqual(['12.34'])
  expect(imports.targets()).toEqual(['12'])
  expect(imports.legacyImports()).toBe(0)
})

test('safe 401 deployment failure retains archived queue item without exposing upstream text', async ({ page }) => {
  const imports = await mockOrbitApi(page, 'safe-401')

  await page.goto('/subscriptions')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'unauthorized-subscription.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify({ type: 'codex', access_token: 'test-only' })),
  })
  await expect.poll(imports.preflights).toBe(1)
  await page.getByRole('radio', { name: /Primary Sub2API/ }).check()
  await page.getByRole('button', { name: '确认归档并部署' }).click()

  await expect(page.getByText('归档已保留，但目标部署未完成')).toBeVisible()
  await expect(page.getByText('Sub2API authorization failed')).toBeVisible()
  await expect(page.getByText('operation-1')).toBeVisible()
  await expect(page.getByText(/Primary Sub2API \(#11\)/)).toBeVisible()
  await expect(page.getByText('failed · HTTP 401')).toBeVisible()
  await expect(page.getByText('access_token=must-not-render')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '重新预检' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '重试同一目标与操作' })).toHaveCount(0)
  expect(imports.commits()).toBe(1)
})

test('retryable uncertain failure retries same operation and target without preflight', async ({ page }) => {
  const imports = await mockOrbitApi(page, 'retry-once')

  await page.goto('/subscriptions')
  await page.locator('input[type="file"]').setInputFiles({
    name: 'retryable-subscription.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify({ type: 'codex', access_token: 'test-only' })),
  })
  await expect.poll(imports.preflights).toBe(1)
  await page.getByRole('radio', { name: /Primary Sub2API/ }).check()
  await page.getByRole('button', { name: '确认归档并部署' }).click()

  await expect(page.getByText('uncertain · HTTP 504')).toBeVisible()
  await page.getByRole('button', { name: '重试同一目标与操作' }).click()
  await expect.poll(imports.commits).toBe(2)
  await expect(page.getByText('原操作重试成功（幂等恢复）')).toBeVisible()
  expect(imports.preflights()).toBe(1)
  expect(imports.targets()).toEqual(['11', '11'])
  expect(imports.tokens()).toEqual(['signed-token-1', 'signed-token-1'])
})

test('failed and uncertain bindings remain visible as diagnostics', async ({ page }) => {
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (path.endsWith('/api/health')) return route.fulfill({ json: { status: 'ok', name: 'CPA Orbit' } })
    if (path.endsWith('/api/cpa/status')) return route.fulfill({ json: { online: true, authFileCount: 2 } })
    if (path.endsWith('/api/settings')) return route.fulfill({ json: { themeMode: 'light' } })
    if (path.endsWith('/api/subscriptions/poll-status')) return route.fulfill({ json: { enabled: true, running: false, intervalMinutes: 5, totalAccounts: 1, completed: 0, succeeded: 0, failed: 0, runsStarted: 0, runsCompleted: 0 } })
    if (path.endsWith('/api/subscriptions') && request.method() === 'GET') return route.fulfill({ json: {
      subscriptions: [{ id: 'binding-failed', email: 'binding@example.invalid' }],
      total: 1, page: 1, pageSize: 10, totalPages: 1, folders: [],
      insights: { normal: 0, pending: 1, error: 0, priced: 0, totalCost: 0, averageCost: 0, expiringSoon: 0 },
    } })
    if (path.endsWith('/api/subscriptions/binding-failed/bindings')) return route.fulfill({ json: { bindings: [{
      id: 9, subscriptionId: 'binding-failed', targetId: 11, mode: 'primary', ownership: 'managed',
      desiredState: 'active', observedState: 'uncertain', lastError: 'Safe deployment result unavailable',
    }] } })
    if (path.endsWith('/api/gateways/targets')) return route.fulfill({ json: { targets: [{ id: 11, kind: 'sub2api', name: 'Primary Sub2API', baseUrl: '', enabled: true, primary: true, allowRemote: false, defaultConcurrency: 1, defaultPriority: 0, rateMultiplier: 1 }] } })
    return route.fulfill({ status: 404, json: { message: `Unhandled test route: ${path}` } })
  })

  await page.goto('/subscriptions')
  await expect(page.getByText('Primary Sub2API · 结果不确定')).toBeVisible()
  await expect(page.getByText('Safe deployment result unavailable')).toBeVisible()
  await page.getByRole('button', { name: /查看 .* 详情/ }).click()
  await expect(page.getByText('运行绑定诊断')).toBeVisible()
  await expect(page.getByRole('button', { name: '部署到主号池' })).toHaveCount(0)
})

test('settings gateways contains target controls while operations is absent', async ({ page }) => {
  let savedTarget: Record<string, unknown> | null = null
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (path.endsWith('/api/health')) return route.fulfill({ json: { status: 'ok', name: 'CPA Orbit' } })
    if (path.endsWith('/api/cpa/status')) return route.fulfill({ json: { online: true, authFileCount: 2 } })
    if (path.endsWith('/api/settings')) return route.fulfill({ json: { themeMode: 'light' } })
    if (path.endsWith('/api/gateways/overview')) return route.fulfill({ json: { targets: [{ target: { id: 1, kind: 'sub2api', name: '主号池', baseUrl: 'http://127.0.0.1:8080', enabled: true, primary: true, allowRemote: false, defaultConcurrency: 2, defaultPriority: 0, rateMultiplier: 1, adminKeyConfigured: true }, health: { status: 'ok', latencyMs: 12 } }], bindings: [], operations: [], snapshots: [], checkedAt: '2026-07-22T08:00:00Z' } })
    if (path.endsWith('/api/gateways/targets') && request.method() === 'POST') {
      savedTarget = request.postDataJSON()
      return route.fulfill({ json: { id: 2, ...savedTarget } })
    }
    return route.fulfill({ status: 404, json: { message: `Unhandled test route: ${path}` } })
  })
  await page.goto('/settings?section=gateways')
  await expect(page.getByRole('heading', { name: '网关配置' })).toBeVisible()
  await expect(page.getByText('主号池', { exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: '号池运维' })).toHaveCount(0)
  await page.getByRole('button', { name: '添加网关' }).click()
  await page.getByLabel('显示名称').fill('New gateway')
  await page.getByLabel('管理密钥').fill('write-only-key')
  await page.getByRole('button', { name: '保存目标' }).click()
  await expect.poll(() => savedTarget).toMatchObject({ name: 'New gateway', adminKey: 'write-only-key' })
  await page.goto('/operations')
  await expect(page).toHaveURL(/\/$/)
})

test('toolbox keeps Luban and removes external converter', async ({ page }) => {
  await mockOrbitApi(page)
  await page.goto('/toolbox')
  await expect(page.getByRole('heading', { name: '鲁班接码' })).toBeVisible()
  await expect(page.getByText('JSON 转换台')).toHaveCount(0)
  await expect(page.locator('a[href="https://cvt.okcode.cc.cd/"]')).toHaveCount(0)
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
