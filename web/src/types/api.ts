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
  source?: 'k12' | 'gpt-plus' | string
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
  gptPlusOffers?: Offer[]
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
  accountPollMinutes?: number
  account_poll_minutes?: number
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
  themeMode?: 'light' | 'dark' | 'auto'
  startOnLogin?: boolean
  closeToTray?: boolean
  desktopNotifications?: boolean
  flashOnAlert?: boolean
  [key: string]: unknown
}

export interface SubscriptionPage {
  subscriptions: Subscription[]
  total: number
  page: number
  pageSize: number
  totalPages: number
  folders: string[]
  insights?: SubscriptionInsights
}

export interface SubscriptionInsights {
  normal: number
  pending: number
  error: number
  priced: number
  totalCost: number
  averageCost: number
  expiringSoon: number
}

export interface SubscriptionQuery {
  page?: number
  pageSize?: number
  folder?: string
  status?: string
  search?: string
}

export interface ImportIdentitySummary {
  provider: string
  type?: string
  email?: string
  accountId?: string
  recognizedFields: string[]
}

export interface ImportTargetCompatibility {
  compatible: boolean
  reasonCode: string
}

export interface ImportAnalysisState {
  state?: 'none' | 'duplicate' | 'conflict' | 'unknown' | string
  reasonCode?: string
  message?: string
}

export interface ImportAnalysis {
  version: string
  format: string
  identity: ImportIdentitySummary
  compatibility: Partial<Record<'cpa' | 'sub2api', ImportTargetCompatibility>>
  digest: string
  duplicate?: ImportAnalysisState
  conflict?: ImportAnalysisState
}

export interface ImportTargetOption {
  targetId: number
  kind: 'sub2api' | 'cpa'
  name: string
  enabled: boolean
  compatible: boolean
  reasonCode: string
}

export interface ImportPreflightResponse {
  operationId: string
  expiresAt: string
  preflightToken: string
  analysis: ImportAnalysis
  targets: ImportTargetOption[]
}

export interface ImportPreflightOptions {
  file: File
}

export interface ImportCommitOptions {
  file: File
  preflightToken: string
  targetId: number
  acquisitionPrice?: string
}

export interface ImportCommitResponse {
  operationId: string
  subscriptionId: string
  subscription?: Subscription
  deployment: DeploymentBinding | null
  outcome: 'succeeded' | 'failed' | 'uncertain' | string
  retryable: boolean
  httpStatus: number
  archived: boolean
  idempotent: boolean
}

export interface SubscriptionPollStatus {
  enabled: boolean
  running: boolean
  intervalMinutes: number
  nextRunAt?: string
  lastStartedAt?: string
  lastFinishedAt?: string
  totalAccounts: number
  completed: number
  succeeded: number
  failed: number
  runsStarted: number
  runsCompleted: number
  lastError?: string
}

export interface GatewayTarget {
  id: number
  kind: 'sub2api' | 'cpa'
  name: string
  baseUrl: string
  adminKey?: string
  adminKeyConfigured?: boolean
  enabled: boolean
  primary: boolean
  allowRemote: boolean
  defaultGroupIds?: number[]
  defaultConcurrency: number
  defaultPriority: number
  rateMultiplier: number
  createdAt?: string
  updatedAt?: string
}

export interface GatewayHealth {
  status: 'ok' | 'unavailable' | 'disabled' | string
  latencyMs?: number
  checkedAt?: string
  message?: string
}

export interface GatewayTargetStatus {
  target: GatewayTarget
  health: GatewayHealth
}

export interface DeploymentBinding {
  id: number
  subscriptionId: string
  targetId: number
  remoteAccountId?: string
  mode: 'primary' | 'fallback' | string
  ownership: 'managed' | 'adopted' | string
  desiredState: string
  observedState: string
  lastError?: string
  lastSyncedAt?: string
}

export interface SyncOperation {
  id: number
  subscriptionId: string
  targetId: number
  kind: string
  status: string
  attempt: number
  lastError?: string
  createdAt: string
  completedAt?: string
}

export interface GatewaySnapshot {
  targetId: number
  data: Record<string, unknown>
  stale: boolean
  lastError?: string
  lastAttemptAt: string
  lastSuccessAt?: string
}

export interface GatewayOverview {
  targets: GatewayTargetStatus[]
  bindings: DeploymentBinding[]
  operations: SyncOperation[]
  snapshots: GatewaySnapshot[]
  checkedAt: string
}

export interface UsageBucket {
  id: number
  targetId: number
  bucketAt: string
  bucketMinutes: number
  accountId?: string
  groupName?: string
  model?: string
  requests: number
  successes: number
  failures: number
  inputTokens: number
  outputTokens: number
  cacheCreationTokens: number
  cacheReadTokens: number
  cost: number
  actualCost: number
  averageDurationMs: number
  firstTokenMs: number
}

export interface GatewayUsageResponse {
  buckets: UsageBucket[]
  snapshots: GatewaySnapshot[]
  from: string
  to: string
}

export interface ApiMessage {
  message?: string
  detail?: string
  success?: boolean
  [key: string]: unknown
}
