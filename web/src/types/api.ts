export type Id = string | number

export interface Offer {
  id?: Id
  itemId?: string
  item_id?: string
  price: number
  merchant?: string
  shopName?: string
  shop_name?: string
  shopId?: string
  shop_id?: string
  inventory?: number | string | null
  stock?: number | string | null
  updatedAt?: string
  updated_at?: string
  title?: string
  orderURL?: string
  orderUrl?: string
  order_url?: string
  [key: string]: unknown
}

export interface QuotaWindow {
  usedPercent?: number | null
  remainingPercent?: number | null
  limitWindowSeconds?: number
  resetAfterSeconds?: number
  resetAt?: string
}

export interface UsageQuota {
  planType?: string
  allowed?: boolean | null
  limitReached?: boolean
  fiveHour?: QuotaWindow | null
  sevenDay?: QuotaWindow | null
  creditsBalance?: number | null
  hasCredits?: boolean
  unlimited?: boolean
}

export interface SubscriptionConnectivity {
  status?: string
  reasonCode?: string
  httpStatus?: number
  latencyMs?: number | null
  checkedAt?: string
  error?: string
  cpaStatus?: string
  cpaStatusMessage?: string
  cpaUnavailable?: boolean
  nextRetryAt?: string
  quota?: UsageQuota | null
}

export interface Subscription {
  id: Id
  accountId?: string
  account_id?: string
  provider?: string
  email?: string
  status?: string
  category?: 'normal' | 'error' | string
  expired?: string
  connected?: boolean
  connectivity?: SubscriptionConnectivity | null
  balance?: number | null
  acquisitionPrice?: number | null
  acquisition_price?: number | null
  remaining?: number | null
  remainingQuota?: number | null
  remaining_quota?: number | null
  remainingDays?: number | null
  remaining_days?: number | null
  file?: string
  fileName?: string
  file_name?: string
  folder?: string
  orderURL?: string
  orderUrl?: string
  order_url?: string
  baseUrl?: string
  base_url?: string
  plan?: string
  planType?: string
  expiresAt?: string
  expires_at?: string
  lastError?: string
  last_error?: string
  checkedAt?: string
  checked_at?: string
  syncedToCpa?: boolean
  synced_to_cpa?: boolean
  [key: string]: unknown
}

export interface Alert {
  id?: Id
  type?: string
  level?: string
  title?: string
  message?: string
  merchant?: string
  orderUrl?: string
  price?: number
  threshold?: number
  createdAt?: string
  created_at?: string
  read?: boolean
  [key: string]: unknown
}

export interface DashboardStats {
  totalSubscriptions?: number
  connected?: number
  expired?: number
  unchecked?: number
  offersUpdatedAt?: string
  lowestPrice?: number | null
  lowest_price?: number | null
  subscriptionCount?: number
  subscription_count?: number
  connectedCount?: number
  connected_count?: number
  actionRequired?: number
  action_required?: number
  threshold?: number | null
  [key: string]: unknown
}

export interface DashboardResponse {
  offers: Offer[]
  priceHistory?: PriceSample[]
  gptPlusPriceHistory?: PriceSample[]
  subscriptions: Subscription[]
  alerts: Alert[]
  stats: DashboardStats
}

export interface LubanBalance {
  configured: boolean
  balance?: number
  checkedAt?: string
}

export interface LubanCountry {
  id?: string
  name_en?: string
  name_cn?: string
  code?: string
}

export interface LubanService {
  service_id: string
  country_name_zh?: string
  country_name_en?: string
  service_name?: string
  provider?: string
  cost?: number
}

export interface LubanNumberSession {
  requestId: string
  number: string
  status: 'waiting' | 'received'
}

export interface LubanSmsStatus {
  requestId: string
  status: 'waiting' | 'received'
  number?: string
  code?: string
}

export interface OfferFeed {
  offers: Offer[]
  updatedAt?: string
  sourceUrl?: string
  lastError?: string
}

export interface PriceSample {
  at: string
  average: number
}

export interface Settings {
  threshold?: number
  priceThreshold?: number
  price_threshold?: number
  refreshMinutes?: number
  refreshInterval?: number
  refresh_interval?: number
  baseUrl?: string
  base_url?: string
  apiKey?: string
  api_key?: string
  apiKeyConfigured?: boolean
  cpaManagementKeyConfigured?: boolean
  lubanApiKeyConfigured?: boolean
  webhookUrl?: string
  webhook_url?: string
  allowRemoteBaseUrl?: boolean
  allow_remote_base_url?: boolean
  cpaAuthDir?: string
  cpa_auth_dir?: string
  syncToCpaAuthDir?: boolean
  sync_to_cpa_auth_dir?: boolean
  [key: string]: unknown
}

export interface SubscriptionPage {
  subscriptions: Subscription[]
  total: number
  page: number
  pageSize: number
  totalPages: number
  folders: string[]
}

export interface SubscriptionQuery {
  page?: number
  pageSize?: number
  folder?: string
  status?: string
  search?: string
}

export interface ImportSubscriptionsOptions {
  file: File
  acquisitionPrice?: string
}

export interface ApiMessage {
  message?: string
  detail?: string
  success?: boolean
  [key: string]: unknown
}
