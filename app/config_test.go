package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDesktopConfigUsesPerUserDefault(t *testing.T) {
	executableDir := t.TempDir()
	userConfigDir := filepath.Join(t.TempDir(), "user-config")
	config, err := loadDesktopConfigFrom("", "", executableDir, userConfigDir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(userConfigDir, "CPA Orbit")
	if config.DataDir != want {
		t.Fatalf("data dir = %q, want %q", config.DataDir, want)
	}
	if config.ConfigPath != "" {
		t.Fatalf("config path = %q, want empty", config.ConfigPath)
	}
}

func TestDefaultConfigPathSupportsMacAppBundle(t *testing.T) {
	bundleParent := t.TempDir()
	executableDir := filepath.Join(bundleParent, "CPAOrbit.app", "Contents", "MacOS")
	want := filepath.Join(bundleParent, configFileName)
	if got := defaultConfigPath(executableDir); got != want {
		t.Fatalf("config path = %q, want %q", got, want)
	}
}

func TestLoadDesktopConfigResolvesPortableRelativeDataDir(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	if err := os.WriteFile(configPath, []byte(`{"dataDir":"./portable-data","windowWidth":1440,"windowHeight":900}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadDesktopConfigFrom(configPath, "", t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if config.DataDir != filepath.Join(dir, "portable-data") {
		t.Fatalf("data dir = %q", config.DataDir)
	}
	if config.WindowWidth != 1440 || config.WindowHeight != 900 {
		t.Fatalf("window = %dx%d", config.WindowWidth, config.WindowHeight)
	}
}

func TestLoadDesktopConfigEnvironmentOverrideWins(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	if err := os.WriteFile(configPath, []byte(`{"dataDir":"./from-config"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(t.TempDir(), "override")
	config, err := loadDesktopConfigFrom(configPath, override, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if config.DataDir != override {
		t.Fatalf("data dir = %q, want %q", config.DataDir, override)
	}
}

func TestLoadDesktopConfigRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, configFileName)
	if err := os.WriteFile(configPath, []byte(`{"dataDirectory":"./typo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadDesktopConfigFrom(configPath, "", t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("expected unknown field error")
	}
}
