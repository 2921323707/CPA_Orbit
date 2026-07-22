package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const companionAddress = "127.0.0.1:8317"

// companionSpec describes a desktop companion without embedding shell syntax.
type companionSpec struct {
	Name             string
	Executable       string
	executable       string
	Args             []string
	WorkingDirectory string
	Address          string
	ReadinessURL     string
	ReadinessMethod  string
	ReadinessHeaders map[string]string
	ExpectedStatus   int
	ExpectedBody     []string
	ExpectedJSON     map[string]string
	StartupTimeout   time.Duration
	Required         bool
	Ownership        string

	command *exec.Cmd
	done    chan error
	owned   bool
	mu      sync.Mutex
}

type companionManager struct {
	services []*companionSpec
}

// companionService is retained as a compatibility alias for integrations that
// used the old CPA-only name.
type companionService = companionSpec

func newCompanionManager(services ...*companionSpec) *companionManager {
	manager := &companionManager{}
	for _, service := range services {
		if service != nil {
			manager.services = append(manager.services, service)
		}
	}
	return manager
}

func (m *companionManager) Start() error {
	if m == nil {
		return nil
	}
	var failures []string
	for _, service := range m.services {
		started, err := service.Start()
		if err != nil {
			if service.Required {
				failures = append(failures, fmt.Sprintf("%s: %v", service.Name, err))
			}
			continue
		}
		_ = started
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (m *companionManager) Stop() error {
	if m == nil {
		return nil
	}
	var failures []string
	for i := len(m.services) - 1; i >= 0; i-- {
		if err := m.services[i].Stop(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", m.services[i].Name, err))
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// discoverCompanion preserves CPA discovery and defaults for older callers.
func discoverCompanion(executableDir, dataDir string) (*companionService, error) {
	manager, err := discoverCompanions(executableDir, dataDir, desktopConfig{})
	if err != nil {
		return nil, err
	}
	for _, service := range manager.services {
		if service.Name == "CPA" {
			return service, nil
		}
	}
	return nil, nil
}

func discoverCompanions(executableDir, dataDir string, config desktopConfig) (*companionManager, error) {
	manager := newCompanionManager()
	cpa, err := discoverCPA(executableDir, dataDir, config.CPA)
	if err != nil {
		return nil, err
	}
	if cpa != nil {
		manager.services = append(manager.services, cpa)
	}
	sub2api, sub2APIErr := discoverGenericCompanion("Sub2API", config.Sub2API, sub2APIEnvironment())
	if sub2api != nil {
		manager.services = append(manager.services, sub2api)
	}
	return manager, sub2APIErr
}

func discoverCPA(executableDir, dataDir string, fileConfig *companionConfigFile) (*companionSpec, error) {
	executableOverride := strings.TrimSpace(os.Getenv("CPA_ORBIT_CPA_EXECUTABLE"))
	configOverride := strings.TrimSpace(os.Getenv("CPA_ORBIT_CPA_CONFIG"))
	if executableOverride != "" || configOverride != "" {
		if executableOverride == "" {
			return nil, errors.New("CPA_ORBIT_CPA_EXECUTABLE is required when CPA_ORBIT_CPA_CONFIG is set")
		}
		if configOverride == "" {
			configOverride = filepath.Join(filepath.Dir(executableOverride), "config.yaml")
		}
		return companionFromFiles(executableOverride, configOverride)
	}
	candidates := [][2]string{
		{filepath.Join(executableDir, "cpa", "cli-proxy-api.exe"), filepath.Join(executableDir, "cpa", "config.yaml")},
		{filepath.Join(executableDir, "cpa", "app", "cli-proxy-api.exe"), filepath.Join(executableDir, "cpa", "app", "config.yaml")},
		{filepath.Join(dataDir, "cpa", "app", "cli-proxy-api.exe"), filepath.Join(dataDir, "cpa", "app", "config.yaml")},
	}
	if root, ok := repositoryRootFromExecutable(executableDir); ok {
		candidates = append(candidates, [2]string{filepath.Join(root, "cpa", "app", "cli-proxy-api.exe"), filepath.Join(root, "cpa", "app", "config.yaml")})
	}
	for _, candidate := range candidates {
		if service, err := companionFromFiles(candidate[0], candidate[1]); err == nil {
			return service, nil
		}
	}
	if fileConfig != nil && fileConfig.Executable != "" {
		return companionFromConfig("CPA", *fileConfig)
	}
	return nil, nil
}

func companionFromConfig(name string, config companionConfigFile) (*companionSpec, error) {
	if strings.TrimSpace(config.Executable) == "" {
		return nil, fmt.Errorf("%s executable is required", name)
	}
	args := append([]string(nil), config.Args...)
	timeout, err := durationFromEnv(config.StartupTimeout, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("%s startupTimeout: %w", name, err)
	}
	address := config.Address
	if address == "" {
		return nil, fmt.Errorf("%s address is required", name)
	}
	return &companionSpec{Name: name, Executable: config.Executable, Args: args, WorkingDirectory: config.WorkingDirectory, Address: address, ReadinessURL: config.ReadinessURL, ReadinessMethod: config.ReadinessMethod, ReadinessHeaders: config.ReadinessHeaders, ExpectedStatus: config.ExpectedStatus, ExpectedBody: config.ExpectedBody, ExpectedJSON: config.ExpectedJSON, StartupTimeout: timeout, Required: config.Required, Ownership: config.Ownership}, nil
}

func discoverGenericCompanion(name string, config *companionConfigFile, environment map[string]string) (*companionSpec, error) {
	if config == nil {
		config = &companionConfigFile{}
	}
	if value := strings.TrimSpace(environment["executable"]); value != "" {
		config.Executable = value
	}
	if value := strings.TrimSpace(environment["args"]); value != "" {
		args, err := parseCompanionArgs(value)
		if err != nil {
			return nil, err
		}
		config.Args = args
	}
	if value := strings.TrimSpace(environment["workingDirectory"]); value != "" {
		config.WorkingDirectory = value
	}
	if value := strings.TrimSpace(environment["address"]); value != "" {
		config.Address = value
	}
	if value := strings.TrimSpace(environment["readinessURL"]); value != "" {
		config.ReadinessURL = value
	}
	if value := strings.TrimSpace(environment["readinessMethod"]); value != "" {
		config.ReadinessMethod = value
	}
	if value := strings.TrimSpace(environment["readinessHeaders"]); value != "" {
		headers, err := parseCompanionHeaders(value)
		if err != nil {
			return nil, err
		}
		config.ReadinessHeaders = headers
	}
	if value := strings.TrimSpace(environment["timeout"]); value != "" {
		config.StartupTimeout = value
	}
	if value := strings.TrimSpace(environment["required"]); value != "" {
		required, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("%s required: %w", name, err)
		}
		config.Required = required
	}
	if config.Executable == "" {
		if config.Required {
			return nil, fmt.Errorf("%s executable is required", name)
		}
		return nil, nil
	}
	service, err := companionFromConfig(name, *config)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func sub2APIEnvironment() map[string]string {
	return map[string]string{
		"executable": os.Getenv("CPA_ORBIT_SUB2API_EXECUTABLE"), "args": os.Getenv("CPA_ORBIT_SUB2API_ARGS"),
		"workingDirectory": os.Getenv("CPA_ORBIT_SUB2API_WORKING_DIRECTORY"), "address": os.Getenv("CPA_ORBIT_SUB2API_ADDRESS"),
		"readinessURL": os.Getenv("CPA_ORBIT_SUB2API_READINESS_URL"), "readinessMethod": os.Getenv("CPA_ORBIT_SUB2API_READINESS_METHOD"),
		"readinessHeaders": os.Getenv("CPA_ORBIT_SUB2API_READINESS_HEADERS"), "timeout": os.Getenv("CPA_ORBIT_SUB2API_STARTUP_TIMEOUT"),
		"required": os.Getenv("CPA_ORBIT_SUB2API_REQUIRED"),
	}
}

func companionFromFiles(executablePath, configPath string) (*companionSpec, error) {
	executablePath, err := filepath.Abs(executablePath)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIProxyAPI executable: %w", err)
	}
	configPath, err = filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("resolve CLIProxyAPI config: %w", err)
	}
	for label, path := range map[string]string{"executable": executablePath, "config": configPath} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("CLIProxyAPI %s is unavailable: %s", label, path)
		}
	}
	return &companionSpec{Name: "CPA", Executable: executablePath, executable: executablePath, Args: []string{"-config", configPath, "-no-browser"}, WorkingDirectory: filepath.Dir(executablePath), Address: companionAddress, ReadinessURL: "http://" + companionAddress + "/", ReadinessMethod: http.MethodGet, ExpectedStatus: http.StatusOK, ExpectedBody: []string{"CLI Proxy API Server"}, StartupTimeout: 10 * time.Second, Required: false, Ownership: "desktop"}, nil
}

// Start validates an existing listener before reusing it, then starts only when
// necessary. It never emits command arguments (which may contain secrets).
func (s *companionSpec) Start() (bool, error) {
	if s == nil {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.command != nil {
		return false, nil
	}
	if s.Address != "" && companionReady(s) {
		return false, nil
	}
	if s.Executable == "" && s.executable != "" {
		s.Executable = s.executable
	}
	if s.Executable == "" {
		return false, errors.New("executable is not configured")
	}
	if s.Ownership == "" {
		s.Ownership = "desktop"
	}
	command := exec.Command(s.Executable, s.Args...)
	if s.WorkingDirectory != "" {
		command.Dir = s.WorkingDirectory
	}
	configureCompanionCommand(command)
	if err := command.Start(); err != nil {
		return false, fmt.Errorf("start %s: %w", s.Name, err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	timeout := s.StartupTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if err == nil {
				err = errors.New("process exited before readiness")
			}
			return false, fmt.Errorf("%s stopped during startup: %w", s.Name, err)
		case <-ticker.C:
			if companionReady(s) {
				s.command, s.done, s.owned = command, done, true
				return true, nil
			}
		case <-timer.C:
			_ = command.Process.Kill()
			<-done
			return false, fmt.Errorf("%s did not become ready within %s", s.Name, timeout)
		}
	}
}

func (s *companionSpec) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	command, done, owned := s.command, s.done, s.owned
	s.command, s.done, s.owned = nil, nil, false
	s.mu.Unlock()
	if command == nil || !owned {
		return nil
	}
	select {
	case <-done:
		return nil
	default:
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop %s: %w", s.Name, err)
	}
	<-done
	return nil
}

func companionReady(s *companionSpec) bool {
	if s.Address == "" {
		return false
	}
	if s.ReadinessURL == "" {
		return tcpListening(s.Address)
	}
	method := s.ReadinessMethod
	if method == "" {
		method = http.MethodGet
	}
	request, err := http.NewRequest(method, s.ReadinessURL, nil)
	if err != nil {
		return false
	}
	for key, value := range s.ReadinessHeaders {
		request.Header.Set(key, value)
	}
	client := &http.Client{Timeout: 750 * time.Millisecond}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	status := s.ExpectedStatus
	if status == 0 {
		status = http.StatusOK
	}
	if response.StatusCode != status {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return false
	}
	for _, expected := range s.ExpectedBody {
		if !bytes.Contains(body, []byte(expected)) {
			return false
		}
	}
	if len(s.ExpectedJSON) > 0 {
		var payload map[string]any
		if json.Unmarshal(body, &payload) != nil {
			return false
		}
		for key, expected := range s.ExpectedJSON {
			if fmt.Sprint(payload[key]) != expected {
				return false
			}
		}
	}
	return true
}

func tcpListening(address string) bool {
	connection, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
	if err != nil {
		return false
	}
	_ = connection.Close()
	return true
}

func parseCompanionArgs(value string) ([]string, error) {
	var args []string
	if err := json.Unmarshal([]byte(value), &args); err != nil {
		return nil, fmt.Errorf("companion args must be a JSON array: %w", err)
	}
	return args, nil
}

func parseCompanionHeaders(value string) (map[string]string, error) {
	var headers map[string]string
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return nil, fmt.Errorf("companion headers must be a JSON object: %w", err)
	}
	return headers, nil
}

func durationFromEnv(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(value)
}
