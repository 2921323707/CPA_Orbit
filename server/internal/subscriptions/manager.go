package subscriptions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/storage"
)

var unsafeNameRE = regexp.MustCompile(`[^A-Za-z0-9._+-]+`)

const codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"

type cpaAuthEntry struct {
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

type cpaAuthList struct {
	Files []cpaAuthEntry `json:"files"`
}

type cpaAPICallResponse struct {
	StatusCode int    `json:"status_code"`
	Body       string `json:"body"`
}

var ErrDuplicateSubscription = errors.New("subscription already exists")

type DuplicateSubscriptionError struct {
	MatchField    string
	ExistingEmail string
	ExistingFile  string
}

func (e *DuplicateSubscriptionError) Error() string {
	return fmt.Sprintf("%s: matched %s", ErrDuplicateSubscription, e.MatchField)
}

func (e *DuplicateSubscriptionError) Unwrap() error { return ErrDuplicateSubscription }

type PageResult struct {
	Subscriptions []model.Subscription `json:"subscriptions"`
	Total         int                  `json:"total"`
	Page          int                  `json:"page"`
	PageSize      int                  `json:"pageSize"`
	TotalPages    int                  `json:"totalPages"`
	Folders       []string             `json:"folders"`
	Insights      SubscriptionInsights `json:"insights"`
}

type SubscriptionInsights struct {
	Normal       int     `json:"normal"`
	Error        int     `json:"error"`
	Priced       int     `json:"priced"`
	TotalCost    float64 `json:"totalCost"`
	AverageCost  float64 `json:"averageCost"`
	ExpiringSoon int     `json:"expiringSoon"`
}

type ImportOptions struct {
	OrderURL         string
	BaseURL          string
	AcquisitionPrice string
}

type Manager struct {
	mu             sync.RWMutex
	root           string
	checksPath     string
	settings       *config.Store
	items          map[string]model.Subscription
	checks         map[string]model.Connectivity
	importMu       sync.Mutex
	testMu         sync.Mutex
	lastUsageCheck time.Time
	authCache      []cpaAuthEntry
	authCacheAt    time.Time
}

func NewManager(root, checksPath string, settings *config.Store) (*Manager, error) {
	m := &Manager{root: root, checksPath: checksPath, settings: settings, items: make(map[string]model.Subscription), checks: make(map[string]model.Connectivity)}
	if err := storage.LoadJSON(checksPath, &m.checks); err != nil {
		return nil, fmt.Errorf("load connectivity checks: %w", err)
	}
	if err := m.Scan(); err != nil {
		return nil, err
	}
	if err := m.ReconcileRuntime(); err != nil {
		return nil, err
	}
	return m, nil
}

// ReconcileRuntime rebuilds the CPA auth directory from subscription archives.
// The directory is a runtime projection and may safely be recreated at startup.
func (m *Manager) ReconcileRuntime() error {
	settings := m.settings.Get()
	if !settings.SyncToCPAAuthDir || config.ValidateCPAAuthDir(settings.CPAAuthDir) != nil {
		return nil
	}
	items := m.List("", "", "")
	expectedContent := make([]string, 0, len(items))
	for _, item := range items {
		path, err := safeArchivedPath(m.root, item.RelativePath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read subscription %s for runtime sync: %w", item.RelativePath, err)
		}
		fingerprint, err := jsonFingerprint(data)
		if err != nil {
			return fmt.Errorf("parse subscription %s for runtime sync: %w", item.RelativePath, err)
		}
		expectedContent = append(expectedContent, fingerprint)
		if _, err := syncBytesToAuthDir(data, item.Email, settings.CPAAuthDir); err != nil {
			return fmt.Errorf("sync subscription %s to runtime: %w", item.RelativePath, err)
		}
	}
	if err := removeOrphanRuntimeFiles(settings.CPAAuthDir, expectedContent); err != nil {
		return err
	}
	m.invalidateAuthCache()
	return nil
}

func removeOrphanRuntimeFiles(authDir string, expected []string) error {
	resolvedDir, err := filepath.EvalSymlinks(authDir)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(resolvedDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(resolvedDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fingerprint, err := jsonFingerprint(data)
		if err != nil {
			continue
		}
		matched := false
		for _, candidate := range expected {
			if fingerprint == candidate {
				matched = true
				break
			}
		}
		if !matched {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove orphan runtime file %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

func (m *Manager) Scan() error {
	if err := os.MkdirAll(m.root, 0o700); err != nil {
		return fmt.Errorf("create subscription archive: %w", err)
	}
	items := make(map[string]model.Subscription)
	m.mu.RLock()
	checks := make(map[string]model.Connectivity, len(m.checks))
	for id, check := range m.checks {
		checks[id] = check
	}
	m.mu.RUnlock()
	err := filepath.WalkDir(m.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(m.root, path)
		if err != nil {
			return nil
		}
		sub, err := ParseJSON(data, rel, time.Now())
		if err != nil {
			return nil
		}
		if check, ok := checks[sub.ID]; ok {
			sub.Connectivity = check
		}
		items[sub.ID] = sub
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan subscriptions: %w", err)
	}
	m.mu.Lock()
	m.items = items
	m.mu.Unlock()
	return nil
}

func (m *Manager) List(folder, category, search string) []model.Subscription {
	return m.filteredSubscriptions(folder, category, search)
}

func (m *Manager) Page(folder, category, search string, page, pageSize int) PageResult {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	items := m.filteredSubscriptions(folder, category, search)
	total := len(items)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
		if page > totalPages {
			page = totalPages
		}
	}
	start := (page - 1) * pageSize
	if start < 0 || start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	folders := m.folders()
	return PageResult{
		Subscriptions: items[start:end],
		Total:         total, Page: page, PageSize: pageSize, TotalPages: totalPages,
		Folders: folders, Insights: subscriptionInsights(items),
	}
}

func subscriptionInsights(items []model.Subscription) SubscriptionInsights {
	var result SubscriptionInsights
	for _, item := range items {
		if subscriptionCategory(item) == "normal" {
			result.Normal++
		} else {
			result.Error++
		}
		if item.AcquisitionPrice != nil {
			result.Priced++
			result.TotalCost += *item.AcquisitionPrice
		}
		if item.RemainingDays != nil && *item.RemainingDays >= 0 && *item.RemainingDays <= 7 {
			result.ExpiringSoon++
		}
	}
	if result.Priced > 0 {
		result.AverageCost = result.TotalCost / float64(result.Priced)
	}
	return result
}

func (m *Manager) filteredSubscriptions(folder, category, search string) []model.Subscription {
	folder = strings.ToLower(strings.Trim(filepath.ToSlash(folder), "/"))
	category = strings.ToLower(strings.TrimSpace(category))
	search = strings.ToLower(strings.TrimSpace(search))
	m.mu.RLock()
	result := make([]model.Subscription, 0, len(m.items))
	for _, item := range m.items {
		item.Category = subscriptionCategory(item)
		if folder != "" && strings.ToLower(strings.Trim(item.Folder, "/")) != folder {
			continue
		}
		if category != "" && category != "all" && item.Category != category {
			continue
		}
		if search != "" {
			haystack := strings.ToLower(strings.Join([]string{item.Email, item.Name, item.AccountID, item.ChatGPTAccountID, item.FileName, item.RelativePath}, " "))
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		result = append(result, item)
	}
	m.mu.RUnlock()
	sort.Slice(result, func(i, j int) bool {
		if result[i].RelativePath == result[j].RelativePath {
			return strings.ToLower(result[i].Email) < strings.ToLower(result[j].Email)
		}
		return result[i].RelativePath > result[j].RelativePath
	})
	return result
}

func (m *Manager) folders() []string {
	m.mu.RLock()
	set := make(map[string]struct{})
	for _, item := range m.items {
		if folder := strings.Trim(item.Folder, "/"); folder != "" {
			set[folder] = struct{}{}
		}
	}
	m.mu.RUnlock()
	folders := make([]string, 0, len(set))
	for folder := range set {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	return folders
}

func subscriptionCategory(item model.Subscription) string {
	quota := item.Connectivity.Quota
	if quota != nil && quota.FiveHour != nil && quota.FiveHour.RemainingPercent != nil && *quota.FiveHour.RemainingPercent <= 0 {
		if quota.SevenDay == nil || quota.SevenDay.RemainingPercent == nil || *quota.SevenDay.RemainingPercent > 0 {
			return "normal"
		}
	}
	if item.Connectivity.Status == "ok" {
		return "normal"
	}
	return "error"
}

func (m *Manager) Get(id string) (model.Subscription, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[id]
	return item, ok
}

func (m *Manager) duplicateSubscription(data []byte) (model.Subscription, bool, error) {
	fingerprint, err := jsonFingerprint(data)
	if err != nil {
		return model.Subscription{}, false, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, existing := range m.items {
		path, pathErr := safeArchivedPath(m.root, existing.RelativePath)
		if pathErr != nil {
			continue
		}
		stored, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		storedFingerprint, fingerprintErr := jsonFingerprint(stored)
		if fingerprintErr == nil && storedFingerprint == fingerprint {
			return existing, true, nil
		}
	}
	return model.Subscription{}, false, nil
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
	return string(digest[:]), nil
}

// Import keeps the legacy argument shape for callers outside the HTTP layer.
// New imports should use ImportWithOptions so only supported metadata is accepted.
func (m *Manager) Import(data []byte, originalName, orderURL, baseURL string) (model.Subscription, bool, error) {
	return m.ImportWithOptions(data, originalName, ImportOptions{OrderURL: orderURL, BaseURL: baseURL})
}

func (m *Manager) ImportWithOptions(data []byte, originalName string, options ImportOptions) (model.Subscription, bool, error) {
	m.importMu.Lock()
	defer m.importMu.Unlock()
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil || raw == nil {
		return model.Subscription{}, false, errors.New("uploaded file must contain a JSON object")
	}
	_, err := ParseJSON(data, "validation.json", time.Now())
	if err != nil {
		return model.Subscription{}, false, err
	}
	if existing, duplicate, duplicateErr := m.duplicateSubscription(data); duplicateErr != nil {
		return model.Subscription{}, false, duplicateErr
	} else if duplicate {
		return model.Subscription{}, false, &DuplicateSubscriptionError{
			MatchField: "json", ExistingEmail: existing.Email, ExistingFile: existing.FileName,
		}
	}
	orderURL := strings.TrimSpace(options.OrderURL)
	baseURL := strings.TrimSpace(options.BaseURL)
	if orderURL != "" {
		if err := config.ValidateHTTPURL(orderURL); err != nil {
			return model.Subscription{}, false, fmt.Errorf("orderUrl: %w", err)
		}
		raw["order_url"] = orderURL
	}
	if baseURL != "" {
		if err := config.ValidateHTTPURL(baseURL); err != nil {
			return model.Subscription{}, false, fmt.Errorf("baseUrl: %w", err)
		}
		raw["base_url"] = strings.TrimRight(baseURL, "/")
	}
	acquisitionPrice := strings.TrimSpace(options.AcquisitionPrice)
	if acquisitionPrice != "" {
		value, err := strconv.ParseFloat(acquisitionPrice, 64)
		if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return model.Subscription{}, false, errors.New("acquisitionPrice must be a non-negative number")
		}
		raw["acquisition_price"] = value
	}
	encoded, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return model.Subscription{}, false, err
	}
	encoded = append(encoded, '\n')
	settings := m.settings.Get()
	folder, err := m.archiveFolder(time.Now())
	if err != nil {
		return model.Subscription{}, false, err
	}
	name := sanitizeJSONName(originalName)
	target := uniquePath(folder, name)
	if err := writeNewFile(target, encoded); err != nil {
		return model.Subscription{}, false, fmt.Errorf("archive subscription: %w", err)
	}
	if err := m.Scan(); err != nil {
		return model.Subscription{}, false, err
	}
	rel, _ := filepath.Rel(m.root, target)
	id := stableID(rel)
	sub, ok := m.Get(id)
	if !ok {
		return model.Subscription{}, false, errors.New("imported subscription was not found after archive scan")
	}
	synced := false
	if settings.SyncToCPAAuthDir {
		if _, err := syncBytesToAuthDir(encoded, sub.Email, settings.CPAAuthDir); err != nil {
			return sub, false, fmt.Errorf("subscription archived but CPA auth-dir sync failed: %w", err)
		}
		m.invalidateAuthCache()
		synced = true
	}
	return sub, synced, nil
}

func (m *Manager) Sync(id string) (string, error) {
	sub, ok := m.Get(id)
	if !ok {
		return "", os.ErrNotExist
	}
	settings := m.settings.Get()
	if err := config.ValidateCPAAuthDir(settings.CPAAuthDir); err != nil {
		return "", err
	}
	path, err := safeArchivedPath(m.root, sub.RelativePath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	target, err := syncBytesToAuthDir(data, sub.Email, settings.CPAAuthDir)
	if err == nil {
		m.invalidateAuthCache()
	}
	return target, err
}

func (m *Manager) invalidateAuthCache() {
	m.testMu.Lock()
	m.authCache = nil
	m.authCacheAt = time.Time{}
	m.testMu.Unlock()
}

func (m *Manager) Delete(id string) error {
	sub, ok := m.Get(id)
	if !ok {
		return os.ErrNotExist
	}
	path, err := safeArchivedPath(m.root, sub.RelativePath)
	if err != nil {
		return err
	}
	var runtimeTarget string
	var settings config.Settings
	if m.settings != nil {
		settings = m.settings.Get()
	}
	if m.settings != nil && settings.SyncToCPAAuthDir && config.ValidateCPAAuthDir(settings.CPAAuthDir) == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		runtimeTarget, err = matchingAuthTarget(settings.CPAAuthDir, data)
		if err != nil {
			return fmt.Errorf("cannot delete runtime copy safely: %w", err)
		}
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	if runtimeTarget != "" {
		if err := os.Remove(runtimeTarget); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("subscription archive deleted but runtime copy cleanup failed: %w", err)
		}
	}
	m.mu.Lock()
	delete(m.items, id)
	delete(m.checks, id)
	checks := make(map[string]model.Connectivity, len(m.checks))
	for key, check := range m.checks {
		checks[key] = check
	}
	m.mu.Unlock()
	if err := storage.SaveJSON(m.checksPath, checks); err != nil {
		return fmt.Errorf("subscription deleted but connectivity state cleanup failed: %w", err)
	}
	return nil
}

func (m *Manager) Test(ctx context.Context, id string) (model.Connectivity, error) {
	sub, ok := m.Get(id)
	if !ok {
		return model.Connectivity{}, os.ErrNotExist
	}
	m.testMu.Lock()
	defer m.testMu.Unlock()
	if wait := 500*time.Millisecond - time.Since(m.lastUsageCheck); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return model.Connectivity{}, ctx.Err()
		case <-timer.C:
		}
	}
	m.lastUsageCheck = time.Now()
	checkedAt := time.Now()
	settings := m.settings.Get()
	if strings.TrimSpace(settings.CPAManagementKey) == "" {
		check := model.Connectivity{Status: "configuration_error", ReasonCode: "management_key_missing", CheckedAt: checkedAt, Error: "CPA management key is not configured"}
		m.saveCheck(id, check)
		return check, nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	authFiles, err := m.cpaAuthFiles(requestCtx, settings)
	if err != nil {
		check := model.Connectivity{Status: "cpa_unavailable", ReasonCode: "auth_files_unavailable", CheckedAt: checkedAt, Error: cleanError(err.Error())}
		m.saveCheck(id, check)
		return check, nil
	}
	auth, reason := matchCPAAuth(authFiles, sub)
	if reason != "" {
		check := model.Connectivity{Status: reason, ReasonCode: reason, CheckedAt: checkedAt, Error: connectivityReasonMessage(reason)}
		m.saveCheck(id, check)
		return check, nil
	}
	check := model.Connectivity{
		Status: auth.Status, CheckedAt: checkedAt, CPAStatus: auth.Status,
		CPAStatusMessage: auth.StatusMessage, CPAUnavailable: auth.Unavailable,
	}
	if retryAt, ok := parseTime(auth.NextRetryAfter); ok {
		check.NextRetryAt = retryAt
	}
	if auth.Disabled || strings.EqualFold(auth.Status, "disabled") {
		check.Status = "disabled"
		check.ReasonCode = "cpa_disabled"
		check.Error = "该账号已在 CPA 中禁用"
		m.saveCheck(id, check)
		return check, nil
	}
	provider := normalizeProvider(firstNonEmpty(sub.Provider, auth.Provider, sub.Type))
	if provider != "codex" {
		if strings.TrimSpace(check.Status) == "" {
			check.Status = "ok"
		}
		check.ReasonCode = "provider_status_only"
		m.saveCheck(id, check)
		return check, nil
	}
	accountID := firstNonEmpty(auth.Account, sub.AccountID, sub.ChatGPTAccountID)
	if accountID == "" {
		check.Status = "missing_account_id"
		check.ReasonCode = "missing_account_id"
		check.Error = "缺少 ChatGPT account_id，无法安全查询对应工作区额度"
		m.saveCheck(id, check)
		return check, nil
	}
	usageCheck := m.checkCPAUsage(requestCtx, settings, auth.AuthIndex, accountID, checkedAt)
	usageCheck.CPAStatus = auth.Status
	usageCheck.CPAStatusMessage = auth.StatusMessage
	usageCheck.CPAUnavailable = auth.Unavailable
	usageCheck.NextRetryAt = check.NextRetryAt
	if strings.EqualFold(auth.StatusMessage, "payment_required") && usageCheck.Status == "ok" {
		usageCheck.Status = "payment_required"
		usageCheck.ReasonCode = "cpa_payment_required"
		usageCheck.Error = "CPA 最近一次模型调用返回 HTTP 402/403"
	}
	m.saveCheck(id, usageCheck)
	return usageCheck, nil
}

func (m *Manager) cpaAuthFiles(ctx context.Context, settings config.Settings) ([]cpaAuthEntry, error) {
	if len(m.authCache) > 0 && time.Since(m.authCacheAt) < 30*time.Second {
		return append([]cpaAuthEntry(nil), m.authCache...), nil
	}
	endpoint := managementBaseURL(settings.BaseURL) + "/auth-files"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Management-Key", settings.CPAManagementKey)
	req.Header.Set("Accept", "application/json")
	resp, err := managementHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("CPA auth-files returned HTTP %d", resp.StatusCode)
	}
	var payload cpaAuthList
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode CPA auth-files: %w", err)
	}
	filtered := make([]cpaAuthEntry, 0, len(payload.Files))
	for _, entry := range payload.Files {
		if entry.RuntimeOnly || strings.TrimSpace(entry.AuthIndex) == "" {
			continue
		}
		filtered = append(filtered, entry)
	}
	m.authCache = append(m.authCache[:0], filtered...)
	m.authCacheAt = time.Now()
	return append([]cpaAuthEntry(nil), filtered...), nil
}

func matchCPAAuth(entries []cpaAuthEntry, sub model.Subscription) (cpaAuthEntry, string) {
	ids := nonEmptySet(sub.AccountID, sub.ChatGPTAccountID)
	provider := normalizeProvider(sub.Provider)
	matches := make([]cpaAuthEntry, 0, 2)
	if len(ids) > 0 {
		for _, entry := range entries {
			if provider != "unknown" && normalizeProvider(entry.Provider) != "unknown" && provider != normalizeProvider(entry.Provider) {
				continue
			}
			if _, ok := ids[strings.TrimSpace(entry.Account)]; ok {
				matches = append(matches, entry)
			}
		}
		if len(matches) > 1 && sub.Email != "" {
			email := strings.ToLower(strings.TrimSpace(sub.Email))
			narrowed := matches[:0]
			for _, entry := range matches {
				if strings.ToLower(strings.TrimSpace(entry.Email)) == email {
					narrowed = append(narrowed, entry)
				}
			}
			matches = narrowed
		}
	}
	if len(matches) == 0 && sub.Email != "" {
		email := strings.ToLower(strings.TrimSpace(sub.Email))
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
		return cpaAuthEntry{}, "not_in_cpa_pool"
	}
	if len(matches) > 1 {
		return cpaAuthEntry{}, "ambiguous_account"
	}
	return matches[0], ""
}

func (m *Manager) checkCPAUsage(ctx context.Context, settings config.Settings, authIndex, accountID string, checkedAt time.Time) model.Connectivity {
	requestBody := map[string]any{
		"auth_index": authIndex,
		"method":     http.MethodGet,
		"url":        codexUsageURL,
		"header": map[string]string{
			"Authorization":      "Bearer $TOKEN$",
			"ChatGPT-Account-ID": accountID,
			"Accept":             "application/json",
			"User-Agent":         "codex_cli_rs",
			"Originator":         "codex_cli_rs",
			"OpenAI-Beta":        "codex-1",
		},
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return model.Connectivity{Status: "error", ReasonCode: "request_encode_failed", CheckedAt: checkedAt, Error: "failed to encode usage request"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, managementBaseURL(settings.BaseURL)+"/api-call", bytes.NewReader(encoded))
	if err != nil {
		return model.Connectivity{Status: "error", ReasonCode: "request_build_failed", CheckedAt: checkedAt, Error: "failed to build usage request"}
	}
	req.Header.Set("X-Management-Key", settings.CPAManagementKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	started := time.Now()
	resp, err := managementHTTPClient().Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		status := "upstream_error"
		reason := "management_api_error"
		if ctx.Err() != nil || strings.Contains(strings.ToLower(err.Error()), "timeout") {
			status, reason = "timeout", "usage_timeout"
		}
		return model.Connectivity{Status: status, ReasonCode: reason, LatencyMS: latency, CheckedAt: checkedAt, Error: cleanError(err.Error())}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.Connectivity{Status: "cpa_unavailable", ReasonCode: "management_http_error", HTTPStatus: resp.StatusCode, LatencyMS: latency, CheckedAt: checkedAt, Error: fmt.Sprintf("CPA management API returned HTTP %d", resp.StatusCode)}
	}
	var upstream cpaAPICallResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&upstream); err != nil {
		return model.Connectivity{Status: "invalid_response", ReasonCode: "management_response_invalid", LatencyMS: latency, CheckedAt: checkedAt, Error: "CPA management response is invalid"}
	}
	check := model.Connectivity{Status: statusForUpstream(upstream.StatusCode), ReasonCode: reasonForUpstream(upstream.StatusCode), HTTPStatus: upstream.StatusCode, LatencyMS: latency, CheckedAt: checkedAt}
	if upstream.StatusCode != http.StatusOK {
		check.Error = sanitizedUpstreamError(upstream.Body, upstream.StatusCode)
		return check
	}
	quota, err := parseUsageQuota(upstream.Body, checkedAt)
	if err != nil {
		check.Status = "invalid_response"
		check.ReasonCode = "usage_response_invalid"
		check.Error = err.Error()
		return check
	}
	check.Quota = quota
	if quota.LimitReached || (quota.Allowed != nil && !*quota.Allowed) || quotaWindowExhausted(quota.FiveHour) || quotaWindowExhausted(quota.SevenDay) {
		check.Status = "quota_exhausted"
		check.ReasonCode = "usage_limit_reached"
		check.Error = "额度已耗尽，等待窗口重置"
	}
	return check
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
	switch reason {
	case "not_in_cpa_pool":
		return "该归档文件尚未同步到 CPA 活动池"
	case "ambiguous_account":
		return "CPA 活动池存在多个匹配账号，无法确定唯一文件"
	default:
		return reason
	}
}

func cleanError(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if len(message) > 240 {
		message = message[:240] + "…"
	}
	return message
}

func parseUsageQuota(body string, checkedAt time.Time) (*model.UsageQuota, error) {
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
		window.UsedPercent = &used
		window.RemainingPercent = &remaining
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
	seconds := window.LimitWindowSeconds
	switch {
	case seconds >= 4*60*60 && seconds <= 6*60*60:
		quota.FiveHour = window
	case seconds >= 6*24*60*60 && seconds <= 8*24*60*60:
		quota.SevenDay = window
	}
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
		if value, ok := raw[key]; ok && value != nil {
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
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

func (m *Manager) saveCheck(id string, check model.Connectivity) {
	m.mu.Lock()
	m.checks[id] = check
	if item, ok := m.items[id]; ok {
		item.Connectivity = check
		m.items[id] = item
	}
	copyChecks := make(map[string]model.Connectivity, len(m.checks))
	for key, value := range m.checks {
		copyChecks[key] = value
	}
	m.mu.Unlock()
	_ = storage.SaveJSON(m.checksPath, copyChecks)
}

func modelsEndpoint(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		base = base[:len(base)-3]
	}
	return strings.TrimRight(base, "/") + "/v1/models"
}

func (m *Manager) archiveFolder(now time.Time) (string, error) {
	rootResolved, err := filepath.Abs(m.root)
	if err != nil {
		return "", err
	}
	folder := filepath.Join(rootResolved, now.Format("0102"))
	if info, err := os.Lstat(folder); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("archive date folder cannot be a symbolic link")
	}
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return "", err
	}
	if !within(rootResolved, resolved) {
		return "", errors.New("archive folder escapes subscription root")
	}
	return resolved, nil
}

func sanitizeJSONName(name string) string {
	name = filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	name = unsafeNameRE.ReplaceAllString(name, "_")
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "subscription.json"
	}
	if !strings.EqualFold(filepath.Ext(name), ".json") {
		name += ".json"
	}
	return name
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

func writeNewFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

type authIdentity struct {
	AccountID        string `json:"account_id"`
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Email            string `json:"email"`
	Name             string `json:"name"`
	Provider         string `json:"provider"`
	Type             string `json:"type"`
}

func syncBytesToAuthDir(data []byte, email, authDir string) (string, error) {
	if err := config.ValidateCPAAuthDir(authDir); err != nil {
		return "", err
	}
	resolvedDir, err := filepath.EvalSymlinks(authDir)
	if err != nil {
		return "", err
	}
	incoming, err := decodeAuthIdentity(data)
	if err != nil {
		return "", fmt.Errorf("parse CPA auth identity: %w", err)
	}
	if strings.TrimSpace(incoming.Email) == "" {
		incoming.Email = strings.TrimSpace(email)
	}
	target, err := matchingAuthTarget(resolvedDir, data)
	if err != nil {
		return "", err
	}
	if target == "" {
		namePart := strings.TrimSpace(incoming.Email)
		if namePart == "" {
			namePart = firstNonEmpty(incoming.AccountID, incoming.ChatGPTAccountID)
		}
		namePart = strings.Trim(unsafeNameRE.ReplaceAllString(namePart, "_"), ". ")
		if namePart == "" {
			return "", errors.New("subscription email or account_id is required for CPA auth-dir filename")
		}
		provider := normalizeProvider(incoming.Provider)
		prefix := provider + "_oauth_"
		if provider == "unknown" {
			prefix = "oauth_"
		}
		name := prefix + namePart + ".json"
		target = uniquePath(resolvedDir, name)
	}
	if !within(resolvedDir, target) {
		return "", errors.New("unsafe CPA auth-dir target path")
	}
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", errors.New("CPA auth-dir target must be a regular file")
		}
		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil || !within(resolvedDir, resolvedTarget) {
			return "", errors.New("CPA auth-dir target escapes configured directory")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	// Keep the temporary file out of CPA's JSON watcher until the final rename.
	tmp, err := os.CreateTemp(resolvedDir, ".cpa-sync-*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := bytes.NewReader(data).WriteTo(tmp); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmpName, target); err != nil {
		if removeErr := os.Remove(target); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return "", err
		}
		if err := os.Rename(tmpName, target); err != nil {
			return "", err
		}
	}
	return target, nil
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

func identitiesMatch(left, right authIdentity) bool {
	if left.Provider != "unknown" && right.Provider != "unknown" && left.Provider != right.Provider {
		return false
	}
	leftIDs := nonEmptySet(left.AccountID, left.ChatGPTAccountID)
	for id := range nonEmptySet(right.AccountID, right.ChatGPTAccountID) {
		if _, ok := leftIDs[id]; ok {
			return true
		}
	}
	return left.Email != "" && right.Email != "" && strings.EqualFold(left.Email, right.Email)
}

func matchingAuthTarget(authDir string, incoming []byte) (string, error) {
	entries, err := os.ReadDir(authDir)
	if err != nil {
		return "", err
	}
	fingerprint, err := jsonFingerprint(incoming)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(authDir, entry.Name())
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		storedFingerprint, err := jsonFingerprint(data)
		if err != nil || storedFingerprint != fingerprint {
			continue
		}
		matches = append(matches, path)
	}
	if len(matches) > 1 {
		return "", errors.New("multiple CPA auth files contain the same JSON; resolve the duplicate archive")
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", nil
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

func safeArchivedPath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New("absolute subscription path is not allowed")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(relative))
	if !within(rootAbs, target) {
		return "", errors.New("subscription path escapes archive root")
	}
	info, err := os.Lstat(target)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("subscription archive entry must be a regular file")
	}
	return target, nil
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
