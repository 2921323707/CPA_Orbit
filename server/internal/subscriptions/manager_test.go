package subscriptions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/model"
)

func TestSanitizeJSONNamePreventsTraversal(t *testing.T) {
	name := sanitizeJSONName(`../../evil.json`)
	if name != "evil.json" || filepath.Base(name) != name {
		t.Fatalf("unsafe sanitized name: %q", name)
	}
}

func TestSafeArchivedPathRejectsTraversalAndSymlink(t *testing.T) {
	root := t.TempDir()
	if _, err := safeArchivedPath(root, "../outside.json"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	target := filepath.Join(root, "ok.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := safeArchivedPath(root, "ok.json"); err != nil {
		t.Fatal(err)
	}
}

func TestModelsEndpoint(t *testing.T) {
	for input, want := range map[string]string{
		"http://127.0.0.1:8317/v1":  "http://127.0.0.1:8317/v1/models",
		"http://127.0.0.1:8317/v1/": "http://127.0.0.1:8317/v1/models",
		"http://127.0.0.1:8317":     "http://127.0.0.1:8317/v1/models",
	} {
		if got := modelsEndpoint(input); got != want {
			t.Errorf("modelsEndpoint(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestSyncBytesToAuthDirKeepsDistinctJSONForSameAccount(t *testing.T) {
	authDir := t.TempDir()
	existingPath := filepath.Join(authDir, "mature-name.json")
	if err := os.WriteFile(existingPath, []byte(`{"account_id":"account-1","email":"old@example.com","access_token":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	incoming := []byte(`{"account_id":"account-1","email":"new@example.com","access_token":"new"}`)
	target, err := syncBytesToAuthDir(incoming, "new@example.com", authDir)
	if err != nil {
		t.Fatal(err)
	}
	if target == existingPath {
		t.Fatalf("distinct JSON reused existing path %q", target)
	}
	entries, err := os.ReadDir(authDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected two distinct auth files; entries=%d", len(entries))
	}
}

func TestSyncBytesToAuthDirUsesProviderAwareFilename(t *testing.T) {
	authDir := t.TempDir()
	incoming := []byte(`{"provider":"claude","email":"person@example.com","access_token":"new"}`)
	target, err := syncBytesToAuthDir(incoming, "person@example.com", authDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(target) != "claude_oauth_person_example.com.json" {
		t.Fatalf("unexpected provider-aware target: %q", target)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
	if files := func() []string {
		entries, _ := os.ReadDir(authDir)
		result := make([]string, 0, len(entries))
		for _, entry := range entries {
			result = append(result, entry.Name())
		}
		return result
	}(); len(files) != 1 {
		t.Fatalf("temporary file leaked into auth-dir: %v", files)
	}
}

func TestSyncBytesToAuthDirRejectsAmbiguousDuplicateAccount(t *testing.T) {
	authDir := t.TempDir()
	for _, name := range []string{"one.json", "two.json"} {
		if err := os.WriteFile(filepath.Join(authDir, name), []byte(`{"account_id":"duplicate","email":"same@example.com"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, err := syncBytesToAuthDir([]byte(`{"account_id":"duplicate","email":"same@example.com"}`), "same@example.com", authDir)
	if err == nil || !strings.Contains(err.Error(), "same JSON") {
		t.Fatalf("expected exact JSON duplicate rejection, got %v", err)
	}
}

func TestParseUsageQuotaFiveHourAndSevenDay(t *testing.T) {
	body := `{
		"plan_type":"k12",
		"rate_limit":{"allowed":true,"limit_reached":false,
			"primary_window":{"used_percent":25,"limit_window_seconds":18000,"reset_after_seconds":60},
			"secondary_window":{"used_percent":"40","limit_window_seconds":604800,"reset_at":1780000000}},
		"credits":{"has_credits":true,"unlimited":false,"balance":"12.5"}
	}`
	checkedAt := time.Unix(1770000000, 0).UTC()
	quota, err := parseUsageQuota(body, checkedAt)
	if err != nil {
		t.Fatal(err)
	}
	if quota.FiveHour == nil || quota.FiveHour.RemainingPercent == nil || *quota.FiveHour.RemainingPercent != 75 {
		t.Fatalf("unexpected five-hour quota: %+v", quota.FiveHour)
	}
	if quota.SevenDay == nil || quota.SevenDay.RemainingPercent == nil || *quota.SevenDay.RemainingPercent != 60 {
		t.Fatalf("unexpected seven-day quota: %+v", quota.SevenDay)
	}
	if quota.CreditsBalance == nil || *quota.CreditsBalance != 12.5 || !quota.Credits {
		t.Fatalf("unexpected credits: %+v", quota)
	}
	if !quota.FiveHour.ResetAt.Equal(checkedAt.Add(time.Minute)) {
		t.Fatalf("unexpected calculated reset: %v", quota.FiveHour.ResetAt)
	}
}

func TestMatchCPAAuthPrefersAccountThenEmail(t *testing.T) {
	entries := []cpaAuthEntry{
		{AuthIndex: "one", Account: "account-1", Email: "first@example.com"},
		{AuthIndex: "two", Account: "account-2", Email: "second@example.com"},
	}
	matched, reason := matchCPAAuth(entries, model.Subscription{AccountID: "account-2", Email: "wrong@example.com"})
	if reason != "" || matched.AuthIndex != "two" {
		t.Fatalf("account match failed: %+v %q", matched, reason)
	}
	matched, reason = matchCPAAuth(entries, model.Subscription{Email: "FIRST@example.com"})
	if reason != "" || matched.AuthIndex != "one" {
		t.Fatalf("email match failed: %+v %q", matched, reason)
	}
	_, reason = matchCPAAuth(entries, model.Subscription{Email: "missing@example.com"})
	if reason != "not_in_cpa_pool" {
		t.Fatalf("unexpected missing reason: %q", reason)
	}
}

func TestMatchCPAAuthRespectsProvider(t *testing.T) {
	entries := []cpaAuthEntry{
		{AuthIndex: "claude", Provider: "claude", Email: "same@example.com"},
		{AuthIndex: "codex", Provider: "codex", Email: "same@example.com"},
	}
	matched, reason := matchCPAAuth(entries, model.Subscription{Provider: "claude", Email: "same@example.com"})
	if reason != "" || matched.AuthIndex != "claude" {
		t.Fatalf("provider match failed: %+v %q", matched, reason)
	}
}

func TestSubscriptionCategory(t *testing.T) {
	zero, half := 0.0, 50.0
	for name, test := range map[string]struct {
		item model.Subscription
		want string
	}{
		"normal":              {item: model.Subscription{Connectivity: model.Connectivity{Status: "ok"}}, want: "normal"},
		"pending":             {item: model.Subscription{Connectivity: model.Connectivity{Status: "unknown"}}, want: "pending"},
		"five-hour exhausted": {item: model.Subscription{Connectivity: model.Connectivity{Status: "quota_exhausted", Quota: &model.UsageQuota{FiveHour: &model.QuotaWindow{RemainingPercent: &zero}, SevenDay: &model.QuotaWindow{RemainingPercent: &half}}}}, want: "normal"},
		"other error":         {item: model.Subscription{Connectivity: model.Connectivity{Status: "payment_required"}}, want: "error"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := subscriptionCategory(test.item); got != test.want {
				t.Fatalf("category=%q, want %q", got, test.want)
			}
		})
	}
}

func TestPageReturnsTenItemsAndFolders(t *testing.T) {
	m := &Manager{items: make(map[string]model.Subscription)}
	totalCost := 0.0
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("id-%02d", i)
		price := float64(i + 1)
		days := i
		totalCost += price
		m.items[id] = model.Subscription{ID: id, Email: fmt.Sprintf("user%02d@example.com", i), Folder: fmt.Sprintf("071%d", i%3), RelativePath: fmt.Sprintf("071%d/%02d.json", i%3, i), AcquisitionPrice: &price, RemainingDays: &days, Connectivity: model.Connectivity{Status: "ok"}}
	}
	result := m.Page("", "normal", "", 2, 10)
	if result.Total != 25 || len(result.Subscriptions) != 10 || result.Page != 2 || result.TotalPages != 3 {
		t.Fatalf("unexpected page: %+v", result)
	}
	if len(result.Folders) != 3 {
		t.Fatalf("unexpected folders: %v", result.Folders)
	}
	if result.Insights.Normal != 25 || result.Insights.Error != 0 || result.Insights.Priced != 25 || result.Insights.ExpiringSoon != 8 {
		t.Fatalf("unexpected insights: %+v", result.Insights)
	}
	if result.Insights.TotalCost != totalCost || result.Insights.AverageCost != totalCost/25 {
		t.Fatalf("unexpected cost insights: %+v", result.Insights)
	}
}

func TestDuplicateSubscriptionAndDelete(t *testing.T) {
	root := t.TempDir()
	checksPath := filepath.Join(t.TempDir(), "checks.json")
	path := filepath.Join(root, "0718", "one.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"email":"one@example.com","type":"codex"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	id := stableID("0718/one.json")
	m := &Manager{root: root, checksPath: checksPath, items: map[string]model.Subscription{id: {ID: id, Email: "one@example.com", RelativePath: "0718/one.json"}}, checks: map[string]model.Connectivity{id: {Status: "error"}}}
	exact := []byte(`{"email":"one@example.com","type":"codex"}`)
	if _, duplicate, err := m.duplicateSubscription(exact); err != nil || !duplicate {
		t.Fatalf("expected exact JSON duplicate detection, duplicate=%v err=%v", duplicate, err)
	}
	distinct := []byte(`{"email":"one@example.com","type":"codex","access_token":"different"}`)
	if _, duplicate, err := m.duplicateSubscription(distinct); err != nil || duplicate {
		t.Fatalf("unexpected partial JSON duplicate detection, duplicate=%v err=%v", duplicate, err)
	}
	if err := m.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("archive file still exists: %v", err)
	}
	if _, ok := m.Get(id); ok {
		t.Fatal("deleted subscription remains in memory")
	}
}

func TestImportAllowsPartialJSONDifferences(t *testing.T) {
	root := t.TempDir()
	settings, err := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{root: root, checksPath: filepath.Join(t.TempDir(), "checks.json"), settings: settings, items: make(map[string]model.Subscription), checks: make(map[string]model.Connectivity)}
	first := []byte(`{"email":"same@example.com","account_id":"same-id","access_token":"one"}`)
	second := []byte(`{"email":"same@example.com","account_id":"same-id","access_token":"two"}`)
	if _, _, err := m.Import(first, "first.json", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Import(second, "second.json", "", ""); err != nil {
		t.Fatalf("partial JSON difference should import as a distinct archive: %v", err)
	}
	if got := len(m.List("", "", "")); got != 2 {
		t.Fatalf("expected two distinct archives, got %d", got)
	}
	if _, _, err := m.Import(first, "duplicate.json", "", ""); !errors.Is(err, ErrDuplicateSubscription) {
		t.Fatalf("expected exact JSON duplicate, got %v", err)
	}
}

func TestImportArchivesByGatewayProviderAndDate(t *testing.T) {
	root := t.TempDir()
	settings, err := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := &Manager{root: root, checksPath: filepath.Join(t.TempDir(), "checks.json"), settings: settings, items: make(map[string]model.Subscription), checks: make(map[string]model.Connectivity)}
	date := time.Now().Format("0102")

	sub2apiItem, _, err := m.ImportWithOptions([]byte(`{"email":"sub2api@example.com","access_token":"one"}`), "sub2api.json", ImportOptions{ArchiveProvider: "sub2api", SkipLegacySync: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.ToSlash(filepath.Join("sub2api", date, "sub2api.json")); sub2apiItem.RelativePath != want {
		t.Fatalf("Sub2API archive path=%q, want %q", sub2apiItem.RelativePath, want)
	}

	cpaItem, _, err := m.ImportWithOptions([]byte(`{"email":"cpa@example.com","access_token":"two"}`), "cpa.json", ImportOptions{ArchiveProvider: "cpa", SkipLegacySync: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.ToSlash(filepath.Join("cpa", date, "cpa.json")); cpaItem.RelativePath != want {
		t.Fatalf("CPA archive path=%q, want %q", cpaItem.RelativePath, want)
	}
}
