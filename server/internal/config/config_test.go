package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAccountPollMinutesPersistenceAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"threshold":1,"refreshMinutes":5,"baseUrl":"http://127.0.0.1:8317/v1","themeMode":"auto"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get().AccountPollMinutes; got != 5 {
		t.Fatalf("old settings accountPollMinutes = %d, want 5", got)
	}
	settings := store.Get()
	settings.AccountPollMinutes = 0
	if err := store.Update(settings); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get().AccountPollMinutes; got != 0 {
		t.Fatalf("explicit disabled accountPollMinutes = %d, want 0", got)
	}
	for _, value := range []int{-1, 1, 4, 1441} {
		settings.AccountPollMinutes = value
		if err := ValidateSettings(settings); err == nil {
			t.Errorf("accountPollMinutes %d should be invalid", value)
		}
	}
	for _, value := range []int{0, 5, 1440} {
		settings.AccountPollMinutes = value
		if err := ValidateSettings(settings); err != nil {
			t.Errorf("accountPollMinutes %d should be valid: %v", value, err)
		}
	}
}

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		remote bool
		ok     bool
	}{
		{"localhost", "http://localhost:8317/v1", false, true},
		{"ipv4 loopback", "http://127.0.0.1:8317/v1", false, true},
		{"ipv6 loopback", "http://[::1]:8317/v1", false, true},
		{"remote blocked", "https://example.com/v1", false, false},
		{"remote allowed", "https://example.com/v1", true, true},
		{"file blocked", "file:///tmp/token", true, false},
		{"credentials blocked", "http://user:pass@localhost/v1", false, false},
		{"relative blocked", "/v1", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBaseURL(tt.url, tt.remote)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidateBaseURL() error = %v, ok=%v", err, tt.ok)
			}
		})
	}
}
