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
  ImportSubscriptionsOptions,
  Offer,
  Settings,
  SubscriptionConnectivity,
  SubscriptionPage,
  SubscriptionQuery,
} from '../types/api'

export const API_BASE = (import.meta.env.VITE_API_BASE || 'http://127.0.0.1:8080/api').replace(/\/$/, '')

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

export const api = {
  getDashboard: () => request<DashboardResponse>('/dashboard'),
  async getOffers() {
    return unwrapList(await request<Offer[] | { offers?: Offer[]; items?: Offer[]; data?: Offer[] }>('/offers'))
  },
  refreshOffers: () => request<ApiMessage>('/offers/refresh', { method: 'POST' }),
  getGptPlus: () => request<OfferFeed>('/gpt-plus'),
  refreshGptPlus: () => request<OfferFeed>('/gpt-plus/refresh', { method: 'POST' }),
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
  importSubscriptions(options: ImportSubscriptionsOptions) {
    const body = new FormData()
    // WebView2 can expose the selected File metadata correctly while sending
    // an empty multipart part when the original File object is appended
    // directly. Read the file first and rebuild a Blob so desktop and browser
    // clients send the same bytes to the Go API.
    return options.file.text().then((text) => {
      const normalized = text.replace(/^\uFEFF/, '')
      if (!normalized.trim()) {
        throw new ApiError('选择的 JSON 文件为空或无法读取', 400)
      }
      body.append('file', new Blob([normalized], { type: 'application/json' }), options.file.name)
      if (options.acquisitionPrice) body.append('acquisitionPrice', options.acquisitionPrice)
      return request<ApiMessage>('/subscriptions/import', { method: 'POST', body })
    })
  },
  testSubscription: (id: string | number) => request<SubscriptionConnectivity>(`/subscriptions/${encodeURIComponent(id)}/test`, { method: 'POST' }),
  syncSubscription: (id: string | number) => request<ApiMessage>(`/subscriptions/${encodeURIComponent(id)}/sync`, { method: 'POST' }),
  deleteSubscription: (id: string | number) => request<ApiMessage>(`/subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE' }),
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
