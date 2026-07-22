package sub2api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
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

type clientError struct {
	Kind       string
	HTTPStatus int
}

func newClientError(kind string, status int) *clientError {
	return &clientError{Kind: kind, HTTPStatus: status}
}

func (e *clientError) Error() string {
	switch e.Kind {
	case "account_not_found":
		return "Sub2API account was not found"
	case "account_test_failed":
		return "Sub2API account test failed"
	case "auth":
		return "Sub2API authorization failed"
	case "transport":
		return "Sub2API request failed"
	case "read":
		return "read Sub2API response failed"
	case "oversize":
		return "Sub2API response exceeded the size limit"
	default:
		return "Sub2API returned an invalid response"
	}
}

type accountTestEvent struct {
	Type    string `json:"type"`
	Success bool   `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
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
	if imported.Total <= 0 || imported.Created < 0 || imported.Updated < 0 || imported.Skipped < 0 || imported.Failed < 0 || imported.Created+imported.Updated+imported.Skipped+imported.Failed != imported.Total || len(imported.Items) != imported.Total {
		return gateways.DeploymentResult{}, importError("invalid_import_result", "Sub2API returned an invalid import result", false, 0)
	}
	accountID := int64(0)
	action := ""
	for _, item := range imported.Items {
		switch item.Action {
		case "created", "updated", "skipped":
			if item.AccountID <= 0 || accountID != 0 {
				return gateways.DeploymentResult{}, importError("invalid_import_result", "Sub2API returned an ambiguous import result", false, 0)
			}
			accountID, action = item.AccountID, item.Action
		case "failed":
			if item.AccountID != 0 {
				return gateways.DeploymentResult{}, importError("import_failed", "Sub2API rejected the credential import", true, 0)
			}
		default:
			return gateways.DeploymentResult{}, importError("invalid_import_result", "Sub2API returned an unknown import action", false, 0)
		}
	}
	if imported.Failed > 0 || accountID <= 0 {
		return gateways.DeploymentResult{}, importError("import_failed", "Sub2API credential import failed", true, 0)
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
		var safe *clientError
		if errors.As(err, &safe) {
			switch safe.Kind {
			case "account_not_found":
				return gateways.InspectResult{Connectivity: model.Connectivity{Status: "not_found", ReasonCode: "sub2api_account_not_found", HTTPStatus: safe.HTTPStatus, CheckedAt: checkedAt, Error: "Sub2API 账号不存在"}}, nil
			case "account_test_failed":
				return gateways.InspectResult{Connectivity: model.Connectivity{Status: "error", ReasonCode: "sub2api_account_test_failed", CheckedAt: checkedAt, Error: "Sub2API 账号连通性测试失败"}}, nil
			case "auth":
				return gateways.InspectResult{Connectivity: model.Connectivity{Status: "sub2api_unavailable", ReasonCode: "sub2api_auth_failed", HTTPStatus: safe.HTTPStatus, CheckedAt: checkedAt, Error: "Sub2API 管理密钥无效"}}, nil
			}
		}
		return gateways.InspectResult{Connectivity: model.Connectivity{Status: "sub2api_unavailable", ReasonCode: "account_test_unavailable", CheckedAt: checkedAt, Error: "Sub2API 账号测试不可用"}}, nil
	}
	check := model.Connectivity{Status: "ok", LatencyMS: test.LatencyMS, CheckedAt: checkedAt}
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

func (c *Client) ReconcileBinding(ctx context.Context, binding gateways.BindingRef, credential gateways.Credential) (gateways.BindingReconciliation, error) {
	oldID, err := parseAccountID(binding.ExternalID)
	if err != nil {
		return gateways.BindingReconciliation{}, err
	}
	page, err := c.ListAccounts(ctx, 1, 500, "")
	if err != nil {
		return gateways.BindingReconciliation{}, err
	}
	if page.Pages > 1 || page.Total > len(page.Items) {
		return gateways.BindingReconciliation{Outcome: "ambiguous", Binding: binding}, nil
	}
	var matches []Account
	for _, account := range page.Items {
		if account.ID == oldID || !strings.EqualFold(account.Platform, credential.Provider) || !strings.EqualFold(account.Type, "oauth") {
			continue
		}
		if strings.TrimSpace(account.Credentials.OrbitSubscriptionID) != credential.SubscriptionID {
			continue
		}
		if credentialMatchesAccount(credential.CredentialSet, account.Credentials) {
			matches = append(matches, account)
		}
	}
	if len(matches) != 1 {
		outcome := "missing"
		if len(matches) > 1 {
			outcome = "ambiguous"
		}
		return gateways.BindingReconciliation{Outcome: outcome, Binding: binding}, nil
	}
	binding.ExternalID = strconv.FormatInt(matches[0].ID, 10)
	binding.ExternalRef = "account:" + binding.ExternalID
	return gateways.BindingReconciliation{Outcome: "rebound", Binding: binding}, nil
}

func credentialMatchesAccount(values map[string]string, remote AccountCredentials) bool {
	remoteValues := map[string]string{
		"account_id": remote.AccountID, "chatgpt_account_id": remote.ChatGPTAccountID,
		"chatgpt_user_id": remote.ChatGPTUserID, "agent_runtime_id": remote.AgentRuntimeID,
	}
	matched := false
	for key, remoteValue := range remoteValues {
		localValue := strings.ToLower(strings.TrimSpace(values[key]))
		remoteValue = strings.ToLower(strings.TrimSpace(remoteValue))
		if localValue == "" || remoteValue == "" {
			continue
		}
		if localValue != remoteValue {
			return false
		}
		matched = true
	}
	return matched
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
	started := time.Now()
	contentType, data, err := c.doRaw(ctx, http.MethodPost, "/accounts/"+strconv.FormatInt(id, 10)+"/test", nil, map[string]any{}, false, "application/json, text/event-stream")
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return AccountTestResult{}, err
	}
	mediaType := "application/json"
	if strings.TrimSpace(contentType) != "" {
		parsedType, _, parseErr := mime.ParseMediaType(contentType)
		if parseErr != nil {
			return AccountTestResult{}, newClientError("invalid_response", 0)
		}
		mediaType = parsedType
	}
	var result AccountTestResult
	switch strings.ToLower(mediaType) {
	case "text/event-stream":
		result, err = parseAccountTestSSE(data)
	case "application/json", "text/json", "":
		err = decodeResponse(data, &result)
	case "text/plain":
		if !json.Valid(bytes.TrimSpace(data)) {
			err = newClientError("invalid_response", 0)
		} else {
			err = decodeResponse(data, &result)
		}
	default:
		err = newClientError("invalid_response", 0)
	}
	if err != nil {
		return AccountTestResult{}, err
	}
	if result.LatencyMS <= 0 {
		result.LatencyMS = latency
	}
	if !result.Success {
		return result, newClientError("account_test_failed", 0)
	}
	return result, nil
}

func parseAccountTestSSE(data []byte) (AccountTestResult, error) {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	terminal := false
	for _, record := range bytes.Split(normalized, []byte("\n\n")) {
		var payloadLines [][]byte
		for _, line := range bytes.Split(record, []byte("\n")) {
			if len(line) == 0 || line[0] == ':' {
				continue
			}
			if bytes.HasPrefix(line, []byte("data:")) {
				value := line[len("data:"):]
				if len(value) > 0 && value[0] == ' ' {
					value = value[1:]
				}
				payloadLines = append(payloadLines, value)
			}
		}
		if len(payloadLines) == 0 {
			continue
		}
		var event accountTestEvent
		if err := json.Unmarshal(bytes.Join(payloadLines, []byte("\n")), &event); err != nil {
			return AccountTestResult{}, newClientError("invalid_response", http.StatusBadGateway)
		}
		switch strings.ToLower(strings.TrimSpace(event.Type)) {
		case "test_complete":
			if terminal || !event.Success {
				return AccountTestResult{}, newClientError("invalid_response", http.StatusBadGateway)
			}
			terminal = true
		case "error":
			if terminal {
				return AccountTestResult{}, newClientError("invalid_response", http.StatusBadGateway)
			}
			if strings.EqualFold(strings.Join(strings.Fields(event.Error), " "), "Account not found") {
				return AccountTestResult{}, newClientError("account_not_found", http.StatusNotFound)
			}
			return AccountTestResult{}, newClientError("account_test_failed", 0)
		}
	}
	if !terminal {
		return AccountTestResult{}, newClientError("invalid_response", http.StatusBadGateway)
	}
	return AccountTestResult{Success: true}, nil
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
	_, data, err := c.doRaw(ctx, method, path, query, requestBody, importRequest, "application/json")
	if err != nil {
		return err
	}
	return decodeResponse(data, responseBody)
}

func (c *Client) doRaw(ctx context.Context, method, path string, query url.Values, requestBody any, importRequest bool, accept string) (string, []byte, error) {
	cfg := c.config()
	endpoint, err := adminEndpoint(cfg.BaseURL, path, query)
	if err != nil {
		return "", nil, err
	}
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return "", nil, fmt.Errorf("encode Sub2API request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return "", nil, errors.New("build Sub2API request failed")
	}
	if strings.TrimSpace(cfg.AdminKey) != "" {
		req.Header.Set("x-api-key", cfg.AdminKey)
	}
	if strings.TrimSpace(cfg.BearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.BearerToken)
	}
	req.Header.Set("Accept", accept)
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
		if importRequest {
			return "", nil, &gateways.DeploymentError{Code: "sub2api_transport_uncertain", Message: "Sub2API import result is uncertain because the connection failed", Outcome: gateways.DeploymentUncertain, Retryable: true, HTTPStatus: http.StatusGatewayTimeout}
		}
		return "", nil, newClientError("transport", http.StatusGatewayTimeout)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		if importRequest {
			return "", nil, &gateways.DeploymentError{Code: "sub2api_transport_uncertain", Message: "Sub2API import result is uncertain because the response was interrupted", Outcome: gateways.DeploymentUncertain, Retryable: true, HTTPStatus: http.StatusGatewayTimeout}
		}
		return "", nil, newClientError("read", http.StatusBadGateway)
	}
	if len(data) > maxResponseBytes {
		return "", nil, newClientError("oversize", http.StatusBadGateway)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if importRequest {
			code, message, retryable := "sub2api_import_failed", "Sub2API rejected the credential import", false
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				code, message = "sub2api_auth_failed", "Sub2API authorization failed"
			} else if resp.StatusCode == http.StatusUnprocessableEntity {
				code, message = "sub2api_import_invalid", "Sub2API rejected the credential format"
			} else if resp.StatusCode >= 500 {
				code, message, retryable = "sub2api_upstream_failed", "Sub2API is temporarily unavailable", true
			}
			return "", nil, &gateways.DeploymentError{Code: code, Message: message, Outcome: gateways.DeploymentFailed, Retryable: retryable, HTTPStatus: resp.StatusCode}
		}
		kind := "http"
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			kind = "auth"
		}
		return "", nil, newClientError(kind, resp.StatusCode)
	}
	return resp.Header.Get("Content-Type"), data, nil
}

func decodeResponse(data []byte, responseBody any) error {
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
			return newClientError("application", http.StatusBadGateway)
		}
		if len(bytes.TrimSpace(envelope.Data)) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
			return nil
		}
		data = envelope.Data
	}
	if err := json.Unmarshal(data, responseBody); err != nil {
		return newClientError("invalid_response", http.StatusBadGateway)
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

func importError(code, message string, retryable bool, status int) error {
	return &gateways.DeploymentError{Code: code, Message: message, Outcome: gateways.DeploymentFailed, Retryable: retryable, HTTPStatus: status}
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
