package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverCompanionUsesSharedRepositoryRuntime(t *testing.T) {
	root := t.TempDir()
	executableDir := filepath.Join(root, "app", "build", "bin")
	files := []string{
		filepath.Join(root, "server", "go.mod"),
		filepath.Join(root, "web", "package.json"),
		filepath.Join(root, "app", "wails.json"),
		filepath.Join(root, "cpa", "app", "cli-proxy-api.exe"),
		filepath.Join(root, "cpa", "app", "config.yaml"),
	}
	for _, path := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	service, err := discoverCompanion(executableDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if service == nil {
		t.Fatal("expected repository companion")
	}
	want := filepath.Join(root, "cpa", "app", "cli-proxy-api.exe")
	if service.executable != want {
		t.Fatalf("executable = %q, want %q", service.executable, want)
	}
}

func TestDiscoverCompanionAllowsMissingOptionalRuntime(t *testing.T) {
	service, err := discoverCompanion(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if service != nil {
		t.Fatal("expected no optional companion")
	}
}
