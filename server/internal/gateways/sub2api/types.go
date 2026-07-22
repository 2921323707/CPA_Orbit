package sub2api

import (
	"encoding/json"
	"time"
)

type Config struct {
	BaseURL       string
	AdminKey      string
	BearerToken   string
	Timeout       time.Duration
	ImportTimeout time.Duration
}

type CodexSessionImportRequest struct {
	Content                 string         `json:"content,omitempty"`
	Contents                []string       `json:"contents,omitempty"`
	Name                    string         `json:"name,omitempty"`
	Notes                   *string        `json:"notes,omitempty"`
	GroupIDs                []int64        `json:"group_ids,omitempty"`
	ProxyID                 *int64         `json:"proxy_id,omitempty"`
	Concurrency             int            `json:"concurrency,omitempty"`
	Priority                int            `json:"priority,omitempty"`
	RateMultiplier          float64        `json:"rate_multiplier,omitempty"`
	LoadFactor              *float64       `json:"load_factor,omitempty"`
	ExpiresAt               *int64         `json:"expires_at,omitempty"`
	AutoPauseOnExpired      bool           `json:"auto_pause_on_expired,omitempty"`
	CredentialExtras        map[string]any `json:"credential_extras,omitempty"`
	Extra                   map[string]any `json:"extra,omitempty"`
	UpdateExisting          bool           `json:"update_existing,omitempty"`
	SkipDefaultGroupBind    bool           `json:"skip_default_group_bind,omitempty"`
	ConfirmMixedChannelRisk bool           `json:"confirm_mixed_channel_risk,omitempty"`
}

type CodexSessionImportMessage struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type CodexSessionImportItem struct {
	Index     int    `json:"index"`
	Name      string `json:"name,omitempty"`
	Action    string `json:"action"`
	AccountID int64  `json:"account_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type CodexSessionImportResult struct {
	Total    int                         `json:"total"`
	Created  int                         `json:"created"`
	Updated  int                         `json:"updated"`
	Skipped  int                         `json:"skipped"`
	Failed   int                         `json:"failed"`
	Items    []CodexSessionImportItem    `json:"items,omitempty"`
	Warnings []CodexSessionImportMessage `json:"warnings,omitempty"`
	Errors   []CodexSessionImportMessage `json:"errors,omitempty"`
}

type Account struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Notes       string          `json:"notes,omitempty"`
	Platform    string          `json:"platform"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	Email       string          `json:"email,omitempty"`
	Concurrency int             `json:"concurrency,omitempty"`
	Priority    int             `json:"priority,omitempty"`
	Groups      json.RawMessage `json:"groups,omitempty"`
}

type AccountPage struct {
	Items    []Account `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Pages    int       `json:"pages"`
}

type AccountTestResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

type OpenAIQuotaUsage struct {
	UserID    string          `json:"user_id,omitempty"`
	AccountID string          `json:"account_id,omitempty"`
	Email     string          `json:"email,omitempty"`
	PlanType  string          `json:"plan_type,omitempty"`
	RateLimit json.RawMessage `json:"rate_limit,omitempty"`
	FetchedAt int64           `json:"fetched_at"`
}

type UsagePage struct {
	Items    []json.RawMessage `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Pages    int               `json:"pages"`
}

type Snapshot map[string]any
