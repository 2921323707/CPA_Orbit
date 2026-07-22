package subscriptions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/model"
)

var unsafeProviderRE = regexp.MustCompile(`[^a-z0-9]+`)

const AuthAnalysisVersion = "orbit-auth-v1"

var safeAuthFields = map[string]struct{}{
	"account_id": {}, "auth_mode": {}, "chatgpt_account_id": {}, "chatgpt_plan_type": {},
	"email": {}, "expired": {}, "last_refresh": {}, "name": {}, "plan_type": {},
	"platform": {}, "provider": {}, "service": {}, "type": {},
}

type TargetCompatibility struct {
	Compatible bool   `json:"compatible"`
	ReasonCode string `json:"reasonCode"`
}

type AuthAnalysisState struct {
	State      string `json:"state"`
	ReasonCode string `json:"reasonCode,omitempty"`
	Message    string `json:"message,omitempty"`
}

type AuthIdentitySummary struct {
	Provider      string            `json:"provider"`
	Type          string            `json:"type,omitempty"`
	Email         string            `json:"email,omitempty"`
	AccountID     string            `json:"accountId,omitempty"`
	Recognized    []string          `json:"recognizedFields"`
	LogicalID     string            `json:"-"`
	CredentialSet map[string]string `json:"-"`
}

type AuthAnalysis struct {
	Version       string                                `json:"version"`
	Format        string                                `json:"format"`
	Identity      AuthIdentitySummary                   `json:"identity"`
	Compatibility map[gateways.Kind]TargetCompatibility `json:"compatibility"`
	Duplicate     AuthAnalysisState                     `json:"duplicate"`
	Conflict      AuthAnalysisState                     `json:"conflict"`
	Canonical     []byte                                `json:"-"`
	Digest        string                                `json:"digest"`
}

// AnalyzeAuthJSON performs canonical, side-effect-free validation and returns
// only masked, allowlisted identity metadata. Credential values remain private.
func AnalyzeAuthJSON(data []byte) (AuthAnalysis, error) {
	var raw map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil || raw == nil {
		return AuthAnalysis{}, errors.New("uploaded file must contain a JSON object")
	}
	if _, err := ParseJSON(data, "analysis.json", time.Now()); err != nil {
		return AuthAnalysis{}, sanitizeAnalysisError(err)
	}
	canonical, err := json.Marshal(raw)
	if err != nil {
		return AuthAnalysis{}, errors.New("uploaded JSON could not be canonicalized")
	}
	digest := sha256.Sum256(canonical)
	isBundle := gateways.IsSub2APIDataPackage(data)
	identityRaw := raw
	format := "cpa-auth"
	if isBundle {
		format = "sub2api-data"
		account, _ := singleSub2APIDataAccount(raw, true)
		identityRaw, _ = account["credentials"].(map[string]any)
		for _, key := range []string{"platform", "type", "name"} {
			if _, ok := identityRaw[key]; !ok {
				identityRaw[key] = account[key]
			}
		}
	}
	fields := make([]string, 0)
	values := make(map[string]string)
	for key := range safeAuthFields {
		if value := stringValue(identityRaw, key); value != "" {
			fields = append(fields, key)
			values[key] = value
		}
	}
	sort.Strings(fields)
	provider := normalizeProvider(firstNonEmpty(values["provider"], values["platform"], values["service"], values["type"]))
	credentialSet := collectCredentialIdentity(raw, isBundle)
	logicalID := logicalCredentialID(provider, credentialSet)
	if logicalID == "" {
		logicalID = "sha256:" + hex.EncodeToString(digest[:])
	}
	analysis := AuthAnalysis{
		Version: AuthAnalysisVersion, Format: format, Canonical: canonical,
		Digest: "sha256:" + hex.EncodeToString(digest[:]),
		Identity: AuthIdentitySummary{
			Provider: provider, Type: values["type"], Email: maskEmail(values["email"]),
			AccountID:  maskOpaque(firstNonEmpty(values["account_id"], values["chatgpt_account_id"])),
			Recognized: fields, LogicalID: logicalID, CredentialSet: credentialSet,
		},
		Compatibility: map[gateways.Kind]TargetCompatibility{},
		Duplicate:     AuthAnalysisState{State: "none", ReasonCode: "no_exact_duplicate"},
		Conflict:      AuthAnalysisState{State: "none", ReasonCode: "no_active_assignment_conflict"},
	}
	if isBundle {
		analysis.Compatibility[gateways.KindCPA] = TargetCompatibility{ReasonCode: "sub2api_package_requires_sub2api"}
		analysis.Compatibility[gateways.KindSub2API] = TargetCompatibility{Compatible: true, ReasonCode: "compatible_sub2api_package"}
	} else {
		analysis.Compatibility[gateways.KindCPA] = TargetCompatibility{Compatible: true, ReasonCode: "compatible_cpa_auth"}
		if hasImportCredential(raw) {
			analysis.Compatibility[gateways.KindSub2API] = TargetCompatibility{Compatible: true, ReasonCode: "compatible_codex_session"}
		} else {
			analysis.Compatibility[gateways.KindSub2API] = TargetCompatibility{ReasonCode: "missing_supported_credential"}
		}
	}
	return analysis, nil
}

func collectCredentialIdentity(raw map[string]any, bundle bool) map[string]string {
	if bundle {
		account, _ := singleSub2APIDataAccount(raw, true)
		raw, _ = account["credentials"].(map[string]any)
	}
	result := make(map[string]string)
	var walk func(map[string]any)
	walk = func(object map[string]any) {
		for key, value := range object {
			normalized := strings.ToLower(strings.TrimSpace(key))
			if nested, ok := value.(map[string]any); ok {
				walk(nested)
				continue
			}
			switch normalized {
			case "account_id", "chatgpt_account_id", "chatgpt_user_id", "agent_runtime_id", "email":
				if text := strings.ToLower(strings.TrimSpace(fmt.Sprint(value))); text != "" {
					result[normalized] = text
				}
			}
		}
	}
	walk(raw)
	return result
}

func logicalCredentialID(provider string, values map[string]string) string {
	for _, key := range []string{"account_id", "chatgpt_account_id", "chatgpt_user_id", "agent_runtime_id", "email"} {
		if value := values[key]; value != "" {
			sum := sha256.Sum256([]byte(AuthAnalysisVersion + "\x00" + provider + "\x00" + key + "\x00" + value))
			return "sha256:" + hex.EncodeToString(sum[:])
		}
	}
	return ""
}

func hasImportCredential(raw map[string]any) bool {
	secretNames := map[string]struct{}{"access_token": {}, "refresh_token": {}, "api_key": {}, "agent_private_key": {}, "id_token": {}}
	var found bool
	var walk func(any)
	walk = func(value any) {
		if found {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if _, ok := secretNames[strings.ToLower(strings.TrimSpace(key))]; ok && strings.TrimSpace(fmt.Sprint(child)) != "" {
					found = true
					return
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(raw)
	return found
}

func maskEmail(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return string([]rune(parts[0])[0]) + "***@" + parts[1]
}

func maskOpaque(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 4 {
		return "***"
	}
	return string(runes[:2]) + "***" + string(runes[len(runes)-2:])
}

func sanitizeAnalysisError(err error) error {
	message := err.Error()
	for _, safe := range []string{"exactly one account", "account must be a JSON object", "missing credentials", "JSON root must be an object", "invalid JSON"} {
		if strings.Contains(message, safe) {
			return errors.New(safe)
		}
	}
	return errors.New("unsupported authentication JSON")
}

func ParseJSON(data []byte, relativePath string, now time.Time) (model.Subscription, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return model.Subscription{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if raw == nil {
		return model.Subscription{}, fmt.Errorf("JSON root must be an object")
	}
	isSub2APIData := gateways.IsSub2APIDataPackage(data)
	bundleAccount, err := singleSub2APIDataAccount(raw, isSub2APIData)
	if err != nil {
		return model.Subscription{}, err
	}
	relativePath = filepath.ToSlash(relativePath)
	s := model.Subscription{
		ID: stableID(relativePath), FileName: filepath.Base(relativePath),
		Folder: filepath.ToSlash(filepath.Dir(relativePath)), RelativePath: relativePath,
		Email: stringValue(raw, "email"), Name: stringValue(raw, "name"),
		AccountID: stringValue(raw, "account_id"), ChatGPTAccountID: stringValue(raw, "chatgpt_account_id"),
		PlanType: stringValue(raw, "plan_type"), ChatGPTPlanType: stringValue(raw, "chatgpt_plan_type"),
		Expired: stringValue(raw, "expired"), LastRefresh: stringValue(raw, "last_refresh"),
		Type: stringValue(raw, "type"), BaseURL: stringValue(raw, "base_url"),
		OrderURL: stringValue(raw, "order_url"), Connectivity: model.Connectivity{Status: "unknown"},
		Status: "unknown",
	}
	s.Provider = normalizeProvider(firstNonEmpty(stringValue(raw, "provider"), stringValue(raw, "service"), s.Type))
	if isSub2APIData {
		credentials, _ := bundleAccount["credentials"].(map[string]any)
		extra, _ := bundleAccount["extra"].(map[string]any)
		s.Email = firstNonEmpty(stringValue(credentials, "email"), stringValue(extra, "email"), stringValue(bundleAccount, "name"))
		s.Name = firstNonEmpty(stringValue(bundleAccount, "name"), s.Email)
		s.AccountID = firstNonEmpty(stringValue(credentials, "account_id"), stringValue(extra, "account_id"))
		s.ChatGPTAccountID = firstNonEmpty(stringValue(credentials, "chatgpt_account_id"), stringValue(extra, "chatgpt_account_id"))
		s.PlanType = firstNonEmpty(stringValue(credentials, "plan_type"), stringValue(extra, "plan_type"))
		s.ChatGPTPlanType = firstNonEmpty(stringValue(credentials, "chatgpt_plan_type"), stringValue(extra, "chatgpt_plan_type"))
		s.Expired = firstNonEmpty(stringValue(credentials, "expired"), stringValue(extra, "expired"))
		s.LastRefresh = firstNonEmpty(stringValue(credentials, "last_refresh"), stringValue(extra, "last_refresh"))
		s.Type = firstNonEmpty(stringValue(bundleAccount, "type"), "oauth")
		s.Provider = normalizeProvider(firstNonEmpty(stringValue(bundleAccount, "platform"), "openai"))
	}
	if s.Folder == "." {
		s.Folder = ""
	}
	s.Balance = firstNumber(raw, "balance", "remaining_quota", "remaining", "credits")
	s.AcquisitionPrice = firstNumber(raw, "acquisition_price", "acquisitionPrice")
	if expires, ok := parseTime(s.Expired); ok {
		delta := expires.Sub(now).Hours() / 24
		days := 0
		if delta > 0 {
			days = int(math.Ceil(delta))
			s.Status = "active"
		} else {
			s.Status = "expired"
		}
		s.RemainingDays = &days
	}
	return s, nil
}

// singleSub2APIDataAccount projects Sub2API's portable data export onto the
// one-archive-file/one-runtime-account model used by Orbit. Multi-account
// exports must be split so every remote account retains an independent binding.
func singleSub2APIDataAccount(raw map[string]any, isSub2APIData bool) (map[string]any, error) {
	if !isSub2APIData {
		return nil, nil
	}
	accounts, ok := raw["accounts"].([]any)
	if !ok || len(accounts) != 1 {
		return nil, fmt.Errorf("Sub2API data import must contain exactly one account; got %d", len(accounts))
	}
	account, ok := accounts[0].(map[string]any)
	if !ok || account == nil {
		return nil, errors.New("Sub2API data account must be a JSON object")
	}
	credentials, ok := account["credentials"].(map[string]any)
	if !ok || len(credentials) == 0 {
		return nil, errors.New("Sub2API data account is missing credentials")
	}
	return account, nil
}

func normalizeProvider(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, "._- ")
	if value == "" {
		return "unknown"
	}
	value = unsafeProviderRE.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}

func stableID(relativePath string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.ToSlash(relativePath))))
	return hex.EncodeToString(sum[:8])
}

func stringValue(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	s, ok := value.(string)
	if ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNumber(raw map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		var n float64
		switch v := value.(type) {
		case float64:
			n = v
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				continue
			}
			n = parsed
		default:
			continue
		}
		return &n
	}
	return nil
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
