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

func TestDeployStripsLegacyBaseURLWithoutMutatingArchive(t *testing.T) {
	credentialJSON := `{"access_token":"secret-access","base_url":"http://127.0.0.1:8317/v1"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request CodexSessionImportRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var imported map[string]any
		if err := json.Unmarshal([]byte(request.Content), &imported); err != nil {
			t.Fatal(err)
		}
		if _, exists := imported["base_url"]; exists {
			t.Fatal("legacy CPA base_url was forwarded to Sub2API")
		}
		fmt.Fprint(w, `{"code":0,"data":{"total":1,"created":1,"failed":0,"items":[{"action":"created","account_id":42}]}}`)
	}))
	defer server.Close()

	archived := []byte(credentialJSON)
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	if _, err := client.Deploy(context.Background(), gateways.Credential{Data: archived}, gateways.DeployOptions{}); err != nil {
		t.Fatal(err)
	}
	if string(archived) != credentialJSON {
		t.Fatal("archived credential was mutated")
	}
}

func TestDeployExpandsSingleAccountSub2APIData(t *testing.T) {
	bundle := `{"exported_at":"2026-07-22T00:00:00Z","proxies":[],"accounts":[{"name":"agent@example.com","platform":"openai","type":"oauth","credentials":{"auth_mode":"agent_identity","email":"agent@example.com","agent_runtime_id":"runtime-1","agent_private_key":"private-key","account_id":"account-1","chatgpt_user_id":"user-1"},"extra":{"source":"import"},"concurrency":3,"priority":7,"rate_multiplier":1.25,"auto_pause_on_expired":true}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request CodexSessionImportRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var content map[string]any
		if err := json.Unmarshal([]byte(request.Content), &content); err != nil {
			t.Fatal(err)
		}
		if content["auth_mode"] != "agent_identity" || content["agent_runtime_id"] != "runtime-1" || content["agent_private_key"] != "private-key" {
			t.Fatalf("agent identity was not extracted: %#v", content)
		}
		if request.Name != "agent@example.com" || request.Concurrency != 3 || request.Priority != 7 || request.RateMultiplier != 1.25 || !request.AutoPauseOnExpired {
			t.Fatalf("account settings were not retained: %+v", request)
		}
		fmt.Fprint(w, `{"code":0,"data":{"total":1,"created":1,"failed":0,"items":[{"action":"created","account_id":402}]}}`)
	}))
	defer server.Close()

	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	result, err := client.Deploy(context.Background(), gateways.Credential{SubscriptionID: "sub-agent", Data: []byte(bundle)}, gateways.DeployOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Binding.ExternalID != "402" {
		t.Fatalf("unexpected binding: %+v", result)
	}
}

func TestDeployRejectsMultiAccountSub2APIData(t *testing.T) {
	bundle := []byte(`{"type":"sub2api-data","version":1,"accounts":[{"name":"one"},{"name":"two"}]}`)
	client := NewClient(func() Config { return Config{BaseURL: "http://127.0.0.1:8080", AdminKey: "key"} })
	if _, err := client.Deploy(context.Background(), gateways.Credential{Data: bundle}, gateways.DeployOptions{}); err == nil || !strings.Contains(err.Error(), "exactly one account") {
		t.Fatalf("expected safe multi-account rejection, got %v", err)
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

func TestInspectParsesSSEAccountTestAndReadsQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/accounts/42/test":
			if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
				t.Fatalf("Accept=%q", r.Header.Get("Accept"))
			}
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			fmt.Fprint(w, "data: {\"type\":\"test_start\",\"model\":\"gpt-5.4\"}\r\n\r\ndata: {\"type\":\"content\",\"text\":\"Hi\"}\r\n\r\ndata: {\"type\":\"test_complete\",\"success\":true}\r\n\r\n")
		case "/api/v1/admin/openai/accounts/42/quota":
			fmt.Fprint(w, `{"account_id":"acct","plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":25,"limit_window_seconds":18000}},"fetched_at":1780000000}`)
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
	if result.Connectivity.Status != "ok" || result.Connectivity.LatencyMS < 0 || result.Connectivity.Quota == nil || result.Connectivity.Quota.PlanType != "plus" {
		t.Fatalf("unexpected SSE inspect result: %+v", result)
	}
}

func TestInspectClassifiesSSEAccountNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"error\",\"error\":\"Account not found\"}\n\n")
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	result, err := client.Inspect(context.Background(), gateways.BindingRef{ExternalID: "42"}, gateways.Credential{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Connectivity.Status != "not_found" || result.Connectivity.ReasonCode != "sub2api_account_not_found" {
		t.Fatalf("unexpected missing result: %+v", result)
	}
}

func TestInspectSanitizesSSETestFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"error\",\"error\":\"secret-access upstream failure\"}\n\n")
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	result, err := client.Inspect(context.Background(), gateways.BindingRef{ExternalID: "42"}, gateways.Credential{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Connectivity.Status != "error" || strings.Contains(result.Connectivity.Error, "secret-access") {
		t.Fatalf("unsafe result: %+v", result)
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

func TestReconcileBindingUsesOrbitMarkerAndStrongIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":0,"data":{"items":[{"id":9,"platform":"openai","type":"oauth","credentials":{"orbit_subscription_id":"sub-1","chatgpt_account_id":"workspace-1","chatgpt_user_id":"user-1","email":"person@example.com"}}],"total":1,"page":1,"page_size":500,"pages":1}}`)
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	result, err := client.ReconcileBinding(context.Background(), gateways.BindingRef{ExternalID: "8", Managed: true}, gateways.Credential{
		SubscriptionID: "sub-1", Provider: "openai", CredentialSet: map[string]string{"chatgpt_account_id": "workspace-1", "chatgpt_user_id": "user-1", "email": "person@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "rebound" || result.Binding.ExternalID != "9" {
		t.Fatalf("unexpected reconciliation: %+v", result)
	}
}

func TestReconcileBindingRejectsEmailOnlyMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":0,"data":{"items":[{"id":9,"platform":"openai","type":"oauth","credentials":{"orbit_subscription_id":"sub-1","email":"person@example.com"}}],"total":1,"page":1,"page_size":500,"pages":1}}`)
	}))
	defer server.Close()
	client := NewClient(func() Config { return Config{BaseURL: server.URL, AdminKey: "key"} })
	result, err := client.ReconcileBinding(context.Background(), gateways.BindingRef{ExternalID: "8", Managed: true}, gateways.Credential{SubscriptionID: "sub-1", Provider: "openai", CredentialSet: map[string]string{"email": "person@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "missing" || result.Binding.ExternalID != "8" {
		t.Fatalf("email-only match was accepted: %+v", result)
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
