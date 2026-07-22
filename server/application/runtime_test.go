package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeCreatesPortableLayoutAndServesAPI(t *testing.T) {
	root := filepath.Join(t.TempDir(), "portable-data")
	runtime, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	if runtime.Root() != root {
		t.Fatalf("root = %q, want %q", runtime.Root(), root)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()
	runtime.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != "ok" {
		t.Fatalf("status payload = %q", payload.Status)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runtime.Start(ctx)
	cancel()
}

func TestGatewayTargetAPIStoresSecretWriteOnlyAndTestsSub2API(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sub2api-admin-secret" {
			t.Fatalf("missing Sub2API admin key")
		}
		if r.URL.Path != "/api/v1/admin/accounts" {
			t.Fatalf("unexpected upstream path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"total":0,"page":1,"page_size":1,"pages":0}`))
	}))
	defer upstream.Close()
	runtime, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	body := `{"kind":"sub2api","name":"Primary Sub2API","baseUrl":"` + upstream.URL + `","adminKey":"sub2api-admin-secret","enabled":true,"primary":true,"defaultGroupIds":[3],"defaultConcurrency":2,"defaultPriority":5,"rateMultiplier":1}`
	request := httptest.NewRequest(http.MethodPost, "/api/gateways/targets", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	runtime.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "sub2api-admin-secret") {
		t.Fatalf("target create status=%d body=%s", response.Code, response.Body.String())
	}
	var target struct {
		ID                 int64 `json:"id"`
		AdminKeyConfigured bool  `json:"adminKeyConfigured"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &target); err != nil {
		t.Fatal(err)
	}
	if target.ID == 0 || !target.AdminKeyConfigured {
		t.Fatalf("unexpected target response: %+v", target)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/gateways/targets/"+fmt.Sprint(target.ID)+"/test", nil)
	response = httptest.NewRecorder()
	runtime.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("target test status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDesktopHandlerTrustsOnlyInProcessAPIRoutes(t *testing.T) {
	runtime, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Origin", "wails://wails")
	response := httptest.NewRecorder()
	runtime.DesktopHandler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("desktop API status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/index.html", nil)
	response = httptest.NewRecorder()
	runtime.DesktopHandler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("non-API status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRuntimeRejectsEmptyRoot(t *testing.T) {
	if _, err := New("  "); err == nil {
		t.Fatal("expected an error for an empty root")
	}
}
