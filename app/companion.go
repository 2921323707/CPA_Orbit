package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const companionAddress = "127.0.0.1:8317"

type companionService struct {
	executable string
	config     string
	command    *exec.Cmd
	done       chan error
}

func discoverCompanion(executableDir, dataDir string) (*companionService, error) {
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
		candidates = append(candidates, [2]string{
			filepath.Join(root, "cpa", "app", "cli-proxy-api.exe"),
			filepath.Join(root, "cpa", "app", "config.yaml"),
		})
	}
	for _, candidate := range candidates {
		service, err := companionFromFiles(candidate[0], candidate[1])
		if err == nil {
			return service, nil
		}
	}
	return nil, nil
}

func companionFromFiles(executablePath, configPath string) (*companionService, error) {
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
	return &companionService{executable: executablePath, config: configPath}, nil
}

// Start launches CLIProxyAPI only when port 8317 is not already served. The
// caller therefore owns and later stops only the process started here.
func (s *companionService) Start() (bool, error) {
	if s == nil || tcpListening(companionAddress) {
		return false, nil
	}
	if s.command != nil {
		return false, nil
	}

	command := exec.Command(s.executable, "-config", s.config, "-no-browser")
	command.Dir = filepath.Dir(s.executable)
	configureCompanionCommand(command)
	if err := command.Start(); err != nil {
		return false, fmt.Errorf("start CLIProxyAPI: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()

	timer := time.NewTimer(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if err == nil {
				err = errors.New("process exited before opening port 8317")
			}
			return false, fmt.Errorf("CLIProxyAPI stopped during startup: %w", err)
		case <-ticker.C:
			if tcpListening(companionAddress) {
				s.command = command
				s.done = done
				return true, nil
			}
		case <-timer.C:
			_ = command.Process.Kill()
			<-done
			return false, errors.New("CLIProxyAPI did not open port 8317 within 10 seconds")
		}
	}
}

func (s *companionService) Stop() error {
	if s == nil || s.command == nil {
		return nil
	}
	command, done := s.command, s.done
	s.command, s.done = nil, nil
	select {
	case <-done:
		return nil
	default:
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("stop CLIProxyAPI: %w", err)
	}
	<-done
	return nil
}

func tcpListening(address string) bool {
	connection, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
	if err != nil {
		return false
	}
	_ = connection.Close()
	return true
}
