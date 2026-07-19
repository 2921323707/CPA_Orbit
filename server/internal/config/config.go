package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cpa-monitor/server/internal/storage"
)

const (
	DefaultListenAddr = "127.0.0.1:8080"
	DefaultBaseURL    = "http://127.0.0.1:8317/v1"
)

type Settings struct {
	Threshold            float64 `json:"threshold"`
	RefreshMinutes       int     `json:"refreshMinutes"`
	WebhookURL           string  `json:"webhookUrl"`
	BaseURL              string  `json:"baseUrl"`
	APIKey               string  `json:"apiKey"`
	CPAManagementKey     string  `json:"cpaManagementKey"`
	LubanAPIKey          string  `json:"lubanApiKey"`
	AllowRemoteBaseURL   bool    `json:"allowRemoteBaseUrl"`
	CPAAuthDir           string  `json:"cpaAuthDir"`
	SyncToCPAAuthDir     bool    `json:"syncToCpaAuthDir"`
	ThemeMode            string  `json:"themeMode"`
	StartOnLogin         bool    `json:"startOnLogin"`
	CloseToTray          bool    `json:"closeToTray"`
	DesktopNotifications bool    `json:"desktopNotifications"`
	FlashOnAlert         bool    `json:"flashOnAlert"`
}

type PublicSettings struct {
	Threshold                  float64 `json:"threshold"`
	RefreshMinutes             int     `json:"refreshMinutes"`
	WebhookURL                 string  `json:"webhookUrl"`
	BaseURL                    string  `json:"baseUrl"`
	APIKeyConfigured           bool    `json:"apiKeyConfigured"`
	CPAManagementKeyConfigured bool    `json:"cpaManagementKeyConfigured"`
	LubanAPIKeyConfigured      bool    `json:"lubanApiKeyConfigured"`
	AllowRemoteBaseURL         bool    `json:"allowRemoteBaseUrl"`
	CPAAuthDir                 string  `json:"cpaAuthDir"`
	SyncToCPAAuthDir           bool    `json:"syncToCpaAuthDir"`
	ThemeMode                  string  `json:"themeMode"`
	StartOnLogin               bool    `json:"startOnLogin"`
	CloseToTray                bool    `json:"closeToTray"`
	DesktopNotifications       bool    `json:"desktopNotifications"`
	FlashOnAlert               bool    `json:"flashOnAlert"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data Settings
}

func Defaults() Settings {
	return Settings{Threshold: 1, RefreshMinutes: 5, BaseURL: DefaultBaseURL, ThemeMode: "auto", CloseToTray: true, DesktopNotifications: true, FlashOnAlert: true}
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	var loaded Settings
	if err := storage.LoadJSON(path, &loaded); err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	data, err := settingsFromLoaded(loaded)
	if err != nil {
		return nil, fmt.Errorf("invalid persisted settings: %w", err)
	}
	s.data = data
	return s, nil
}

func (s *Store) Reload() error {
	var loaded Settings
	if err := storage.LoadJSON(s.path, &loaded); err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	data, err := settingsFromLoaded(loaded)
	if err != nil {
		return fmt.Errorf("invalid persisted settings: %w", err)
	}
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()
	return nil
}

func settingsFromLoaded(loaded Settings) (Settings, error) {
	s := Defaults()
	if loaded.Threshold > 0 {
		s.Threshold = loaded.Threshold
	}
	if loaded.RefreshMinutes > 0 {
		s.RefreshMinutes = loaded.RefreshMinutes
	}
	if strings.TrimSpace(loaded.BaseURL) != "" {
		s.BaseURL = strings.TrimSpace(loaded.BaseURL)
	}
	s.WebhookURL = strings.TrimSpace(loaded.WebhookURL)
	s.APIKey = loaded.APIKey
	s.CPAManagementKey = loaded.CPAManagementKey
	s.LubanAPIKey = loaded.LubanAPIKey
	s.AllowRemoteBaseURL = loaded.AllowRemoteBaseURL
	s.CPAAuthDir = strings.TrimSpace(loaded.CPAAuthDir)
	s.SyncToCPAAuthDir = loaded.SyncToCPAAuthDir
	if strings.TrimSpace(loaded.ThemeMode) != "" {
		s.ThemeMode = strings.ToLower(strings.TrimSpace(loaded.ThemeMode))
		s.StartOnLogin = loaded.StartOnLogin
		s.CloseToTray = loaded.CloseToTray
		s.DesktopNotifications = loaded.DesktopNotifications
		s.FlashOnAlert = loaded.FlashOnAlert
	}
	if err := ValidateSettings(s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *Store) Public() PublicSettings {
	v := s.Get()
	return PublicSettings{
		Threshold: v.Threshold, RefreshMinutes: v.RefreshMinutes,
		WebhookURL: v.WebhookURL, BaseURL: v.BaseURL,
		APIKeyConfigured: v.APIKey != "", CPAManagementKeyConfigured: v.CPAManagementKey != "",
		LubanAPIKeyConfigured: v.LubanAPIKey != "",
		AllowRemoteBaseURL:    v.AllowRemoteBaseURL, CPAAuthDir: v.CPAAuthDir,
		SyncToCPAAuthDir: v.SyncToCPAAuthDir, ThemeMode: v.ThemeMode,
		StartOnLogin: v.StartOnLogin, CloseToTray: v.CloseToTray,
		DesktopNotifications: v.DesktopNotifications, FlashOnAlert: v.FlashOnAlert,
	}
}

func (s *Store) Update(v Settings) error {
	if err := ValidateSettings(v); err != nil {
		return err
	}
	v.WebhookURL = strings.TrimSpace(v.WebhookURL)
	v.BaseURL = strings.TrimRight(strings.TrimSpace(v.BaseURL), "/")
	v.CPAAuthDir = strings.TrimSpace(v.CPAAuthDir)
	v.LubanAPIKey = strings.TrimSpace(v.LubanAPIKey)
	v.ThemeMode = strings.ToLower(strings.TrimSpace(v.ThemeMode))
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := storage.SaveJSON(s.path, v); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	s.data = v
	return nil
}

func ValidateSettings(v Settings) error {
	if v.Threshold <= 0 {
		return errors.New("threshold must be greater than zero")
	}
	if v.RefreshMinutes < 1 || v.RefreshMinutes > 1440 {
		return errors.New("refreshMinutes must be between 1 and 1440")
	}
	if v.ThemeMode != "light" && v.ThemeMode != "dark" && v.ThemeMode != "auto" {
		return errors.New("themeMode must be light, dark, or auto")
	}
	if err := ValidateBaseURL(v.BaseURL, v.AllowRemoteBaseURL); err != nil {
		return fmt.Errorf("baseUrl: %w", err)
	}
	if v.WebhookURL != "" {
		if err := ValidateHTTPURL(v.WebhookURL); err != nil {
			return fmt.Errorf("webhookUrl: %w", err)
		}
	}
	if v.SyncToCPAAuthDir {
		if err := ValidateCPAAuthDir(v.CPAAuthDir); err != nil {
			return fmt.Errorf("cpaAuthDir: %w", err)
		}
	}
	return nil
}

func ValidateHTTPURL(raw string) error {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("must be a valid absolute URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("only http and https URLs are allowed")
	}
	if u.Host == "" || u.Hostname() == "" {
		return errors.New("URL host is required")
	}
	if u.User != nil {
		return errors.New("URL credentials are not allowed")
	}
	return nil
}

func ValidateCPAAuthDir(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("is required when syncToCpaAuthDir is enabled")
	}
	if !filepath.IsAbs(raw) {
		return errors.New("must be an absolute path")
	}
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		return errors.New("must exist and be accessible")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return errors.New("must be an existing directory")
	}
	return nil
}

func ValidateBaseURL(raw string, allowRemote bool) error {
	if err := ValidateHTTPURL(raw); err != nil {
		return err
	}
	if allowRemote {
		return nil
	}
	u, _ := url.Parse(strings.TrimSpace(raw))
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("remote hosts are disabled; enable allowRemoteBaseUrl to use one")
	}
	return nil
}
