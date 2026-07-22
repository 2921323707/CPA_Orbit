import type { Alert, DashboardStats, Offer, Settings, Subscription } from '../types/api'

export const offerItemId = (offer: Offer) => String(offer.itemId ?? offer.item_id ?? offer.id ?? '—')
export const offerMerchant = (offer: Offer) => String(offer.merchant ?? offer.shopName ?? offer.shop_name ?? '—')
export const offerShopId = (offer: Offer) => String(offer.shopId ?? offer.shop_id ?? '—')
export const offerInventory = (offer: Offer) => offer.inventory ?? offer.stock ?? null
export const offerUpdatedAt = (offer: Offer) => offer.updatedAt ?? offer.updated_at
export const offerOrderUrl = (offer: Offer) => String(offer.orderURL ?? offer.orderUrl ?? offer.order_url ?? '')

export const subscriptionAccountId = (item: Subscription) => String(item.accountId ?? item.account_id ?? item.id)
export const subscriptionProvider = (item: Subscription) => String(item.provider ?? item.type ?? 'unknown')
export const subscriptionConnected = (item: Subscription) => item.connectivity
  ? ['connected', 'active', 'ok', 'healthy'].includes(String(item.connectivity.status ?? '').toLowerCase())
  : (item.connected ?? false)
export const subscriptionRemaining = (item: Subscription) => item.balance ?? item.remainingQuota ?? item.remaining_quota ?? item.remaining ?? null
export const subscriptionAcquisitionPrice = (item: Subscription) => item.acquisitionPrice ?? item.acquisition_price ?? null
export const subscriptionRemainingDays = (item: Subscription) => item.remainingDays ?? item.remaining_days ?? null
export const subscriptionFile = (item: Subscription) => String(item.fileName ?? item.file_name ?? item.file ?? '—')
export const subscriptionFolder = (item: Subscription) => String(item.folder ?? '').trim()
export const subscriptionOrderUrl = (item: Subscription) => String(item.orderURL ?? item.orderUrl ?? item.order_url ?? '')
export const subscriptionBaseUrl = (item: Subscription) => String(item.baseUrl ?? item.base_url ?? '')
export const subscriptionExpiresAt = (item: Subscription) => item.expired ?? item.expiresAt ?? item.expires_at
export const subscriptionPlan = (item: Subscription) => String(item.planType ?? item.plan ?? '—')
export const subscriptionLastError = (item: Subscription) => String(item.connectivity?.error ?? item.lastError ?? item.last_error ?? '')
export const subscriptionCheckedAt = (item: Subscription) => item.connectivity?.checkedAt ?? item.checkedAt ?? item.checked_at
export const subscriptionCheckFreshness = (item: Subscription, now = Date.now()) => {
  const checkedAt = subscriptionCheckedAt(item)
  if (!checkedAt) return 'never' as const
  const timestamp = Date.parse(checkedAt)
  if (!Number.isFinite(timestamp) || now - timestamp > 15 * 60 * 1000) return 'stale' as const
  return 'fresh' as const
}
export const subscriptionSyncedToCpa = (item: Subscription) => item.syncedToCpa ?? item.synced_to_cpa ?? false

export const alertCreatedAt = (item: Alert) => item.createdAt ?? item.created_at

export const statLowestPrice = (stats: DashboardStats) => stats.lowestPrice ?? stats.lowest_price ?? null
export const statSubscriptionCount = (stats: DashboardStats) => stats.totalSubscriptions ?? stats.subscriptionCount ?? stats.subscription_count ?? 0
export const statConnectedCount = (stats: DashboardStats) => stats.connected ?? stats.connectedCount ?? stats.connected_count ?? 0
export const statActionRequired = (stats: DashboardStats) => stats.actionRequired ?? stats.action_required ?? ((stats.expired ?? 0) + (stats.unchecked ?? 0))

export const settingsPriceThreshold = (settings: Settings) => settings.threshold ?? settings.priceThreshold ?? settings.price_threshold ?? 0
export const settingsRefreshInterval = (settings: Settings) => settings.refreshMinutes ?? settings.refreshInterval ?? settings.refresh_interval ?? 1
export const settingsAccountPollInterval = (settings: Settings) => settings.accountPollMinutes ?? settings.account_poll_minutes ?? 5
export const settingsBaseUrl = (settings: Settings) => String(settings.baseUrl ?? settings.base_url ?? '')
export const settingsWebhookUrl = (settings: Settings) => String(settings.webhookUrl ?? settings.webhook_url ?? '')
export const settingsAllowRemote = (settings: Settings) => settings.allowRemoteBaseUrl ?? settings.allow_remote_base_url ?? false
export const settingsCpaAuthDir = (settings: Settings) => String(settings.cpaAuthDir ?? settings.cpa_auth_dir ?? '')
export const settingsSyncToCpa = (settings: Settings) => settings.syncToCpaAuthDir ?? settings.sync_to_cpa_auth_dir ?? false
