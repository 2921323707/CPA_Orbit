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
	defaultWindowHeight = 820
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
	if result.WindowWidth < 960 || result.WindowHeight < 640 {
		return desktopConfig{}, errors.New("windowWidth must be at least 960 and windowHeight at least 640")
	}

	dataDir := dataOverride
	if dataDir == "" {
		dataDir = strings.TrimSpace(fileConfig.DataDir)
	}
	if dataDir == "" {
		dataDir = filepath.Join(userConfigDir, "CPA Orbit")
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
