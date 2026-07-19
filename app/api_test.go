package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrepareAPIServesSharedRuntimeOnNetworkAndDesktop(t *testing.T) {
	app, err := newApp(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.prepareAPI("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.shutdown(context.Background()) })

	response, err := http.Get("http://" + app.apiAddress + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("network health status = %d", response.StatusCode)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Origin", "wails://wails")
	recorder := httptest.NewRecorder()
	app.handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("desktop health status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestPrepareAPIReusesExistingMonitorBackend(t *testing.T) {
	external := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Origin") != "" {
			t.Errorf("proxied origin = %q, want empty", request.Header.Get("Origin"))
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"status":"ok","name":"CPA Orbit"}`))
	}))
	defer external.Close()

	app, err := newApp(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	address := strings.TrimPrefix(external.URL, "http://")
	if err := app.prepareAPI(address); err != nil {
		t.Fatal(err)
	}
	if app.ownsAPIRuntime {
		t.Fatal("expected the existing Monitor API to be reused")
	}

	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Origin", "wails://wails")
	recorder := httptest.NewRecorder()
	app.handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("proxied health status = %d", recorder.Code)
	}
}
