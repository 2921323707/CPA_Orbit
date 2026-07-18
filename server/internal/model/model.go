package model

import "time"

type Offer struct {
	Status    string  `json:"status"`
	Stock     string  `json:"stock"`
	Merchant  string  `json:"merchant"`
	ShopID    string  `json:"shopId"`
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	UpdatedAt string  `json:"updatedAt"`
	OrderURL  string  `json:"orderUrl"`
	ItemID    string  `json:"itemId"`
}

type OffersFile struct {
	Offers    []Offer   `json:"offers"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type OfferFeed struct {
	Offers    []Offer   `json:"offers"`
	UpdatedAt time.Time `json:"updatedAt"`
	SourceURL string    `json:"sourceUrl"`
	LastError string    `json:"lastError,omitempty"`
}

type PriceSample struct {
	At      time.Time `json:"at"`
	Average float64   `json:"average"`
}

type Alert struct {
	ID            string    `json:"id"`
	ItemID        string    `json:"itemId"`
	Title         string    `json:"title"`
	Price         float64   `json:"price"`
	Threshold     float64   `json:"threshold"`
	Merchant      string    `json:"merchant"`
	OrderURL      string    `json:"orderUrl"`
	CreatedAt     time.Time `json:"createdAt"`
	WebhookSent   bool      `json:"webhookSent"`
	WebhookError  string    `json:"webhookError,omitempty"`
	WebhookSentAt time.Time `json:"webhookSentAt,omitempty"`
}

type QuotaWindow struct {
	UsedPercent        *float64  `json:"usedPercent,omitempty"`
	RemainingPercent   *float64  `json:"remainingPercent,omitempty"`
	LimitWindowSeconds int64     `json:"limitWindowSeconds,omitempty"`
	ResetAfterSeconds  int64     `json:"resetAfterSeconds,omitempty"`
	ResetAt            time.Time `json:"resetAt,omitempty"`
}

type UsageQuota struct {
	PlanType       string       `json:"planType,omitempty"`
	Allowed        *bool        `json:"allowed,omitempty"`
	LimitReached   bool         `json:"limitReached"`
	FiveHour       *QuotaWindow `json:"fiveHour,omitempty"`
	SevenDay       *QuotaWindow `json:"sevenDay,omitempty"`
	CreditsBalance *float64     `json:"creditsBalance,omitempty"`
	Credits        bool         `json:"hasCredits"`
	Unlimited      bool         `json:"unlimited"`
}

type Connectivity struct {
	Status           string      `json:"status"`
	ReasonCode       string      `json:"reasonCode,omitempty"`
	HTTPStatus       int         `json:"httpStatus,omitempty"`
	LatencyMS        int64       `json:"latencyMs"`
	CheckedAt        time.Time   `json:"checkedAt,omitempty"`
	Error            string      `json:"error,omitempty"`
	CPAStatus        string      `json:"cpaStatus,omitempty"`
	CPAStatusMessage string      `json:"cpaStatusMessage,omitempty"`
	CPAUnavailable   bool        `json:"cpaUnavailable"`
	NextRetryAt      time.Time   `json:"nextRetryAt,omitempty"`
	Quota            *UsageQuota `json:"quota,omitempty"`
}

type Subscription struct {
	ID               string       `json:"id"`
	FileName         string       `json:"fileName"`
	Folder           string       `json:"folder"`
	RelativePath     string       `json:"relativePath"`
	Email            string       `json:"email"`
	Name             string       `json:"name"`
	AccountID        string       `json:"accountId"`
	ChatGPTAccountID string       `json:"chatgptAccountId"`
	PlanType         string       `json:"planType"`
	ChatGPTPlanType  string       `json:"chatgptPlanType"`
	Expired          string       `json:"expired"`
	LastRefresh      string       `json:"lastRefresh"`
	Type             string       `json:"type"`
	Provider         string       `json:"provider"`
	BaseURL          string       `json:"baseUrl"`
	OrderURL         string       `json:"orderUrl"`
	Balance          *float64     `json:"balance"`
	AcquisitionPrice *float64     `json:"acquisitionPrice,omitempty"`
	RemainingDays    *int         `json:"remainingDays"`
	Status           string       `json:"status"`
	Category         string       `json:"category"`
	Connectivity     Connectivity `json:"connectivity"`
}
