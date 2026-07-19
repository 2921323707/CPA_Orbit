package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestRuntimeCreatesPortableLayoutAndServesAPI(t *testing.T) {
	root := filepath.Join(t.TempDir(), "portable-data")
	runtime, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
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

func TestDesktopHandlerTrustsOnlyInProcessAPIRoutes(t *testing.T) {
	runtime, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

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
