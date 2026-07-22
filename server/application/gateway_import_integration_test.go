package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestGatewayImportIntegration(t *testing.T) {
	var mu sync.Mutex
	unauthorized := false
	var importedContents []string
	var deletedAccounts []string

	sub2api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "sub2api-test-key" {
			t.Errorf("Sub2API x-api-key = %q", got)
		}
		mu.Lock()
		defer mu.Unlock()
		if unauthorized && r.URL.Path == "/api/v1/admin/accounts" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"message":"fixture-admin-secret fixture-access-token"}`)
			return
		}
		switch r.URL.Path {
		case "/api/v1/admin/accounts":
			_, _ = io.WriteString(w, `{"items":[],"total":0,"page":1,"page_size":1,"pages":0}`)
		case "/api/v1/admin/accounts/import/codex-session":
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode import request: %v", err)
			}
			importedContents = append(importedContents, body.Content)
			if strings.Contains(body.Content, "base_url") {
				t.Error("credential routing metadata was forwarded to Sub2API")
			}
			_, _ = io.WriteString(w, `{"code":0,"data":{"total":1,"created":1,"failed":0,"items":[{"action":"created","account_id":42}]}}`)
		case "/api/v1/admin/accounts/42/test":
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			_, _ = io.WriteString(w, "data: {\"type\":\"test_start\",\"model\":\"gpt-5.4\"}\n\ndata: {\"type\":\"test_complete\",\"success\":true}\n\n")
		case "/api/v1/admin/openai/accounts/42/quota":
			_, _ = io.WriteString(w, `{"account_id":"acct-fixture-123","plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":25,"limit_window_seconds":18000},"secondary_window":{"used_percent":40,"limit_window_seconds":604800}}}`)
		case "/api/v1/admin/accounts/42":
			deletedAccounts = append(deletedAccounts, "42")
			_, _ = io.WriteString(w, `{"message":"deleted"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer sub2api.Close()

	cpa := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Management-Key") != "cpa-test-key" {
			t.Errorf("CPA management key was not sent")
		}
		if r.URL.Path != "/v0/management/auth-files" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"files":[]}`)
	}))
	defer cpa.Close()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	settings := fmt.Sprintf(`{"baseUrl":%q,"cpaManagementKey":"cpa-test-key"}`, cpa.URL+"/v1")
	if err := os.WriteFile(filepath.Join(root, "data", "settings.json"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	// The seeded CPA target exercises the CPA management endpoint without touching
	// any repository credential archive.
	cpaTargetID := gatewayTargetID(t, runtime.Handler(), "Legacy CPA")
	cpaTest := requestJSON(t, runtime.Handler(), http.MethodPost, fmt.Sprintf("/api/gateways/targets/%d/test", cpaTargetID), nil, "")
	if cpaTest.Code != http.StatusOK || !strings.Contains(cpaTest.Body.String(), `"status":"ok"`) {
		t.Fatalf("CPA target test status=%d body=%s", cpaTest.Code, cpaTest.Body.String())
	}

	create := requestJSON(t, runtime.Handler(), http.MethodPost, "/api/gateways/targets", map[string]any{
		"kind": "sub2api", "name": "Fixture Sub2API", "baseUrl": sub2api.URL,
		"adminKey": "sub2api-test-key", "enabled": true, "primary": true,
		"defaultGroupIds": []int64{3}, "defaultConcurrency": 2, "defaultPriority": 4, "rateMultiplier": 1,
	}, "sub2api-test-key")
	if create.Code != http.StatusOK || strings.Contains(create.Body.String(), "sub2api-test-key") {
		t.Fatalf("target create status=%d body=%s", create.Code, create.Body.String())
	}
	var target struct {
		ID int64 `json:"id"`
	}
	decodeResponse(t, create, &target)
	if target.ID <= 0 {
		t.Fatal("target ID was not returned")
	}

	unauthorized = true
	failedTest := requestJSON(t, runtime.Handler(), http.MethodPost, fmt.Sprintf("/api/gateways/targets/%d/test", target.ID), nil, "")
	if failedTest.Code != http.StatusOK || strings.Contains(failedTest.Body.String(), "fixture-admin-secret") || strings.Contains(failedTest.Body.String(), "fixture-access-token") {
		t.Fatalf("unsafe target failure response status=%d body=%s", failedTest.Code, failedTest.Body.String())
	}
	unauthorized = false
	goodTest := requestJSON(t, runtime.Handler(), http.MethodPost, fmt.Sprintf("/api/gateways/targets/%d/test", target.ID), nil, "")
	if goodTest.Code != http.StatusOK || !strings.Contains(goodTest.Body.String(), `"status":"ok"`) {
		t.Fatalf("target test status=%d body=%s", goodTest.Code, goodTest.Body.String())
	}

	bundleFixture, err := os.ReadFile(filepath.Join("..", "testdata", "gateway-import", "sub2api-data.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundlePreflight := requestMultipart(t, runtime.Handler(), "/api/subscriptions/import/preflight", bundleFixture, "sub2api-data.json", "")
	if bundlePreflight.Code != http.StatusOK || !strings.Contains(bundlePreflight.Body.String(), `"format":"sub2api-data"`) {
		t.Fatalf("bundle preflight status=%d body=%s", bundlePreflight.Code, bundlePreflight.Body.String())
	}

	fixture, err := os.ReadFile(filepath.Join("..", "testdata", "gateway-import", "codex-session.json"))
	if err != nil {
		t.Fatal(err)
	}
	preflight := requestMultipart(t, runtime.Handler(), "/api/subscriptions/import/preflight", fixture, "codex-session.json", "")
	if preflight.Code != http.StatusOK {
		t.Fatalf("preflight status=%d body=%s", preflight.Code, preflight.Body.String())
	}
	var pf struct {
		OperationID string `json:"operationId"`
		Token       string `json:"preflightToken"`
		Targets     []struct {
			ID         int64 `json:"targetId"`
			Compatible bool  `json:"compatible"`
		} `json:"targets"`
	}
	decodeResponse(t, preflight, &pf)
	if pf.OperationID == "" || pf.Token == "" || !targetCompatible(pf.Targets, target.ID) {
		t.Fatalf("unexpected preflight: %+v", pf)
	}

	commitPath := "/api/subscriptions/import/commit?targetId=" + url.QueryEscape(fmt.Sprint(target.ID)) + "&acquisitionPrice=12.50"
	commit := requestMultipart(t, runtime.Handler(), commitPath, fixture, "codex-session.json", pf.Token)
	if commit.Code != http.StatusCreated || strings.Contains(commit.Body.String(), "fixture-access-token") {
		t.Fatalf("commit status=%d body=%s", commit.Code, commit.Body.String())
	}
	var result struct {
		Subscription struct {
			ID           string `json:"id"`
			RelativePath string `json:"relativePath"`
		} `json:"subscription"`
		Deployment struct {
			RemoteAccountID string `json:"remoteAccountId"`
			TargetID        int64  `json:"targetId"`
		} `json:"deployment"`
		Idempotent bool `json:"idempotent"`
	}
	decodeResponse(t, commit, &result)
	if result.Subscription.ID == "" || !strings.HasPrefix(result.Subscription.RelativePath, "sub2api/") || result.Deployment.RemoteAccountID != "42" || result.Deployment.TargetID != target.ID || result.Idempotent {
		t.Fatalf("unexpected commit result: %+v", result)
	}

	idempotent := requestMultipart(t, runtime.Handler(), commitPath, fixture, "codex-session.json", pf.Token)
	if idempotent.Code != http.StatusOK || !strings.Contains(idempotent.Body.String(), `"idempotent":true`) {
		t.Fatalf("idempotent commit status=%d body=%s", idempotent.Code, idempotent.Body.String())
	}

	testPath := "/api/subscriptions/" + url.PathEscape(result.Subscription.ID) + "/test"
	subscriptionTest := requestJSON(t, runtime.Handler(), http.MethodPost, testPath, nil, "")
	if subscriptionTest.Code != http.StatusOK || !strings.Contains(subscriptionTest.Body.String(), `"status":"ok"`) || !strings.Contains(subscriptionTest.Body.String(), `"planType":"plus"`) {
		t.Fatalf("subscription test status=%d body=%s", subscriptionTest.Code, subscriptionTest.Body.String())
	}
	bindings := requestJSON(t, runtime.Handler(), http.MethodGet, "/api/subscriptions/"+url.PathEscape(result.Subscription.ID)+"/bindings", nil, "")
	if bindings.Code != http.StatusOK || !strings.Contains(bindings.Body.String(), `"remoteAccountId":"42"`) {
		t.Fatalf("bindings status=%d body=%s", bindings.Code, bindings.Body.String())
	}
	operations := requestJSON(t, runtime.Handler(), http.MethodGet, "/api/gateways/operations", nil, "")
	if operations.Code != http.StatusOK || !strings.Contains(operations.Body.String(), `"status":"succeeded"`) {
		t.Fatalf("operations status=%d body=%s", operations.Code, operations.Body.String())
	}

	archive := requestJSON(t, runtime.Handler(), http.MethodDelete, "/api/subscriptions/"+url.PathEscape(result.Subscription.ID), nil, "")
	if archive.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", archive.Code, archive.Body.String())
	}
	if len(deletedAccounts) != 1 || len(importedContents) != 1 {
		t.Fatalf("unexpected gateway calls imports=%d deletes=%d", len(importedContents), len(deletedAccounts))
	}
}

func gatewayTargetID(t *testing.T, handler http.Handler, name string) int64 {
	t.Helper()
	response := requestJSON(t, handler, http.MethodGet, "/api/gateways/targets", nil, "")
	var payload struct {
		Targets []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"targets"`
	}
	decodeResponse(t, response, &payload)
	for _, target := range payload.Targets {
		if target.Name == name {
			return target.ID
		}
	}
	t.Fatalf("target %q was not found", name)
	return 0
}

func targetCompatible(targets []struct {
	ID         int64 `json:"targetId"`
	Compatible bool  `json:"compatible"`
}, id int64) bool {
	for _, target := range targets {
		if target.ID == id {
			return target.Compatible
		}
	}
	return false
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, body any, secret string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if secret != "" {
		request.Header.Set("X-Test-Secret", secret)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func requestMultipart(t *testing.T, handler http.Handler, path string, data []byte, filename, token string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("X-Orbit-Preflight-Token", token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
}
