import type {
  Alert,
  ApiMessage,
  DashboardResponse,
	OfferFeed,
  LubanBalance,
  LubanCountry,
  LubanNumberSession,
  LubanService,
  LubanSmsStatus,
  ImportCommitOptions,
  ImportCommitResponse,
  ImportPreflightOptions,
  ImportPreflightResponse,
  DeploymentBinding,
  GatewayHealth,
  GatewayOverview,
  GatewayTarget,
  GatewayUsageResponse,
  Offer,
  Settings,
  SubscriptionConnectivity,
  SubscriptionPage,
  SubscriptionPollStatus,
  SubscriptionQuery,
} from '../types/api'

export const API_BASE = (import.meta.env.VITE_API_BASE || 'http://127.0.0.1:8090/api').replace(/\/$/, '')

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  if (init?.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  let response: Response
  try {
    response = await fetch(`${API_BASE}${path}`, { ...init, headers })
  } catch {
    throw new ApiError('无法连接后端服务', 0)
  }

  const contentType = response.headers.get('content-type') || ''
  const payload = contentType.includes('application/json')
    ? await response.json().catch(() => null)
    : await response.text().catch(() => '')

  if (!response.ok) {
    const nestedError = typeof payload === 'object' && payload && typeof (payload as { error?: unknown }).error === 'object'
      ? (payload as { error: { message?: string } }).error
      : null
    const message = typeof payload === 'object' && payload
      ? String(nestedError?.message || (payload as ApiMessage).detail || (payload as ApiMessage).message || `请求失败（${response.status}）`)
      : String(payload || `请求失败（${response.status}）`)
    throw new ApiError(message, response.status)
  }

  return payload as T
}

function unwrapList<T>(payload: T[] | { items?: T[]; data?: T[]; offers?: T[]; subscriptions?: T[]; alerts?: T[] }): T[] {
  if (Array.isArray(payload)) return payload
  return payload.items ?? payload.data ?? payload.offers ?? payload.subscriptions ?? payload.alerts ?? []
}

async function normalizedImportBody(file: File): Promise<FormData> {
  // WebView2 can expose correct File metadata while serializing an empty part.
  // Rebuilding a BOM-free Blob keeps desktop/browser multipart behavior aligned.
  const normalized = (await file.text()).replace(/^﻿/, '')
  if (!normalized.trim()) throw new ApiError('选择的 JSON 文件为空或无法读取', 400)
  const body = new FormData()
  body.append('file', new Blob([normalized], { type: 'application/json' }), file.name)
  return body
}

async function timedImportRequest<T>(path: string, init: RequestInit): Promise<T> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 20_000)
  try {
    return await request<T>(path, { ...init, signal: controller.signal })
  } catch (error) {
    if (controller.signal.aborted) throw new ApiError('导入请求超时，请检查后端状态后重试', 0)
    throw error
  } finally {
    window.clearTimeout(timeout)
  }
}

export const api = {
  getDashboard: () => request<DashboardResponse>('/dashboard'),
  async getOffers() {
    return unwrapList(await request<Offer[] | { offers?: Offer[]; items?: Offer[]; data?: Offer[] }>('/offers'))
  },
  refreshOffers: () => request<ApiMessage>('/offers/refresh', { method: 'POST' }),
  getGptPlus: () => request<OfferFeed>('/gpt-plus'),
  refreshGptPlus: () => request<OfferFeed>('/gpt-plus/refresh', { method: 'POST' }),
  deletePriceHistory: (source: 'k12' | 'gpt-plus', at: string) => {
    const params = new URLSearchParams({ source, at })
    return request<ApiMessage>(`/price-history?${params.toString()}`, { method: 'DELETE' })
  },
  getLubanBalance: () => request<LubanBalance>('/luban/balance'),
  saveLubanKey: (apiKey: string) => request<LubanBalance>('/luban/key', { method: 'PUT', body: JSON.stringify({ apiKey }) }),
  getLubanCountries: () => request<{ countries: LubanCountry[] }>('/luban/countries'),
  getLubanServices: (country?: string, service?: string) => {
    const params = new URLSearchParams()
    if (country) params.set('country', country)
    if (service) params.set('service', service)
    params.set('language', 'en')
    params.set('page', '1')
    return request<{ services: LubanService[] }>(`/luban/services?${params.toString()}`)
  },
  requestLubanNumber: (serviceId: string) => request<LubanNumberSession>('/luban/number', { method: 'POST', body: JSON.stringify({ serviceId }) }),
  getLubanSms: (requestId: string) => request<LubanSmsStatus>(`/luban/sms?requestId=${encodeURIComponent(requestId)}`),
  releaseLubanNumber: (requestId: string) => request<ApiMessage>('/luban/release', { method: 'POST', body: JSON.stringify({ requestId }) }),
  getSubscriptions(query: SubscriptionQuery = {}) {
    const params = new URLSearchParams()
    if (query.page) params.set('page', String(query.page))
    if (query.pageSize) params.set('pageSize', String(query.pageSize))
    if (query.folder && query.folder !== 'all') params.set('folder', query.folder)
    if (query.status && query.status !== 'all') params.set('status', query.status)
    if (query.search) params.set('search', query.search)
    const suffix = params.size ? `?${params.toString()}` : ''
    return request<SubscriptionPage>(`/subscriptions${suffix}`)
  },
  async preflightSubscriptionImport(options: ImportPreflightOptions) {
    return timedImportRequest<ImportPreflightResponse>('/subscriptions/import/preflight', {
      method: 'POST',
      body: await normalizedImportBody(options.file),
    })
  },
  async commitSubscriptionImport(options: ImportCommitOptions) {
    // Keep multipart file-only for WebView2. Non-secret metadata remains in the
    // query string; the signed token is sent separately and never displayed.
    const params = new URLSearchParams({ targetId: String(options.targetId) })
    if (options.acquisitionPrice) params.set('acquisitionPrice', options.acquisitionPrice)
    return timedImportRequest<ImportCommitResponse>(`/subscriptions/import/commit?${params.toString()}`, {
      method: 'POST',
      headers: { 'X-Orbit-Preflight-Token': options.preflightToken },
      body: await normalizedImportBody(options.file),
    })
  },
  getSubscriptionPollStatus: () => request<SubscriptionPollStatus>('/subscriptions/poll-status'),
  pollSubscriptionsNow: () => request<{ started: boolean }>('/subscriptions/poll-now', { method: 'POST' }),
  testSubscription: (id: string | number) => request<SubscriptionConnectivity>(`/subscriptions/${encodeURIComponent(id)}/test`, { method: 'POST' }),
  syncSubscription: (id: string | number) => request<ApiMessage>(`/subscriptions/${encodeURIComponent(id)}/sync`, { method: 'POST' }),
  deleteSubscription: (id: string | number) => request<ApiMessage>(`/subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  getGatewayOverview: () => request<GatewayOverview>('/gateways/overview'),
	collectGatewayTelemetry: () => request<ApiMessage>('/gateways/collect', { method: 'POST' }),
  getGatewayTargets: () => request<{ targets: GatewayTarget[] }>('/gateways/targets'),
  saveGatewayTarget: (target: Partial<GatewayTarget> & Pick<GatewayTarget, 'kind' | 'name' | 'baseUrl'>) => request<GatewayTarget>('/gateways/targets', { method: 'POST', body: JSON.stringify(target) }),
  testGatewayTarget: (id: number) => request<GatewayHealth>(`/gateways/targets/${id}/test`, { method: 'POST' }),
  getGatewayUsage: (targetId: number, days = 7) => request<GatewayUsageResponse>(`/gateways/usage?targetId=${targetId}&days=${days}`),
  getSubscriptionBindings: (id: string | number) => request<{ bindings: DeploymentBinding[] }>(`/subscriptions/${encodeURIComponent(id)}/bindings`),
  deploySubscription: (id: string | number, targetId?: number) => request<DeploymentBinding>(`/subscriptions/${encodeURIComponent(id)}/deploy`, { method: 'POST', body: JSON.stringify(targetId ? { targetId } : {}) }),
  detachSubscription: (id: string | number, targetId: number) => request<DeploymentBinding>(`/subscriptions/${encodeURIComponent(id)}/detach`, { method: 'POST', body: JSON.stringify({ targetId }) }),
  migrateSubscription: (id: string | number, fromTargetId: number, toTargetId: number) => request<DeploymentBinding>(`/subscriptions/${encodeURIComponent(id)}/migrate`, { method: 'POST', body: JSON.stringify({ fromTargetId, toTargetId }) }),
  getSettings: () => request<Settings>('/settings'),
  updateSettings: (settings: Settings) => request<Settings | ApiMessage>('/settings', { method: 'PUT', body: JSON.stringify(settings) }),
  testWebhook: (webhookUrl?: string) => request<ApiMessage>('/settings/test-webhook', {
    method: 'POST',
    body: JSON.stringify(webhookUrl ? { webhookUrl } : {}),
  }),
  async getAlerts() {
    return unwrapList(await request<Alert[] | { alerts?: Alert[]; items?: Alert[]; data?: Alert[] }>('/alerts'))
  },
}
