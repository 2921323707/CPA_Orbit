package subscriptions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cpa-monitor/server/internal/model"
)

var unsafeProviderRE = regexp.MustCompile(`[^a-z0-9]+`)

func ParseJSON(data []byte, relativePath string, now time.Time) (model.Subscription, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return model.Subscription{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if raw == nil {
		return model.Subscription{}, fmt.Errorf("JSON root must be an object")
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
