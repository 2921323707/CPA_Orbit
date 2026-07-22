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

func TestParseJSONRecognizesSingleAccountSub2APIData(t *testing.T) {
	data := []byte(`{
		"type":"sub2api-data","version":1,"proxies":[],
		"accounts":[{
			"name":"agent@example.com","platform":"openai","type":"oauth",
			"credentials":{"auth_mode":"agent_identity","email":"agent@example.com","account_id":"acct-1","chatgpt_account_id":"chatgpt-1","plan_type":"plus","agent_private_key":"secret"},
			"extra":{"last_refresh":"2026-07-22T01:02:03Z"}
		}]
	}`)
	s, err := ParseJSON(data, "sub2api/0722/agent.json", time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if s.Email != "agent@example.com" || s.Name != "agent@example.com" || s.Provider != "openai" || s.Type != "oauth" {
		t.Fatalf("Sub2API identity was not recognized: %+v", s)
	}
	if s.AccountID != "acct-1" || s.ChatGPTAccountID != "chatgpt-1" || s.PlanType != "plus" || s.LastRefresh != "2026-07-22T01:02:03Z" {
		t.Fatalf("Sub2API metadata was not projected: %+v", s)
	}
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(encoded), "agent_private_key") || contains(string(encoded), "secret") {
		t.Fatalf("agent identity secret leaked in API model: %s", encoded)
	}
}

func TestParseJSONRejectsMultiAccountSub2APIData(t *testing.T) {
	data := []byte(`{"type":"sub2api-data","version":1,"accounts":[{"name":"one"},{"name":"two"}]}`)
	if _, err := ParseJSON(data, "sub2api/0722/bundle.json", time.Now()); err == nil {
		t.Fatal("expected multi-account bundle rejection")
	}
}

func TestParseJSONRecognizesMarkerlessSub2APIExport(t *testing.T) {
	data := []byte(`{
		"exported_at":"2026-07-22T00:00:00Z","proxies":[],
		"accounts":[{"name":"markerless@example.com","platform":"openai","type":"oauth",
		"credentials":{"email":"markerless@example.com","account_id":"acct-2","plan_type":"team"},
		"extra":{"last_refresh":"2026-07-22T02:03:04Z"}}]
	}`)
	s, err := ParseJSON(data, "sub2api/0722/markerless.json", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if s.Email != "markerless@example.com" || s.AccountID != "acct-2" || s.PlanType != "team" || s.Provider != "openai" {
		t.Fatalf("markerless export metadata was not projected: %+v", s)
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
