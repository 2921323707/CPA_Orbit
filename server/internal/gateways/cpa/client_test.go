package cpa

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cpa-monitor/server/internal/gateways"
)

func TestClientImplementsGatewayAdapter(t *testing.T) {
	var _ gateways.Adapter = (*Client)(nil)
}

func TestReconcileOnlyRemovesManifestOwnedFiles(t *testing.T) {
	authDir := t.TempDir()
	manifest := filepath.Join(t.TempDir(), "cpa-managed.json")
	external := filepath.Join(authDir, "external.json")
	if err := os.WriteFile(external, []byte(`{"email":"external@example.com","access_token":"keep"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewClient(func() Config {
		return Config{AuthDir: authDir, SyncEnabled: true}
	}, manifest)
	credential := gateways.Credential{SubscriptionID: "sub-1", Email: "owned@example.com", Data: []byte(`{"email":"owned@example.com","access_token":"token"}`)}
	result, err := client.Deploy(context.Background(), credential, gateways.DeployOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.Binding.ExternalRef); err != nil {
		t.Fatalf("managed runtime file missing: %v", err)
	}
	if err := client.Reconcile(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(result.Binding.ExternalRef); !os.IsNotExist(err) {
		t.Fatalf("managed orphan was not removed: %v", err)
	}
	if _, err := os.Stat(external); err != nil {
		t.Fatalf("unmanaged CPA file must survive reconciliation: %v", err)
	}
}

func TestDetachRejectsUnmanagedFile(t *testing.T) {
	authDir := t.TempDir()
	external := filepath.Join(authDir, "external.json")
	data := []byte(`{"email":"external@example.com","access_token":"keep"}`)
	if err := os.WriteFile(external, data, 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewClient(func() Config { return Config{AuthDir: authDir, SyncEnabled: true} }, filepath.Join(t.TempDir(), "manifest.json"))
	err := client.Detach(context.Background(), gateways.BindingRef{ExternalRef: external}, gateways.Credential{Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(external); err != nil {
		t.Fatalf("unmanaged file was removed: %v", err)
	}
}
