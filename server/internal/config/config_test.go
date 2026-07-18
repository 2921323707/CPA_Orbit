package config

import "testing"

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
