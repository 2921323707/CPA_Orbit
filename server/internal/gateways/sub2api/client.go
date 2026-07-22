package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/model"
)

const maxResponseBytes = 8 << 20

type Client struct {
	config func() Config
}

func NewClient(configProvider func() Config) *Client {
	return &Client{config: configProvider}
}

func (c *Client) Kind() gateways.Kind { return gateways.KindSub2API }

func (c *Client) Health(ctx context.Context) (gateways.Health, error) {
	checkedAt := time.Now()
	started := time.Now()
	_, err := c.ListAccounts(ctx, 1, 1, "")
	health := gateways.Health{Status: "ok", CheckedAt: checkedAt, LatencyMS: time.Since(started).Milliseconds()}
	if err != nil {
		health.Status = "unavailable"
		health.Message = "Sub2API 管理接口不可用"
	}
	return health, nil
}

func (c *Client) Deploy(ctx context.Context, credential gateways.Credential, options gateways.DeployOptions) (gateways.DeploymentResult, error) {
	if len(bytes.TrimSpace(credential.Data)) == 0 {
		return gateways.DeploymentResult{}, errors.New("Sub2API deployment requires a credential body")
	}
	content := credential.Data
	lookupEmail := credential.Email
	bundleAccount, isBundle, err := parseSub2APIDataAccount(credential.Data)
	if err != nil {
		return gateways.DeploymentResult{}, err
	}
	if isBundle {
		content = bundleAccount.Credentials
		lookupEmail = firstNonEmpty(bundleAccount.Email, lookupEmail)
		if strings.TrimSpace(options.Name) == "" {
			options.Name = bundleAccount.Name
		}
		if options.Concurrency == 0 {
			options.Concurrency = bundleAccount.Concurrency
		}
		if options.Priority == 0 {
			options.Priority = bundleAccount.Priority
		}
		if options.RateMultiplier == 0 {
			options.RateMultiplier = bundleAccount.RateMultiplier
		}
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = firstNonEmpty(lookupEmail, credential.SubscriptionID, "Orbit Codex account")
	}
	request := CodexSessionImportRequest{
		Content:          codexSessionContent(content),
		Name:             name,
		GroupIDs:         append([]int64(nil), options.GroupIDs...),
		Concurrency:      options.Concurrency,
		Priority:         options.Priority,
		RateMultiplier:   options.RateMultiplier,
		UpdateExisting:   options.UpdateExisting,
		CredentialExtras: map[string]any{"orbit_subscription_id": credential.SubscriptionID},
	}
	if isBundle {
		request.AutoPauseOnExpired = bundleAccount.AutoPauseOnExpired
		request.Extra = bundleAccount.Extra
	}
	if notes := strings.TrimSpace(options.Notes); notes != "" {
		request.Notes = &notes
	}
	var imported CodexSessionImportResult
	if err := c.do(ctx, http.MethodPost, "/accounts/import/codex-session", nil, request, &imported, true); err != nil {
		return gateways.DeploymentResult{}, err
	}
	accountID := int64(0)
	action := ""
	for _, item := range imported.Items {
		if item.AccountID > 0 && item.Action != "failed" {
			accountID, action = item.AccountID, item.Action
			break
		}
	}
	if accountID == 0 && lookupEmail != "" {
		page, err := c.ListAccounts(ctx, 1, 20, lookupEmail)
		if err == nil {
			for _, account := range page.Items {
				if strings.EqualFold(strings.TrimSpace(account.Email), strings.TrimSpace(lookupEmail)) || strings.EqualFold(strings.TrimSpace(account.Name), strings.TrimSpace(lookupEmail)) {
					accountID = account.ID
					break
				}
			}
		}
	}
	if imported.Failed > 0 || accountID <= 0 {
		return gateways.DeploymentResult{}, fmt.Errorf("Sub2API import failed (%d failed, %d skipped)", imported.Failed, imported.Skipped)
	}
	status := "deployed"
	if action == "updated" {
		status = "updated"
	} else if action == "skipped" {
		status = "adopted"
	}
	id := strconv.FormatInt(accountID, 10)
	return gateways.DeploymentResult{Binding: gateways.BindingRef{ExternalID: id, ExternalRef: "account:" + id, Managed: true}, Status: status}, nil
}

type sub2APIDataAccount struct {
	Name               string          `json:"name"`
	Platform           string          `json:"platform"`
	Type               string          `json:"type"`
	Credentials        json.RawMessage `json:"credentials"`
	Extra              map[string]any  `json:"extra"`
	Concurrency        int             `json:"concurrency"`
	Priority           int             `json:"priority"`
	RateMultiplier     float64         `json:"rate_multiplier"`
	AutoPauseOnExpired bool            `json:"auto_pause_on_expired"`
	Email              string          `json:"-"`
}

func parseSub2APIDataAccount(data []byte) (sub2APIDataAccount, bool, error) {
	if !gateways.IsSub2APIDataPackage(data) {
		return sub2APIDataAccount{}, false, nil
	}
	var envelope struct {
		Type     string               `json:"type"`
		Accounts []sub2APIDataAccount `json:"accounts"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return sub2APIDataAccount{}, false, fmt.Errorf("invalid credential JSON: %w", err)
	}
	if len(envelope.Accounts) != 1 {
		return sub2APIDataAccount{}, true, fmt.Errorf("Sub2API data import must contain exactly one account; got %d", len(envelope.Accounts))
	}
	account := envelope.Accounts[0]
	if !strings.EqualFold(strings.TrimSpace(account.Platform), "openai") || !strings.EqualFold(strings.TrimSpace(account.Type), "oauth") {
		return sub2APIDataAccount{}, true, errors.New("Sub2API data account must be an OpenAI OAuth account")
	}
	var credentials map[string]any
	if err := json.Unmarshal(account.Credentials, &credentials); err != nil || len(credentials) == 0 {
		return sub2APIDataAccount{}, true, errors.New("Sub2API data account is missing credentials")
	}
	account.Email = firstNonEmpty(stringMapValue(credentials, "email"), stringMapValue(account.Extra, "email"), account.Name)
	return account, true, nil
}

func stringMapValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// codexSessionContent removes client-routing metadata before handing a Codex
// credential to Sub2API. The archived source remains untouched, while a
// legacy CPA base_url can never become an implicit Sub2API routing setting.
func codexSessionContent(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return string(data)
	}
	changed := false
	removeRoutingField := func(object map[string]any) {
		for key := range object {
			normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
			if normalized == "baseurl" {
				delete(object, key)
				changed = true
			}
		}
	}
	switch typed := value.(type) {
	case map[string]any:
		removeRoutingField(typed)
	case []any:
		for _, item := range typed {
			if object, ok := item.(map[string]any); ok {
				removeRoutingField(object)
			}
		}
	}
	if !changed {
		return string(data)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return string(data)
	}
	return string(encoded)
}

func (c *Client) Inspect(ctx context.Context, binding gateways.BindingRef, _ gateways.Credential) (gateways.InspectResult, error) {
	id, err := parseAccountID(binding.ExternalID)
	if err != nil {
		return gateways.InspectResult{}, err
	}
	checkedAt := time.Now()
	test, err := c.TestAccount(ctx, id)
	if err != nil {
		return gateways.InspectResult{Connectivity: model.Connectivity{Status: "sub2api_unavailable", ReasonCode: "account_test_unavailable", CheckedAt: checkedAt, Error: "Sub2API 账号测试不可用"}}, nil
	}
	check := model.Connectivity{Status: "ok", LatencyMS: test.LatencyMS, CheckedAt: checkedAt}
	if !test.Success {
		check.Status = "error"
		check.ReasonCode = "sub2api_account_test_failed"
		check.Error = "Sub2API 账号连通性测试失败"
		return gateways.InspectResult{Connectivity: check}, nil
	}
	quota, err := c.QueryOpenAIQuota(ctx, id)
	if err == nil {
		check.Quota = normalizeQuota(quota, checkedAt)
		if check.Quota != nil && (check.Quota.LimitReached || quotaExhausted(check.Quota.FiveHour) || quotaExhausted(check.Quota.SevenDay)) {
			check.Status = "quota_exhausted"
			check.ReasonCode = "usage_limit_reached"
			check.Error = "额度已耗尽，等待窗口重置"
		}
	}
	return gateways.InspectResult{Connectivity: check}, nil
}

func (c *Client) Detach(ctx context.Context, binding gateways.BindingRef, _ gateways.Credential) error {
	if !binding.Managed {
		return nil
	}
	id, err := parseAccountID(binding.ExternalID)
	if err != nil {
		return err
	}
	var response map[string]any
	return c.do(ctx, http.MethodDelete, "/accounts/"+strconv.FormatInt(id, 10), nil, nil, &response, false)
}

func (c *Client) ListAccounts(ctx context.Context, page, pageSize int, search string) (AccountPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 20
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if search = strings.TrimSpace(search); search != "" {
		query.Set("search", search)
	}
	var result AccountPage
	err := c.do(ctx, http.MethodGet, "/accounts", query, nil, &result, false)
	return result, err
}

func (c *Client) TestAccount(ctx context.Context, id int64) (AccountTestResult, error) {
	var result AccountTestResult
	err := c.do(ctx, http.MethodPost, "/accounts/"+strconv.FormatInt(id, 10)+"/test", nil, map[string]any{}, &result, false)
	return result, err
}

func (c *Client) QueryOpenAIQuota(ctx context.Context, id int64) (OpenAIQuotaUsage, error) {
	var result OpenAIQuotaUsage
	err := c.do(ctx, http.MethodGet, "/openai/accounts/"+strconv.FormatInt(id, 10)+"/quota", nil, nil, &result, false)
	return result, err
}

func (c *Client) AccountUsage(ctx context.Context, id int64, force bool) (Snapshot, error) {
	query := url.Values{}
	if force {
		query.Set("source", "active")
		query.Set("force", "true")
	}
	var result Snapshot
	err := c.do(ctx, http.MethodGet, "/accounts/"+strconv.FormatInt(id, 10)+"/usage", query, nil, &result, false)
	return result, err
}

func (c *Client) DashboardSnapshot(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	err := c.do(ctx, http.MethodGet, "/dashboard/snapshot-v2", nil, nil, &result, false)
	return result, err
}

func (c *Client) AccountAvailability(ctx context.Context) (Snapshot, error) {
	var result Snapshot
	err := c.do(ctx, http.MethodGet, "/ops/account-availability", nil, nil, &result, false)
	return result, err
}

func (c *Client) Usage(ctx context.Context, page, pageSize int) (UsagePage, error) {
	return c.UsageRange(ctx, page, pageSize, time.Time{}, time.Time{})
}

func (c *Client) UsageRange(ctx context.Context, page, pageSize int, from, to time.Time) (UsagePage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if !from.IsZero() {
		query.Set("start_date", from.UTC().Format("2006-01-02"))
	}
	if !to.IsZero() {
		query.Set("end_date", to.UTC().Format("2006-01-02"))
	}
	var result UsagePage
	err := c.do(ctx, http.MethodGet, "/usage", query, nil, &result, false)
	return result, err
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, requestBody, responseBody any, importRequest bool) error {
	cfg := c.config()
	endpoint, err := adminEndpoint(cfg.BaseURL, path, query)
	if err != nil {
		return err
	}
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode Sub2API request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return errors.New("build Sub2API request failed")
	}
	if strings.TrimSpace(cfg.AdminKey) != "" {
		req.Header.Set("x-api-key", cfg.AdminKey)
	}
	if strings.TrimSpace(cfg.BearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.BearerToken)
	}
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if importRequest {
		timeout = cfg.ImportTimeout
		if timeout <= 0 {
			timeout = 120 * time.Second
		}
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("Sub2API request failed")
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return errors.New("read Sub2API response failed")
	}
	if len(data) > maxResponseBytes {
		return errors.New("Sub2API response exceeded the size limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Sub2API %s returned HTTP %d", safeOperation(path), resp.StatusCode)
	}
	if responseBody == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	var envelope struct {
		Code    *int            `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &envelope) == nil && envelope.Code != nil {
		if *envelope.Code != 0 {
			return fmt.Errorf("Sub2API %s returned an application error", safeOperation(path))
		}
		if len(bytes.TrimSpace(envelope.Data)) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
			return nil
		}
		data = envelope.Data
	}
	if err := json.Unmarshal(data, responseBody); err != nil {
		return fmt.Errorf("decode Sub2API %s response failed", safeOperation(path))
	}
	return nil
}

func adminEndpoint(baseURL, path string, query url.Values) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("Sub2API base URL must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("Sub2API base URL must use HTTP or HTTPS")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("Sub2API base URL cannot contain credentials, query, or fragment")
	}
	cleanPath := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/api/v1/admin", "/api/v1"} {
		if strings.HasSuffix(strings.ToLower(cleanPath), suffix) {
			cleanPath = cleanPath[:len(cleanPath)-len(suffix)]
			break
		}
	}
	parsed.Path = strings.TrimRight(cleanPath, "/") + "/api/v1/admin/" + strings.TrimLeft(path, "/")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func normalizeQuota(raw OpenAIQuotaUsage, checkedAt time.Time) *model.UsageQuota {
	quota := &model.UsageQuota{PlanType: raw.PlanType}
	if len(raw.RateLimit) == 0 || string(raw.RateLimit) == "null" {
		return quota
	}
	var rate map[string]any
	if json.Unmarshal(raw.RateLimit, &rate) != nil {
		return quota
	}
	if allowed, ok := rate["allowed"].(bool); ok {
		quota.Allowed = &allowed
	}
	quota.LimitReached, _ = rate["limit_reached"].(bool)
	quota.FiveHour = normalizeWindow(objectField(rate, "primary_window"), checkedAt)
	quota.SevenDay = normalizeWindow(objectField(rate, "secondary_window"), checkedAt)
	return quota
}

func normalizeWindow(raw map[string]any, checkedAt time.Time) *model.QuotaWindow {
	if raw == nil {
		return nil
	}
	window := &model.QuotaWindow{}
	if used, ok := floatField(raw, "used_percent"); ok {
		used = clamp(used)
		remaining := clamp(100 - used)
		window.UsedPercent, window.RemainingPercent = &used, &remaining
	}
	if seconds, ok := floatField(raw, "limit_window_seconds"); ok {
		window.LimitWindowSeconds = int64(seconds)
	}
	if seconds, ok := floatField(raw, "reset_after_seconds"); ok {
		window.ResetAfterSeconds = int64(seconds)
		window.ResetAt = checkedAt.Add(time.Duration(seconds) * time.Second)
	}
	if reset, ok := floatField(raw, "reset_at"); ok && reset > 0 {
		if reset > 1e12 {
			reset /= 1000
		}
		window.ResetAt = time.Unix(int64(reset), 0).UTC()
	}
	return window
}

func objectField(raw map[string]any, key string) map[string]any {
	value, _ := raw[key].(map[string]any)
	return value
}

func floatField(raw map[string]any, key string) (float64, bool) {
	switch value := raw[key].(type) {
	case float64:
		return value, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func quotaExhausted(window *model.QuotaWindow) bool {
	return window != nil && ((window.UsedPercent != nil && *window.UsedPercent >= 100) || (window.RemainingPercent != nil && *window.RemainingPercent <= 0))
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func parseAccountID(value string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("Sub2API account binding is invalid")
	}
	return id, nil
}

func safeOperation(path string) string {
	if strings.Contains(path, "import/codex-session") {
		return "credential import"
	}
	return "admin API"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
