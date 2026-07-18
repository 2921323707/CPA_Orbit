package subscriptions

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseJSONCompatibilityAndNoTokenSerialization(t *testing.T) {
	data := []byte(`{
		"email":"a@example.com", "account_id":"a1", "chatgpt_account_id":"c1",
		"plan_type":"k12", "chatgpt_plan_type":"k12", "access_token":"secret-token",
		"expired":"2026-07-20T12:00:00Z", "last_refresh":"2026-07-18T00:00:00Z",
		"type":"codex", "base_url":"http://127.0.0.1:8317/v1", "order_url":"https://shop/item/1",
		"remaining_quota":"12.5", "acquisition_price":"8.80"
	}`)
	s, err := ParseJSON(data, "0718/a.json", time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if s.Email != "a@example.com" || s.Balance == nil || *s.Balance != 12.5 {
		t.Fatalf("compatibility parse failed: %+v", s)
	}
	if s.AcquisitionPrice == nil || *s.AcquisitionPrice != 8.8 {
		t.Fatalf("acquisition price parse failed: %+v", s.AcquisitionPrice)
	}
	if s.RemainingDays == nil || *s.RemainingDays != 2 || s.Status != "active" {
		t.Fatalf("expiry calculation failed: %+v", s)
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || contains(string(encoded), "secret-token") || contains(string(encoded), "access_token") {
		t.Fatalf("token leaked in JSON: %s", encoded)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
