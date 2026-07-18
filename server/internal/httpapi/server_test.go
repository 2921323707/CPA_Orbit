package httpapi

import "testing"

func TestAllowedOrigin(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://127.0.0.1:5173", "http://LOCALHOST:8080"}
	blocked := []string{"https://example.com", "http://127.0.0.2:3000", "file://localhost/test", "http://user@localhost:3000", "http://localhost:3000/path"}
	for _, origin := range allowed {
		if !allowedOrigin(origin) {
			t.Errorf("expected allowed origin: %s", origin)
		}
	}
	for _, origin := range blocked {
		if allowedOrigin(origin) {
			t.Errorf("expected blocked origin: %s", origin)
		}
	}
}
