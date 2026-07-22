package subscriptions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/gateways"
	cpagateway "cpa-monitor/server/internal/gateways/cpa"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/storage"
)

var unsafeNameRE = regexp.MustCompile(`[^A-Za-z0-9._+-]+`)

type cpaAuthEntry = cpagateway.AuthEntry

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
	SkipLegacySync   bool
}

type Manager struct {
	mu         sync.RWMutex
	root       string
	checksPath string
	settings   *config.Store
	items      map[string]model.Subscription
	checks     map[string]model.Connectivity
	importMu   sync.Mutex
	cpa        *cpagateway.Client
}

func NewManager(root, checksPath string, settings *config.Store) (*Manager, error) {
	m := &Manager{root: root, checksPath: checksPath, settings: settings, items: make(map[string]model.Subscription), checks: make(map[string]model.Connectivity)}
	m.cpa = cpagateway.NewClient(func() cpagateway.Config {
		current := settings.Get()
		return cpagateway.Config{BaseURL: current.BaseURL, ManagementKey: current.CPAManagementKey, AuthDir: current.CPAAuthDir, SyncEnabled: current.SyncToCPAAuthDir}
	}, filepath.Join(filepath.Dir(checksPath), "cpa-managed-files.json"))
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

// ReconcileRuntime projects archives to CPA while preserving files not owned by Orbit.
func (m *Manager) ReconcileRuntime() error {
	settings := m.settings.Get()
	if !settings.SyncToCPAAuthDir || config.ValidateCPAAuthDir(settings.CPAAuthDir) != nil {
		return nil
	}
	items := m.List("", "", "")
	credentials := make([]gateways.Credential, 0, len(items))
	for _, item := range items {
		path, err := safeArchivedPath(m.root, item.RelativePath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read subscription %s for runtime sync: %w", item.RelativePath, err)
		}
		credentials = append(credentials, gatewayCredential(item, data))
	}
	return m.cpa.Reconcile(context.Background(), credentials)
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
	if settings.SyncToCPAAuthDir && !options.SkipLegacySync {
		if _, err := m.cpa.Deploy(context.Background(), gatewayCredential(sub, encoded), gateways.DeployOptions{}); err != nil {
			return sub, false, fmt.Errorf("subscription archived but CPA auth-dir sync failed: %w", err)
		}
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
	result, err := m.cpa.Deploy(context.Background(), gatewayCredential(sub, data), gateways.DeployOptions{UpdateExisting: true})
	return result.Binding.ExternalRef, err
}

func (m *Manager) GatewayCredential(id string) (gateways.Credential, error) {
	sub, ok := m.Get(id)
	if !ok {
		return gateways.Credential{}, os.ErrNotExist
	}
	path, err := safeArchivedPath(m.root, sub.RelativePath)
	if err != nil {
		return gateways.Credential{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return gateways.Credential{}, err
	}
	return gatewayCredential(sub, data), nil
}

func (m *Manager) CPAAdapter() gateways.Adapter { return m.cpa }

func (m *Manager) SaveConnectivity(id string, check model.Connectivity) { m.saveCheck(id, check) }

func (m *Manager) Delete(id string) error {
	sub, ok := m.Get(id)
	if !ok {
		return os.ErrNotExist
	}
	path, err := safeArchivedPath(m.root, sub.RelativePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if m.cpa != nil {
		if err := m.cpa.Detach(context.Background(), gateways.BindingRef{}, gatewayCredential(sub, data)); err != nil {
			return fmt.Errorf("cannot delete runtime copy safely: %w", err)
		}
	}
	if err := os.Remove(path); err != nil {
		return err
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
	result, err := m.cpa.Inspect(ctx, gateways.BindingRef{}, gatewayCredential(sub, nil))
	if err != nil {
		return model.Connectivity{}, err
	}
	m.saveCheck(id, result.Connectivity)
	return result.Connectivity, nil
}

func matchCPAAuth(entries []cpaAuthEntry, sub model.Subscription) (cpaAuthEntry, string) {
	return cpagateway.MatchAuth(entries, gatewayCredential(sub, nil))
}

func parseUsageQuota(body string, checkedAt time.Time) (*model.UsageQuota, error) {
	return cpagateway.ParseUsageQuota(body, checkedAt)
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

func syncBytesToAuthDir(data []byte, email, authDir string) (string, error) {
	return cpagateway.SyncBytesToAuthDir(data, email, authDir)
}

func gatewayCredential(sub model.Subscription, data []byte) gateways.Credential {
	return gateways.Credential{SubscriptionID: sub.ID, Data: data, Email: sub.Email, AccountID: sub.AccountID, ChatGPTAccountID: sub.ChatGPTAccountID, Provider: firstNonEmpty(sub.Provider, sub.Type)}
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
