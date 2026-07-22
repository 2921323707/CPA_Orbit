package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacySubscriptionRootMovesDatesToSub2API(t *testing.T) {
	root := t.TempDir()
	legacyDate := filepath.Join(root, "k12", "0722")
	if err := os.MkdirAll(legacyDate, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDate, "account.json")
	if err := os.WriteFile(legacyFile, []byte(`{"access_token":"kept"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacySubscriptionRoot(root); err != nil {
		t.Fatal(err)
	}
	migrated := filepath.Join(root, "subscriptions", "sub2api", "0722", "account.json")
	if data, err := os.ReadFile(migrated); err != nil || string(data) != `{"access_token":"kept"}` {
		t.Fatalf("migrated file mismatch: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "k12")); !os.IsNotExist(err) {
		t.Fatalf("legacy root still exists: %v", err)
	}
	var pathIDs map[string]string
	data, err := os.ReadFile(filepath.Join(root, "data", "subscription_path_migrations.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &pathIDs); err != nil || pathIDs["sub2api/0722/account.json"] != legacySubscriptionID("0722/account.json") {
		t.Fatalf("legacy path identity map missing: %v", pathIDs)
	}
}

func TestMigrateLegacySubscriptionRootDoesNotOverwriteDestination(t *testing.T) {
	root := t.TempDir()
	legacyDate := filepath.Join(root, "k12", "0722")
	newDate := filepath.Join(root, "subscriptions", "sub2api", "0722")
	if err := os.MkdirAll(legacyDate, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDate, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDate, "same.json"), []byte(`{"source":"legacy"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newDate, "same.json"), []byte(`{"source":"new"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacySubscriptionRoot(root); err == nil {
		t.Fatal("expected collision to stop migration")
	}
	data, err := os.ReadFile(filepath.Join(newDate, "same.json"))
	if err != nil || string(data) != `{"source":"new"}` {
		t.Fatalf("destination was overwritten: data=%q err=%v", data, err)
	}
}
