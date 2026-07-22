package sub2api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cpa-monitor/server/internal/gateways"
)

func TestClientImplementsGatewayAdapter(t *testing.T) {
	var _ gateways.Adapter = (*Client)(nil)
}

func TestDeployImportsCodexSessionWithAdminKey(t *testing.T) {
	credentialJSON := `{"tokens":{"access_token":"secret-access","refresh_token":"secret-refresh"},"email":"plus@example.com"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/accounts/import/codex-session" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "admin-secret" {
			t.Fatalf("x-api-key=%q", got)
		}
		var request CodexSessionImportRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Content != credentialJSON || !request.UpdateExisting || len(request.GroupIDs) != 2 || request.GroupIDs[0] != 3 {
			t.Fatalf("unexpected import request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":0,"message":"success","data":{"total":1,"created":1,"updated":0,"skipped":0,"failed":0,"items":[{"index":0,"action":"created","account_id":42}]}}`)
	}))
	defer server.Close()

	client := NewClient(func() Config { return Config{BaseURL: server.URL + "/api/v1", AdminKey: "admin-secret"} })
	result, err := client.Deploy(context.Background(), gateways.Credential{SubscriptionID: "sub-1", Email: "plus@example.com", Data: []byte(credentialJSON)}, gateways.DeployOptions{UpdateExisting: true, GroupIDs: []int64{3, 5}, Concurrency: 2, Priority: 8})
	if err != nil {
		t.Fatal(err)
	}
	if result.Binding.ExternalID != "42" || !result.Binding.Managed || result.Status != "deployed" {
		t.Fatalf("unexpected deployment: %+v", result)
	}
}

func TestUsageRangeUsesSub2APIDateFormatAndUnwrapsPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("start_date"); got != "2026-07-21" {
			t.Fatalf("start_date=%q", got)
		}
		if got := r.URL.Query().Get("end_date"); got != "2026-07-22" {
			t.Fatalf("end_date=%q", got)
		}
		fmt.Fprint(w, `{"code":0,"message":"success","data":{"items":[{"account_id":42}],"total":1,"page":1,"page_size":100,"pages":1}}`)
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	from, _ := time.Parse(time.RFC3339, "2026-07-21T23:15:00Z")
	to, _ := time.Parse(time.RFC3339, "2026-07-22T01:00:00Z")
	result, err := client.UsageRange(context.Background(), 1, 100, from, to)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Pages != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected usage page: %+v", result)
	}
}

func TestInspectTestsAccountAndReadsQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/accounts/42/test":
			fmt.Fprint(w, `{"success":true,"message":"ok","latency_ms":17}`)
		case "/api/v1/admin/openai/accounts/42/quota":
			fmt.Fprint(w, `{"account_id":"acct","plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":25,"limit_window_seconds":18000},"secondary_window":{"used_percent":40,"limit_window_seconds":604800}},"fetched_at":1780000000}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	result, err := client.Inspect(context.Background(), gateways.BindingRef{ExternalID: "42"}, gateways.Credential{})
	if err != nil {
		t.Fatal(err)
	}
	quota := result.Connectivity.Quota
	if result.Connectivity.Status != "ok" || result.Connectivity.LatencyMS != 17 || quota == nil || quota.PlanType != "plus" || quota.FiveHour == nil || *quota.FiveHour.RemainingPercent != 75 {
		t.Fatalf("unexpected inspect result: %+v", result)
	}
}

func TestDetachOnlyDeletesManagedBinding(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/admin/accounts/42" {
			t.Fatalf("unexpected delete %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"message":"deleted"}`)
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	if err := client.Detach(context.Background(), gateways.BindingRef{ExternalID: "42"}, gateways.Credential{}); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatal("unmanaged binding was deleted")
	}
	if err := client.Detach(context.Background(), gateways.BindingRef{ExternalID: "42", Managed: true}, gateways.Credential{}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("managed delete requests=%d", requests)
	}
}

func TestErrorsNeverExposeCredentialsOrAdminKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"bad credential secret-access admin-secret"}`)
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "admin-secret"} })
	_, err := client.Deploy(context.Background(), gateways.Credential{Data: []byte(`{"access_token":"secret-access"}`)}, gateways.DeployOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret-access") || strings.Contains(err.Error(), "admin-secret") {
		t.Fatalf("secret leaked in error: %v", err)
	}
}

func TestClientRefusesRedirects(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("redirect destination must not be called")
	}))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	if _, err := client.ListAccounts(context.Background(), 1, 1, ""); err == nil {
		t.Fatal("expected redirect rejection")
	}
}
