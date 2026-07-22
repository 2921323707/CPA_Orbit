package cpa

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/storage"
)

const codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

var unsafeNameRE = regexp.MustCompile(`[^A-Za-z0-9._+-]+`)

type Config struct {
	BaseURL       string
	ManagementKey string
	AuthDir       string
	SyncEnabled   bool
}

type AuthEntry struct {
	ID             string `json:"id"`
	AuthIndex      string `json:"auth_index"`
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	Email          string `json:"email"`
	Account        string `json:"account"`
	Status         string `json:"status"`
	StatusMessage  string `json:"status_message"`
	Disabled       bool   `json:"disabled"`
	Unavailable    bool   `json:"unavailable"`
	RuntimeOnly    bool   `json:"runtime_only"`
	NextRetryAfter string `json:"next_retry_after"`
}

type authList struct {
	Files []AuthEntry `json:"files"`
}

type apiCallResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

type managedEntry struct {
	SubscriptionID string `json:"subscriptionId"`
	Path           string `json:"path"`
	Fingerprint    string `json:"fingerprint"`
}

type managedManifest struct {
	Version int                     `json:"version"`
	Entries map[string]managedEntry `json:"entries"`
}

type authIdentity struct {
	AccountID        string `json:"account_id"`
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	Provider         string `json:"provider"`
	Type             string `json:"type"`
}

type Client struct {
	config       func() Config
	manifestPath string
	mu           sync.Mutex
	lastCheck    time.Time
	authCache    []AuthEntry
	authCacheAt  time.Time
}

func NewClient(configProvider func() Config, manifestPath string) *Client {
	return &Client{config: configProvider, manifestPath: manifestPath}
}

func (c *Client) Kind() gateways.Kind { return gateways.KindCPA }

func (c *Client) Health(ctx context.Context) (gateways.Health, error) {
	checkedAt := time.Now()
	started := time.Now()
	_, err := c.authFiles(ctx, c.config())
	health := gateways.Health{CheckedAt: checkedAt, LatencyMS: time.Since(started).Milliseconds(), Status: "ok"}
	if err != nil {
		health.Status = "unavailable"
		health.Message = cleanError(err.Error())
	}
	return health, nil
}

func (c *Client) Deploy(ctx context.Context, credential gateways.Credential, _ gateways.DeployOptions) (gateways.DeploymentResult, error) {
	if err := ctx.Err(); err != nil {
		return gateways.DeploymentResult{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg := c.config()
	if !cfg.SyncEnabled {
		return gateways.DeploymentResult{}, errors.New("CPA auth-dir sync is disabled")
	}
	if err := config.ValidateCPAAuthDir(cfg.AuthDir); err != nil {
		return gateways.DeploymentResult{}, err
	}
	manifest, err := c.loadManifest()
	if err != nil {
		return gateways.DeploymentResult{}, err
	}
	target, fingerprint, err := syncBytesToAuthDir(credential.Data, credential.Email, cfg.AuthDir)
	if err != nil {
		return gateways.DeploymentResult{}, err
	}
	key := strings.TrimSpace(credential.SubscriptionID)
	if key == "" {
		key = fingerprint
	}
	manifest.Entries[key] = managedEntry{SubscriptionID: credential.SubscriptionID, Path: target, Fingerprint: fingerprint}
	if err := c.saveManifest(manifest); err != nil {
		return gateways.DeploymentResult{}, fmt.Errorf("save CPA managed-file manifest: %w", err)
	}
	c.invalidateAuthCacheLocked()
	return gateways.DeploymentResult{Binding: gateways.BindingRef{ExternalID: filepath.Base(target), ExternalRef: target, Managed: true}, Status: "deployed"}, nil
}

// Reconcile projects the supplied archive credentials and prunes only entries
// previously recorded in Orbit's managed-file manifest.
func (c *Client) Reconcile(ctx context.Context, credentials []gateways.Credential) error {
	for _, credential := range credentials {
		if _, err := c.Deploy(ctx, credential, gateways.DeployOptions{UpdateExisting: true}); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg := c.config()
	if !cfg.SyncEnabled {
		return nil
	}
	if err := config.ValidateCPAAuthDir(cfg.AuthDir); err != nil {
		return err
	}
	manifest, err := c.loadManifest()
	if err != nil {
		return err
	}
	expected := make(map[string]struct{}, len(credentials))
	for _, credential := range credentials {
		key := strings.TrimSpace(credential.SubscriptionID)
		if key == "" {
			if fingerprint, fingerprintErr := jsonFingerprint(credential.Data); fingerprintErr == nil {
				key = fingerprint
			}
		}
		expected[key] = struct{}{}
	}
	for key, entry := range manifest.Entries {
		if _, ok := expected[key]; ok {
			continue
		}
		if err := removeManagedFile(cfg.AuthDir, entry); err != nil {
			return err
		}
		delete(manifest.Entries, key)
	}
	if err := c.saveManifest(manifest); err != nil {
		return err
	}
	c.invalidateAuthCacheLocked()
	return nil
}

func (c *Client) Detach(ctx context.Context, binding gateways.BindingRef, credential gateways.Credential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cfg := c.config()
	if config.ValidateCPAAuthDir(cfg.AuthDir) != nil {
		return nil
	}
	manifest, err := c.loadManifest()
	if err != nil {
		return err
	}
	key := strings.TrimSpace(credential.SubscriptionID)
	entry, ok := manifest.Entries[key]
	if !ok && binding.ExternalRef != "" {
		for candidateKey, candidate := range manifest.Entries {
			if samePath(candidate.Path, binding.ExternalRef) {
				key, entry, ok = candidateKey, candidate, true
				break
			}
		}
	}
	if !ok {
		return nil
	}
	if binding.ExternalRef != "" && !samePath(entry.Path, binding.ExternalRef) {
		return errors.New("CPA binding does not match managed-file manifest")
	}
	if len(credential.Data) > 0 {
		fingerprint, fingerprintErr := jsonFingerprint(credential.Data)
		if fingerprintErr != nil {
			return fingerprintErr
		}
		if fingerprint != entry.Fingerprint {
			return errors.New("CPA credential does not match managed-file manifest")
		}
	}
	if err := removeManagedFile(cfg.AuthDir, entry); err != nil {
		return err
	}
	delete(manifest.Entries, key)
	if err := c.saveManifest(manifest); err != nil {
		return err
	}
	c.invalidateAuthCacheLocked()
	return nil
}

func (c *Client) Inspect(ctx context.Context, _ gateways.BindingRef, credential gateways.Credential) (gateways.InspectResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := 500*time.Millisecond - time.Since(c.lastCheck); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return gateways.InspectResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	c.lastCheck = time.Now()
	checkedAt := time.Now()
	cfg := c.config()
	if strings.TrimSpace(cfg.ManagementKey) == "" {
		return gateways.InspectResult{Connectivity: model.Connectivity{Status: "configuration_error", ReasonCode: "management_key_missing", CheckedAt: checkedAt, Error: "CPA management key is not configured"}}, nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	authFiles, err := c.authFilesUnlocked(requestCtx, cfg)
	if err != nil {
		return gateways.InspectResult{Connectivity: model.Connectivity{Status: "cpa_unavailable", ReasonCode: "auth_files_unavailable", CheckedAt: checkedAt, Error: cleanError(err.Error())}}, nil
	}
	auth, reason := MatchAuth(authFiles, credential)
	if reason != "" {
		return gateways.InspectResult{Connectivity: model.Connectivity{Status: reason, ReasonCode: reason, CheckedAt: checkedAt, Error: connectivityReasonMessage(reason)}}, nil
	}
	check := model.Connectivity{Status: auth.Status, CheckedAt: checkedAt, CPAStatus: auth.Status, CPAStatusMessage: auth.StatusMessage, CPAUnavailable: auth.Unavailable}
	if retryAt, ok := parseTime(auth.NextRetryAfter); ok {
		check.NextRetryAt = retryAt
	}
	if auth.Disabled || strings.EqualFold(auth.Status, "disabled") {
		check.Status, check.ReasonCode, check.Error = "disabled", "cpa_disabled", "该账号已在 CPA 中禁用"
		return gateways.InspectResult{Connectivity: check}, nil
	}
	provider := normalizeProvider(firstNonEmpty(credential.Provider, auth.Provider))
	if provider != "codex" {
		if strings.TrimSpace(check.Status) == "" {
			check.Status = "ok"
		}
		check.ReasonCode = "provider_status_only"
		return gateways.InspectResult{Connectivity: check}, nil
	}
	accountID := firstNonEmpty(auth.Account, credential.AccountID, credential.ChatGPTAccountID)
	if accountID == "" {
		check.Status, check.ReasonCode, check.Error = "missing_account_id", "missing_account_id", "缺少 ChatGPT account_id，无法安全查询对应工作区额度"
		return gateways.InspectResult{Connectivity: check}, nil
	}
	usageCheck := c.checkUsage(requestCtx, cfg, auth.AuthIndex, accountID, checkedAt)
	usageCheck.CPAStatus, usageCheck.CPAStatusMessage, usageCheck.CPAUnavailable, usageCheck.NextRetryAt = auth.Status, auth.StatusMessage, auth.Unavailable, check.NextRetryAt
	if strings.EqualFold(auth.StatusMessage, "payment_required") && usageCheck.Status == "ok" {
		usageCheck.Status, usageCheck.ReasonCode, usageCheck.Error = "payment_required", "cpa_payment_required", "CPA 最近一次模型调用返回 HTTP 402/403"
	}
	return gateways.InspectResult{Connectivity: usageCheck}, nil
}

func (c *Client) authFiles(ctx context.Context, cfg Config) ([]AuthEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authFilesUnlocked(ctx, cfg)
}

func (c *Client) authFilesUnlocked(ctx context.Context, cfg Config) ([]AuthEntry, error) {
	if len(c.authCache) > 0 && time.Since(c.authCacheAt) < 30*time.Second {
		return append([]AuthEntry(nil), c.authCache...), nil
	}
	if strings.TrimSpace(cfg.ManagementKey) == "" {
		return nil, errors.New("CPA management key is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, managementBaseURL(cfg.BaseURL)+"/auth-files", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Management-Key", cfg.ManagementKey)
	req.Header.Set("Accept", "application/json")
	resp, err := managementHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CPA auth-files returned HTTP %d", resp.StatusCode)
	}
	var payload authList
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode CPA auth-files: %w", err)
	}
	filtered := make([]AuthEntry, 0, len(payload.Files))
	for _, entry := range payload.Files {
		if !entry.RuntimeOnly && strings.TrimSpace(entry.AuthIndex) != "" {
			filtered = append(filtered, entry)
		}
	}
	c.authCache = append(c.authCache[:0], filtered...)
	c.authCacheAt = time.Now()
	return append([]AuthEntry(nil), filtered...), nil
}

func (c *Client) checkUsage(ctx context.Context, cfg Config, authIndex, accountID string, checkedAt time.Time) model.Connectivity {
	body, err := json.Marshal(map[string]any{"auth_index": authIndex, "method": http.MethodGet, "url": codexUsageURL, "header": map[string]string{"Authorization": "Bearer $TOKEN$", "ChatGPT-Account-ID": accountID, "Accept": "application/json", "User-Agent": "codex_cli_rs", "Originator": "codex_cli_rs", "OpenAI-Beta": "codex-1"}})
	if err != nil {
		return model.Connectivity{Status: "error", ReasonCode: "request_encode_failed", CheckedAt: checkedAt, Error: "failed to encode usage request"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, managementBaseURL(cfg.BaseURL)+"/api-call", bytes.NewReader(body))
	if err != nil {
		return model.Connectivity{Status: "error", ReasonCode: "request_build_failed", CheckedAt: checkedAt, Error: "failed to build usage request"}
	}
	req.Header.Set("X-Management-Key", cfg.ManagementKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	started := time.Now()
	resp, err := managementHTTPClient().Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		status, reason := "upstream_error", "management_api_error"
		if ctx.Err() != nil || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			status, reason = "timeout", "usage_timeout"
		}
		return model.Connectivity{Status: status, ReasonCode: reason, LatencyMS: latency, CheckedAt: checkedAt, Error: cleanError(err.Error())}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.Connectivity{Status: "cpa_unavailable", ReasonCode: "management_http_error", HTTPStatus: resp.StatusCode, LatencyMS: latency, CheckedAt: checkedAt, Error: fmt.Sprintf("CPA management API returned HTTP %d", resp.StatusCode)}
	}
	var upstream apiCallResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&upstream); err != nil {
		return model.Connectivity{Status: "invalid_response", ReasonCode: "management_response_invalid", LatencyMS: latency, CheckedAt: checkedAt, Error: "CPA management response is invalid"}
	}
	check := model.Connectivity{Status: statusForUpstream(upstream.StatusCode), ReasonCode: reasonForUpstream(upstream.StatusCode), HTTPStatus: upstream.StatusCode, LatencyMS: latency, CheckedAt: checkedAt}
	if upstream.StatusCode != http.StatusOK {
		check.Error = sanitizedUpstreamError(upstream.Body, upstream.StatusCode)
		return check
	}
	quota, err := ParseUsageQuota(upstream.Body, checkedAt)
	if err != nil {
		check.Status, check.ReasonCode, check.Error = "invalid_response", "usage_response_invalid", err.Error()
		return check
	}
	check.Quota = quota
	if quota.LimitReached || (quota.Allowed != nil && !*quota.Allowed) || quotaWindowExhausted(quota.FiveHour) || quotaWindowExhausted(quota.SevenDay) {
		check.Status, check.ReasonCode, check.Error = "quota_exhausted", "usage_limit_reached", "额度已耗尽，等待窗口重置"
	}
	return check
}

func (c *Client) loadManifest() (managedManifest, error) {
	manifest := managedManifest{Version: 1, Entries: make(map[string]managedEntry)}
	if strings.TrimSpace(c.manifestPath) == "" {
		return manifest, nil
	}
	if err := storage.LoadJSON(c.manifestPath, &manifest); err != nil {
		return managedManifest{}, fmt.Errorf("load CPA managed-file manifest: %w", err)
	}
	if manifest.Entries == nil {
		manifest.Entries = make(map[string]managedEntry)
	}
	manifest.Version = 1
	return manifest, nil
}

func (c *Client) saveManifest(manifest managedManifest) error {
	if strings.TrimSpace(c.manifestPath) == "" {
		return nil
	}
	return storage.SaveJSON(c.manifestPath, manifest)
}

func (c *Client) invalidateAuthCacheLocked() {
	c.authCache = nil
	c.authCacheAt = time.Time{}
}

func removeManagedFile(authDir string, entry managedEntry) error {
	resolvedDir, err := filepath.EvalSymlinks(authDir)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(entry.Path)
	if err != nil || !within(resolvedDir, path) {
		return errors.New("managed CPA path escapes configured auth directory")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("managed CPA path is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fingerprint, err := jsonFingerprint(data)
	if err != nil || fingerprint != entry.Fingerprint {
		// Ownership no longer proves content ownership. Preserve the file.
		return nil
	}
	return os.Remove(path)
}

func syncBytesToAuthDir(data []byte, email, authDir string) (string, string, error) {
	if err := config.ValidateCPAAuthDir(authDir); err != nil {
		return "", "", err
	}
	resolvedDir, err := filepath.EvalSymlinks(authDir)
	if err != nil {
		return "", "", err
	}
	incoming, err := decodeAuthIdentity(data)
	if err != nil {
		return "", "", fmt.Errorf("parse CPA auth identity: %w", err)
	}
	if incoming.Email == "" {
		incoming.Email = strings.TrimSpace(email)
	}
	target, fingerprint, err := matchingAuthTarget(resolvedDir, data)
	if err != nil {
		return "", "", err
	}
	if target == "" {
		namePart := firstNonEmpty(incoming.Email, incoming.AccountID, incoming.ChatGPTAccountID)
		namePart = strings.Trim(unsafeNameRE.ReplaceAllString(namePart, "_"), ". ")
		if namePart == "" {
			return "", "", errors.New("subscription email or account_id is required for CPA auth-dir filename")
		}
		provider := normalizeProvider(incoming.Provider)
		prefix := provider + "_oauth_"
		if provider == "unknown" {
			prefix = "oauth_"
		}
		target = uniquePath(resolvedDir, prefix+namePart+".json")
	}
	if !within(resolvedDir, target) {
		return "", "", errors.New("unsafe CPA auth-dir target path")
	}
	if info, statErr := os.Lstat(target); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", "", errors.New("CPA auth-dir target must be a regular file")
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", "", statErr
	}
	tmp, err := os.CreateTemp(resolvedDir, ".cpa-sync-*.tmp")
	if err != nil {
		return "", "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		return "", "", err
	}
	if err := os.Rename(tmpName, target); err != nil {
		if removeErr := os.Remove(target); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return "", "", err
		}
		if err := os.Rename(tmpName, target); err != nil {
			return "", "", err
		}
	}
	return target, fingerprint, nil
}

// SyncBytesToAuthDir is retained for compatibility tests and one-off migration
// tooling. Production deployment should use Client.Deploy so ownership is recorded.
func SyncBytesToAuthDir(data []byte, email, authDir string) (string, error) {
	target, _, err := syncBytesToAuthDir(data, email, authDir)
	return target, err
}

func matchingAuthTarget(authDir string, incoming []byte) (string, string, error) {
	fingerprint, err := jsonFingerprint(incoming)
	if err != nil {
		return "", "", err
	}
	entries, err := os.ReadDir(authDir)
	if err != nil {
		return "", "", err
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			continue
		}
		path := filepath.Join(authDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		storedFingerprint, err := jsonFingerprint(data)
		if err == nil && storedFingerprint == fingerprint {
			matches = append(matches, path)
		}
	}
	if len(matches) > 1 {
		return "", "", errors.New("multiple CPA auth files contain the same JSON; resolve the duplicate archive")
	}
	if len(matches) == 1 {
		return matches[0], fingerprint, nil
	}
	return "", fingerprint, nil
}

func MatchAuth(entries []AuthEntry, credential gateways.Credential) (AuthEntry, string) {
	ids := nonEmptySet(credential.AccountID, credential.ChatGPTAccountID)
	provider := normalizeProvider(credential.Provider)
	matches := make([]AuthEntry, 0, 2)
	if len(ids) > 0 {
		for _, entry := range entries {
			if provider != "unknown" && normalizeProvider(entry.Provider) != "unknown" && provider != normalizeProvider(entry.Provider) {
				continue
			}
			if _, ok := ids[strings.TrimSpace(entry.Account)]; ok {
				matches = append(matches, entry)
			}
		}
		if len(matches) > 1 && credential.Email != "" {
			email := strings.ToLower(strings.TrimSpace(credential.Email))
			narrowed := matches[:0]
			for _, entry := range matches {
				if strings.ToLower(strings.TrimSpace(entry.Email)) == email {
					narrowed = append(narrowed, entry)
				}
			}
			matches = narrowed
		}
	}
	if len(matches) == 0 && credential.Email != "" {
		email := strings.ToLower(strings.TrimSpace(credential.Email))
		for _, entry := range entries {
			if provider != "unknown" && normalizeProvider(entry.Provider) != "unknown" && provider != normalizeProvider(entry.Provider) {
				continue
			}
			if strings.ToLower(strings.TrimSpace(entry.Email)) == email {
				matches = append(matches, entry)
			}
		}
	}
	if len(matches) == 0 {
		return AuthEntry{}, "not_in_cpa_pool"
	}
	if len(matches) > 1 {
		return AuthEntry{}, "ambiguous_account"
	}
	return matches[0], ""
}

func ParseUsageQuota(body string, checkedAt time.Time) (*model.UsageQuota, error) {
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, errors.New("usage response is not valid JSON")
	}
	rate := mapField(raw, "rate_limit", "rateLimits")
	if rate == nil {
		return nil, errors.New("usage response does not contain rate_limit")
	}
	quota := &model.UsageQuota{PlanType: stringField(raw, "plan_type", "planType")}
	if allowed, ok := boolField(rate, "allowed"); ok {
		quota.Allowed = &allowed
	}
	quota.LimitReached, _ = boolField(rate, "limit_reached", "limitReached")
	assignQuotaWindow(quota, parseQuotaWindow(mapField(rate, "primary_window", "primary"), checkedAt))
	assignQuotaWindow(quota, parseQuotaWindow(mapField(rate, "secondary_window", "secondary"), checkedAt))
	if credits := mapField(raw, "credits"); credits != nil {
		quota.Credits, _ = boolField(credits, "has_credits", "hasCredits")
		quota.Unlimited, _ = boolField(credits, "unlimited")
		if balance, ok := numberField(credits, "balance"); ok {
			quota.CreditsBalance = &balance
		}
	}
	return quota, nil
}

func parseQuotaWindow(raw map[string]any, checkedAt time.Time) *model.QuotaWindow {
	if raw == nil {
		return nil
	}
	window := &model.QuotaWindow{}
	if used, ok := numberField(raw, "used_percent", "usedPercent"); ok {
		used = clampPercent(used)
		remaining := clampPercent(100 - used)
		window.UsedPercent, window.RemainingPercent = &used, &remaining
	}
	if seconds, ok := numberField(raw, "limit_window_seconds", "window_seconds", "limitWindowSeconds"); ok {
		window.LimitWindowSeconds = int64(seconds)
	} else if minutes, ok := numberField(raw, "window_minutes", "windowMinutes"); ok {
		window.LimitWindowSeconds = int64(minutes * 60)
	}
	if seconds, ok := numberField(raw, "reset_after_seconds", "resetAfterSeconds"); ok {
		window.ResetAfterSeconds = int64(seconds)
	}
	if reset, ok := numberField(raw, "reset_at", "resets_at", "resetsAt"); ok && reset > 0 {
		if reset > 1e12 {
			reset /= 1000
		}
		window.ResetAt = time.Unix(int64(reset), 0).UTC()
	} else if value := stringField(raw, "reset_at", "resets_at", "resetsAt"); value != "" {
		if parsed, ok := parseTime(value); ok {
			window.ResetAt = parsed
		}
	} else if window.ResetAfterSeconds > 0 {
		window.ResetAt = checkedAt.Add(time.Duration(window.ResetAfterSeconds) * time.Second)
	}
	return window
}

func assignQuotaWindow(quota *model.UsageQuota, window *model.QuotaWindow) {
	if window == nil {
		return
	}
	switch seconds := window.LimitWindowSeconds; {
	case seconds >= 4*60*60 && seconds <= 6*60*60:
		quota.FiveHour = window
	case seconds >= 6*24*60*60 && seconds <= 8*24*60*60:
		quota.SevenDay = window
	}
}

func decodeAuthIdentity(data []byte) (authIdentity, error) {
	var identity authIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return authIdentity{}, err
	}
	identity.AccountID = strings.TrimSpace(identity.AccountID)
	identity.ChatGPTAccountID = strings.TrimSpace(identity.ChatGPTAccountID)
	identity.Email = strings.ToLower(strings.TrimSpace(firstNonEmpty(identity.Email, identity.Name)))
	identity.Provider = normalizeProvider(firstNonEmpty(identity.Provider, identity.Type))
	return identity, nil
}

func jsonFingerprint(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func managementBaseURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		base = base[:len(base)-3]
	}
	return strings.TrimRight(base, "/") + "/v0/management"
}

func managementHTTPClient() *http.Client {
	return &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func statusForUpstream(status int) string {
	switch status {
	case http.StatusOK:
		return "ok"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusPaymentRequired:
		return "payment_required"
	case http.StatusForbidden:
		return "forbidden_or_payment_required"
	case http.StatusNotFound:
		return "usage_endpoint_unavailable"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		if status >= 500 {
			return "upstream_error"
		}
		return "error"
	}
}

func reasonForUpstream(status int) string {
	if status == http.StatusOK {
		return ""
	}
	return fmt.Sprintf("upstream_%d", status)
}

func connectivityReasonMessage(reason string) string {
	if reason == "not_in_cpa_pool" {
		return "该归档文件尚未同步到 CPA 活动池"
	}
	if reason == "ambiguous_account" {
		return "CPA 活动池存在多个匹配账号，无法确定唯一文件"
	}
	return reason
}

func sanitizedUpstreamError(body string, status int) string {
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err == nil {
		for _, source := range []map[string]any{mapField(raw, "error"), raw} {
			if source == nil {
				continue
			}
			for _, key := range []string{"message", "detail", "code", "type"} {
				if message := stringField(source, key); message != "" {
					return cleanError(fmt.Sprintf("HTTP %d: %s", status, message))
				}
			}
		}
	}
	return fmt.Sprintf("上游返回 HTTP %d", status)
}

func cleanError(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if len(message) > 240 {
		message = message[:240] + "…"
	}
	return message
}

func quotaWindowExhausted(window *model.QuotaWindow) bool {
	return window != nil && ((window.UsedPercent != nil && *window.UsedPercent >= 100) || (window.RemainingPercent != nil && *window.RemainingPercent <= 0))
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func mapField(raw map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if value, ok := raw[key].(map[string]any); ok {
			return value
		}
	}
	return nil
}

func stringField(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func numberField(raw map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case json.Number:
			number, err := typed.Float64()
			return number, err == nil
		case string:
			number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			return number, err == nil
		}
	}
	return 0, false
}

func boolField(raw map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			value, err := strconv.ParseBool(strings.TrimSpace(typed))
			return value, err == nil
		}
	}
	return false, false
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func normalizeProvider(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "codex"), strings.Contains(value, "openai"), strings.Contains(value, "chatgpt"):
		return "codex"
	case strings.Contains(value, "claude"), strings.Contains(value, "anthropic"):
		return "claude"
	case value == "":
		return "unknown"
	default:
		return value
	}
}

func nonEmptySet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniquePath(folder, name string) string {
	candidate := filepath.Join(folder, name)
	if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate = filepath.Join(folder, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftAbs), filepath.Clean(rightAbs))
}
