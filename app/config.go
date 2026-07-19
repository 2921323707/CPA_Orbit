package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultWindowWidth  = 1280
	defaultWindowHeight = 800
	minimumWindowWidth  = 1024
	minimumWindowHeight = 640
	configFileName      = "cpa-orbit.config.json"
)

type desktopConfigFile struct {
	DataDir      string `json:"dataDir"`
	WindowWidth  int    `json:"windowWidth"`
	WindowHeight int    `json:"windowHeight"`
}

type desktopConfig struct {
	DataDir      string
	ConfigPath   string
	WindowWidth  int
	WindowHeight int
}

func loadDesktopConfig() (desktopConfig, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return desktopConfig{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return desktopConfig{}, fmt.Errorf("resolve executable path: %w", err)
	}
	return loadDesktopConfigFrom(
		strings.TrimSpace(os.Getenv("CPA_ORBIT_CONFIG")),
		strings.TrimSpace(os.Getenv("CPA_ORBIT_DATA_DIR")),
		filepath.Dir(executable),
		userConfigDir,
	)
}

func loadDesktopConfigFrom(explicitConfig, dataOverride, executableDir, userConfigDir string) (desktopConfig, error) {
	result := desktopConfig{
		WindowWidth:  defaultWindowWidth,
		WindowHeight: defaultWindowHeight,
	}
	configPath := explicitConfig
	configRequired := configPath != ""
	if configPath == "" {
		configPath = defaultConfigPath(executableDir)
	}
	if absolute, err := filepath.Abs(configPath); err == nil {
		configPath = absolute
	} else {
		return desktopConfig{}, fmt.Errorf("resolve desktop config path: %w", err)
	}

	var fileConfig desktopConfigFile
	file, err := os.Open(configPath)
	if err != nil {
		if configRequired || !errors.Is(err, os.ErrNotExist) {
			return desktopConfig{}, fmt.Errorf("open desktop config %s: %w", configPath, err)
		}
		configPath = ""
	} else {
		defer file.Close()
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&fileConfig); err != nil {
			return desktopConfig{}, fmt.Errorf("decode desktop config %s: %w", configPath, err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			return desktopConfig{}, fmt.Errorf("decode desktop config %s: %w", configPath, err)
		}
	}

	if fileConfig.WindowWidth != 0 {
		result.WindowWidth = fileConfig.WindowWidth
	}
	if fileConfig.WindowHeight != 0 {
		result.WindowHeight = fileConfig.WindowHeight
	}
	if result.WindowWidth < minimumWindowWidth || result.WindowHeight < minimumWindowHeight {
		return desktopConfig{}, fmt.Errorf("windowWidth must be at least %d and windowHeight at least %d", minimumWindowWidth, minimumWindowHeight)
	}

	dataDir := dataOverride
	if dataDir == "" {
		dataDir = strings.TrimSpace(fileConfig.DataDir)
	}
	if dataDir == "" {
		dataDir = defaultDataDir(executableDir, userConfigDir)
	} else if !filepath.IsAbs(dataDir) {
		base := executableDir
		if configPath != "" {
			base = filepath.Dir(configPath)
		}
		dataDir = filepath.Join(base, dataDir)
	}
	absoluteDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return desktopConfig{}, fmt.Errorf("resolve data directory: %w", err)
	}
	result.DataDir = filepath.Clean(absoluteDataDir)
	result.ConfigPath = configPath
	return result, nil
}

// defaultDataDir lets an executable launched from app/build/bin share the
// repository's data, settings, keys, and subscription archive with web
// development. Standalone copies continue to use the per-user directory.
func defaultDataDir(executableDir, userConfigDir string) string {
	if root, ok := repositoryRootFromExecutable(executableDir); ok {
		return root
	}
	return filepath.Join(userConfigDir, "CPA Orbit")
}

func repositoryRootFromExecutable(executableDir string) (string, bool) {
	root := filepath.Clean(filepath.Join(executableDir, "..", "..", ".."))
	required := []string{
		filepath.Join(root, "server", "go.mod"),
		filepath.Join(root, "web", "package.json"),
		filepath.Join(root, "app", "wails.json"),
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", false
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	return filepath.Clean(absolute), true
}

func defaultConfigPath(executableDir string) string {
	if filepath.Base(executableDir) == "MacOS" {
		contentsDir := filepath.Dir(executableDir)
		appBundle := filepath.Dir(contentsDir)
		if filepath.Base(contentsDir) == "Contents" && strings.HasSuffix(strings.ToLower(filepath.Base(appBundle)), ".app") {
			return filepath.Join(filepath.Dir(appBundle), configFileName)
		}
	}
	return filepath.Join(executableDir, configFileName)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("configuration must contain a single JSON object")
}
